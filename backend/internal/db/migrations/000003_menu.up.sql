-- F03 Katalog: kategori, produk, varian, modifier, riwayat harga
-- Catatan: FK ke ingredients (products.stock_ingredient_id, modifier_option_recipe_items)
-- ditambahkan pada 000006_ingredient_inventory setelah tabel ingredients ada.

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_categories_business_name ON categories (business_id, lower(name));

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    product_type TEXT NOT NULL CHECK (product_type IN ('FOOD', 'BEVERAGE', 'OTHER')),
    stock_mode TEXT NOT NULL CHECK (stock_mode IN ('TRACK_BY_RECIPE', 'TRACK_AS_STOCK_ITEM', 'NO_TRACKING')),
    stock_ingredient_id UUID,
    kitchen_station TEXT CHECK (kitchen_station IN ('KITCHEN', 'BAR')),
    image_url TEXT,
    allow_zero_price BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_available BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_products_business_sku UNIQUE (business_id, sku)
);
CREATE INDEX idx_products_category ON products (category_id, is_active);
CREATE INDEX idx_products_active_available ON products (is_active, is_available);
CREATE INDEX idx_products_name_trgm ON products USING gin (name gin_trgm_ops);

-- Harga jual ada di varian; produk tanpa variasi = satu varian default.
CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    name TEXT NOT NULL DEFAULT 'Reguler',
    sku_suffix TEXT,
    price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Satu varian default per produk.
CREATE UNIQUE INDEX uq_product_variants_default ON product_variants (product_id) WHERE is_default;
CREATE INDEX idx_product_variants_product ON product_variants (product_id, is_active);

CREATE TABLE modifier_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    min_select INT NOT NULL DEFAULT 0 CHECK (min_select >= 0),
    max_select INT NOT NULL DEFAULT 1 CHECK (max_select >= 0),
    is_required BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_modifier_groups_select_range CHECK (min_select <= max_select)
);

CREATE TABLE modifier_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES modifier_groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price_delta NUMERIC(14,2) NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_modifier_options_group ON modifier_options (group_id, is_active);

CREATE TABLE product_modifier_groups (
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    modifier_group_id UUID NOT NULL REFERENCES modifier_groups(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, modifier_group_id)
);

-- FK ingredient_id & replaces_ingredient_id ditambahkan di migrasi 006.
CREATE TABLE modifier_option_recipe_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_id UUID NOT NULL REFERENCES modifier_options(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL,
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    mode TEXT NOT NULL DEFAULT 'ADD' CHECK (mode IN ('ADD', 'REPLACE')),
    replaces_ingredient_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_modifier_recipe_replace CHECK (mode <> 'REPLACE' OR replaces_ingredient_id IS NOT NULL)
);
CREATE INDEX idx_modifier_option_recipe_items_option ON modifier_option_recipe_items (option_id);

-- Append-only.
CREATE TABLE price_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('VARIANT', 'MODIFIER_OPTION')),
    entity_id UUID NOT NULL,
    old_price NUMERIC(14,2),
    new_price NUMERIC(14,2) NOT NULL,
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT
);
CREATE INDEX idx_price_histories_entity ON price_histories (entity_type, entity_id, changed_at DESC);
