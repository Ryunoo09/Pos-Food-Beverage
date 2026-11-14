-- F11 Supplier & pembelian; harga beli aktual tersimpan di receiving.

CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    contact_person TEXT,
    phone TEXT,
    email TEXT,
    address TEXT,
    note TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_suppliers_business_name ON suppliers (business_id, lower(name));

CREATE TABLE supplier_ingredients (
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    purchase_unit_id SMALLINT REFERENCES units(id) ON DELETE RESTRICT,
    last_price NUMERIC(14,2) CHECK (last_price IS NULL OR last_price >= 0),
    lead_time_days INT CHECK (lead_time_days IS NULL OR lead_time_days >= 0),
    is_preferred BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (supplier_id, ingredient_id)
);

CREATE TABLE purchase_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED', 'PARTIALLY_RECEIVED', 'RECEIVED', 'CANCELLED')),
    order_date DATE NOT NULL DEFAULT current_date,
    expected_date DATE,
    subtotal NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    note TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    submitted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    cancelled_reason TEXT,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_purchase_orders_outlet_code UNIQUE (outlet_id, code)
);
CREATE INDEX idx_purchase_orders_outlet_status ON purchase_orders (outlet_id, status, order_date);
CREATE INDEX idx_purchase_orders_supplier ON purchase_orders (supplier_id);

CREATE TABLE purchase_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    purchase_unit_id SMALLINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    quantity_ordered NUMERIC(18,4) NOT NULL CHECK (quantity_ordered > 0),
    unit_price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
    discount_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    line_total NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (line_total >= 0),
    base_quantity_ordered NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK (base_quantity_ordered >= 0),
    quantity_received_base NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK (quantity_received_base >= 0),
    CONSTRAINT uq_purchase_order_items_po_ingredient UNIQUE (purchase_order_id, ingredient_id)
);

-- PO tidak mengubah stok; penerimaan yang mengubah (BR-INV-4). purchase_order_id NULL untuk penerimaan langsung.
CREATE TABLE purchase_receivings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    purchase_order_id UUID REFERENCES purchase_orders(id) ON DELETE RESTRICT,
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    received_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    note TEXT,
    supplier_invoice_no TEXT,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'POSTED' CHECK (status IN ('POSTED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_purchase_receivings_outlet_code UNIQUE (outlet_id, code),
    CONSTRAINT uq_purchase_receivings_idempotency UNIQUE (idempotency_key)
);

CREATE TABLE purchase_receiving_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receiving_id UUID NOT NULL REFERENCES purchase_receivings(id) ON DELETE CASCADE,
    po_item_id UUID REFERENCES purchase_order_items(id) ON DELETE RESTRICT,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    purchase_unit_id SMALLINT NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    quantity_received NUMERIC(18,4) NOT NULL CHECK (quantity_received > 0),
    base_quantity NUMERIC(18,4) NOT NULL CHECK (base_quantity > 0),
    unit_price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (unit_price >= 0),
    discount_allocated NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_allocated >= 0),
    net_unit_cost_base NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (net_unit_cost_base >= 0),
    line_total NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (line_total >= 0),
    stock_movement_id UUID REFERENCES stock_movements(id) ON DELETE RESTRICT,
    CONSTRAINT uq_purchase_receiving_items_movement UNIQUE (stock_movement_id)
);
CREATE INDEX idx_purchase_receiving_items_receiving ON purchase_receiving_items (receiving_id);
CREATE INDEX idx_purchase_receiving_items_ingredient ON purchase_receiving_items (ingredient_id);
