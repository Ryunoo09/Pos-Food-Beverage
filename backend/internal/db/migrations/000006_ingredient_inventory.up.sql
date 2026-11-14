-- F07/F09 Inventaris: satuan, bahan, konversi, saldo, ledger stok, riwayat biaya
-- Sekaligus menambahkan FK tertunda dari migrasi 003 & 004 ke ingredients / stock_movements.

CREATE TABLE units (
    id SMALLINT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    dimension TEXT NOT NULL CHECK (dimension IN ('MASS', 'VOLUME', 'COUNT')),
    to_base_factor NUMERIC(18,6) NOT NULL CHECK (to_base_factor > 0),
    is_base BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT uq_units_code UNIQUE (code)
);

CREATE TABLE ingredient_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_ingredient_categories_business_name ON ingredient_categories (business_id, lower(name));

-- base_unit_id tidak boleh diubah setelah ada movement (dijaga service/audit).
CREATE TABLE ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    category_id UUID REFERENCES ingredient_categories(id) ON DELETE SET NULL,
    base_unit_id SMALLINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    default_purchase_unit_id SMALLINT REFERENCES units(id) ON DELETE RESTRICT,
    min_stock NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_stock_product BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_ingredients_business_code UNIQUE (business_id, code)
);
CREATE INDEX idx_ingredients_category ON ingredients (category_id);
CREATE INDEX idx_ingredients_active ON ingredients (business_id, is_active);

-- Konversi satuan non-standar (mis. karung) per bahan; 1 unit = base_quantity satuan dasar.
CREATE TABLE ingredient_unit_conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    unit_id SMALLINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    base_quantity NUMERIC(18,6) NOT NULL CHECK (base_quantity > 0),
    label TEXT,
    CONSTRAINT uq_ingredient_unit_conversions UNIQUE (ingredient_id, unit_id)
);

-- Saldo per outlet per bahan; hanya diubah service inventory dalam satu transaksi.
CREATE TABLE inventory (
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    quantity_on_hand NUMERIC(18,4) NOT NULL DEFAULT 0,
    avg_unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (avg_unit_cost >= 0),
    last_purchase_cost NUMERIC(18,6),
    last_movement_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (outlet_id, ingredient_id)
);
CREATE INDEX idx_inventory_low ON inventory (outlet_id, quantity_on_hand);
CREATE INDEX idx_inventory_ingredient ON inventory (ingredient_id);

-- Buku besar stok, append-only. Koreksi lewat movement kebalikan.
CREATE TABLE stock_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    movement_type TEXT NOT NULL CHECK (movement_type IN (
        'INITIAL_STOCK', 'PURCHASE', 'SALE', 'SALE_REVERSAL', 'WASTE',
        'WASTE_REVERSAL', 'ADJUSTMENT', 'RETURN', 'TRANSFER_OUT', 'TRANSFER_IN'
    )),
    quantity_delta NUMERIC(18,4) NOT NULL CHECK (quantity_delta <> 0),
    unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    total_cost NUMERIC(14,2) NOT NULL DEFAULT 0,
    balance_after NUMERIC(18,4) NOT NULL,
    avg_cost_after NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (avg_cost_after >= 0),
    reference_type TEXT NOT NULL,
    reference_id UUID,
    is_shortage BOOLEAN NOT NULL DEFAULT false,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_stock_movements_reason CHECK (
        movement_type NOT IN ('ADJUSTMENT', 'WASTE') OR reason IS NOT NULL
    )
);
CREATE INDEX idx_stock_movements_ingredient ON stock_movements (ingredient_id, outlet_id, created_at DESC);
CREATE INDEX idx_stock_movements_reference ON stock_movements (reference_type, reference_id);
CREATE INDEX idx_stock_movements_type ON stock_movements (movement_type, created_at);
-- Cegah adjustment opname ganda per bahan per sesi.
CREATE UNIQUE INDEX uq_stock_movements_opname
    ON stock_movements (reference_type, reference_id, ingredient_id)
    WHERE movement_type = 'ADJUSTMENT' AND reference_type = 'STOCK_OPNAME';
-- Cegah penerimaan ganda per baris.
CREATE UNIQUE INDEX uq_stock_movements_purchase
    ON stock_movements (reference_type, reference_id, ingredient_id, movement_type)
    WHERE reference_type = 'PURCHASE';

CREATE OR REPLACE FUNCTION fn_deny_stock_movement_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'stock_movements bersifat append-only; % ditolak', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TRIGGER trg_stock_movements_append_only
    BEFORE UPDATE OR DELETE ON stock_movements
    FOR EACH STATEMENT EXECUTE FUNCTION fn_deny_stock_movement_mutation();

-- Append-only.
CREATE TABLE ingredient_cost_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    source TEXT NOT NULL CHECK (source IN ('PURCHASE', 'ADJUSTMENT')),
    reference_id UUID,
    unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
    avg_cost_after NUMERIC(18,6) NOT NULL DEFAULT 0,
    quantity_received NUMERIC(18,4) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ingredient_cost_history_ingredient ON ingredient_cost_history (ingredient_id, created_at DESC);

-- FK tertunda: tabel ingredients baru ada sekarang.
ALTER TABLE products
    ADD CONSTRAINT fk_products_stock_ingredient
    FOREIGN KEY (stock_ingredient_id) REFERENCES ingredients(id) ON DELETE RESTRICT;

-- Bila produk dilacak sebagai stok, bahan stoknya wajib diisi.
ALTER TABLE products
    ADD CONSTRAINT chk_products_stock_ingredient_required
    CHECK (stock_mode <> 'TRACK_AS_STOCK_ITEM' OR stock_ingredient_id IS NOT NULL);

ALTER TABLE modifier_option_recipe_items
    ADD CONSTRAINT fk_modifier_recipe_items_ingredient
    FOREIGN KEY (ingredient_id) REFERENCES ingredients(id) ON DELETE RESTRICT;

ALTER TABLE modifier_option_recipe_items
    ADD CONSTRAINT fk_modifier_recipe_items_replaces
    FOREIGN KEY (replaces_ingredient_id) REFERENCES ingredients(id) ON DELETE RESTRICT;

ALTER TABLE order_item_consumptions
    ADD CONSTRAINT fk_order_item_consumptions_ingredient
    FOREIGN KEY (ingredient_id) REFERENCES ingredients(id) ON DELETE RESTRICT;

ALTER TABLE order_item_consumptions
    ADD CONSTRAINT fk_order_item_consumptions_movement
    FOREIGN KEY (stock_movement_id) REFERENCES stock_movements(id) ON DELETE RESTRICT;
