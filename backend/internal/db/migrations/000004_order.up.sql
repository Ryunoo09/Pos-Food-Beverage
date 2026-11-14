-- F04/F05/F15 Penjualan: meja, pesanan, item, modifier, konsumsi bahan, diskon
-- Catatan: FK orders.shift_id -> shifts (009) dan FK order_item_consumptions.ingredient_id
-- / stock_movement_id (006) ditambahkan pada migrasi terkait setelah tabel target ada.
-- Snapshot harga/HPP disimpan di order_items & order_item_consumptions (BR-TX-1).

CREATE TABLE tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    area TEXT,
    number TEXT NOT NULL,
    capacity INT CHECK (capacity IS NULL OR capacity > 0),
    status TEXT NOT NULL DEFAULT 'AVAILABLE'
        CHECK (status IN ('AVAILABLE', 'OCCUPIED', 'RESERVED', 'CLEANING')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tables_outlet_number UNIQUE (outlet_id, number)
);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    order_no TEXT,
    queue_no INT,
    service_type TEXT NOT NULL CHECK (service_type IN ('DINE_IN', 'TAKEAWAY', 'PICKUP', 'DELIVERY')),
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'CONFIRMED', 'IN_PROGRESS', 'READY', 'SERVED', 'COMPLETED', 'CANCELLED')),
    payment_status TEXT NOT NULL DEFAULT 'UNPAID'
        CHECK (payment_status IN ('UNPAID', 'PENDING', 'PARTIALLY_PAID', 'PAID', 'FAILED', 'REFUNDED', 'PARTIALLY_REFUNDED')),
    payment_timing TEXT NOT NULL CHECK (payment_timing IN ('PAY_FIRST', 'PAY_LATER')),
    table_id UUID REFERENCES tables(id) ON DELETE RESTRICT,
    customer_name TEXT,
    customer_phone TEXT,
    delivery_address TEXT,
    delivery_fee NUMERIC(14,2),
    note TEXT,
    priority TEXT NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('NORMAL', 'RUSH')),
    subtotal_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (subtotal_amount >= 0),
    item_discount_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (item_discount_amount >= 0),
    order_discount_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (order_discount_amount >= 0),
    service_charge_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
    service_charge_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (service_charge_amount >= 0),
    tax_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    tax_inclusive BOOLEAN NOT NULL DEFAULT false,
    rounding_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
    total_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    paid_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
    refunded_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (refunded_amount >= 0),
    total_cogs NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (total_cogs >= 0),
    shift_id UUID,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    confirmed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    cancelled_by UUID REFERENCES users(id) ON DELETE SET NULL,
    confirmed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancel_reason_code TEXT,
    cancel_reason_note TEXT,
    pricing_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_orders_outlet_order_no ON orders (outlet_id, order_no) WHERE order_no IS NOT NULL;
-- Satu pesanan aktif per meja (BR-OP-1).
CREATE UNIQUE INDEX uq_orders_table_active ON orders (table_id)
    WHERE status NOT IN ('COMPLETED', 'CANCELLED') AND table_id IS NOT NULL;
CREATE INDEX idx_orders_outlet_status ON orders (outlet_id, status);
CREATE INDEX idx_orders_outlet_created ON orders (outlet_id, created_at);
CREATE INDEX idx_orders_outlet_confirmed ON orders (outlet_id, confirmed_at);
CREATE INDEX idx_orders_outlet_completed ON orders (outlet_id, completed_at);
CREATE INDEX idx_orders_created_by ON orders (created_by, created_at);
CREATE INDEX idx_orders_shift ON orders (shift_id);

-- order_id CASCADE hanya untuk DRAFT; service menjaga agar pesanan non-draft tidak dihapus.
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    round_no INT NOT NULL DEFAULT 1 CHECK (round_no > 0),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    product_name_snapshot TEXT NOT NULL,
    variant_name_snapshot TEXT NOT NULL,
    sku_snapshot TEXT,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
    modifiers_total NUMERIC(14,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    line_total NUMERIC(14,2) NOT NULL DEFAULT 0,
    unit_cogs NUMERIC(14,4) NOT NULL DEFAULT 0 CHECK (unit_cogs >= 0),
    line_cogs NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (line_cogs >= 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'VOID')),
    void_reason TEXT,
    voided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    note TEXT,
    cost_unknown BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_items_order ON order_items (order_id);
CREATE INDEX idx_order_items_product ON order_items (product_id, created_at);

CREATE TABLE order_item_modifiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    option_id UUID REFERENCES modifier_options(id) ON DELETE SET NULL,
    group_name_snapshot TEXT,
    option_name_snapshot TEXT NOT NULL,
    price_delta NUMERIC(14,2) NOT NULL DEFAULT 0,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0)
);
CREATE INDEX idx_order_item_modifiers_item ON order_item_modifiers (order_item_id);

-- Snapshot konsumsi bahan & biaya (sumber HPP dan dasar reversal).
-- FK ingredient_id dan stock_movement_id ditambahkan di migrasi 006.
CREATE TABLE order_item_consumptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('RECIPE', 'MODIFIER', 'STOCK_ITEM')),
    quantity_base NUMERIC(18,4) NOT NULL CHECK (quantity_base >= 0),
    unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    total_cost NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (total_cost >= 0),
    stock_movement_id UUID,
    reversed_quantity_base NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK (reversed_quantity_base >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_item_consumptions_item ON order_item_consumptions (order_item_id);
CREATE INDEX idx_order_item_consumptions_ingredient ON order_item_consumptions (ingredient_id);
CREATE UNIQUE INDEX uq_order_item_consumptions_movement
    ON order_item_consumptions (stock_movement_id) WHERE stock_movement_id IS NOT NULL;

-- Append-only.
CREATE TABLE order_status_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status TEXT NOT NULL,
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT
);
CREATE INDEX idx_order_status_histories_order ON order_status_histories (order_id, changed_at);

-- Append-only.
CREATE TABLE order_item_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('ADDED', 'VOIDED', 'QTY_REDUCED', 'NOTE_CHANGED')),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_item_events_item ON order_item_events (order_item_id, created_at);

CREATE TABLE discounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    discount_type TEXT NOT NULL CHECK (discount_type IN ('PERCENT', 'AMOUNT')),
    value NUMERIC(14,4) NOT NULL DEFAULT 0 CHECK (value >= 0),
    scope TEXT NOT NULL CHECK (scope IN ('ITEM', 'ORDER')),
    min_spend NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (min_spend >= 0),
    max_discount NUMERIC(14,2),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    days_of_week INT[],
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_discounts_outlet_code UNIQUE (outlet_id, code)
);

CREATE TABLE order_discounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    order_item_id UUID REFERENCES order_items(id) ON DELETE CASCADE,
    discount_id UUID REFERENCES discounts(id) ON DELETE SET NULL,
    name_snapshot TEXT NOT NULL,
    discount_type TEXT NOT NULL CHECK (discount_type IN ('PERCENT', 'AMOUNT')),
    value NUMERIC(14,4) NOT NULL DEFAULT 0,
    amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (amount >= 0),
    applied_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    self_approved BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_discounts_order ON order_discounts (order_id);
