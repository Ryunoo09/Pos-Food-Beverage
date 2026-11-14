-- F10/F13/F17 Stock opname, waste, pengeluaran
-- FK cash_movements.expense_id ditambahkan setelah tabel expenses ada.

CREATE TABLE stock_opnames (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'IN_PROGRESS', 'SUBMITTED', 'APPROVED', 'REJECTED', 'COMPLETED', 'CANCELLED')),
    scope TEXT NOT NULL DEFAULT 'ALL' CHECK (scope IN ('ALL', 'SELECTED')),
    started_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    submitted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    rejected_reason TEXT,
    note TEXT,
    version INT NOT NULL DEFAULT 1,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_stock_opnames_outlet_code UNIQUE (outlet_id, code)
);
CREATE INDEX idx_stock_opnames_outlet_status ON stock_opnames (outlet_id, status, created_at DESC);

CREATE TABLE stock_opname_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stock_opname_id UUID NOT NULL REFERENCES stock_opnames(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    system_quantity_snapshot NUMERIC(18,4) NOT NULL DEFAULT 0,
    snapshot_at TIMESTAMPTZ,
    counted_quantity NUMERIC(18,4),
    counted_at TIMESTAMPTZ,
    counted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expected_quantity_at_approval NUMERIC(18,4),
    difference_quantity NUMERIC(18,4),
    difference_value NUMERIC(14,2),
    reason TEXT,
    adjustment_movement_id UUID REFERENCES stock_movements(id) ON DELETE RESTRICT,
    is_active_session BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_stock_opname_items_opname_ingredient UNIQUE (stock_opname_id, ingredient_id),
    CONSTRAINT uq_stock_opname_items_movement UNIQUE (adjustment_movement_id)
);
-- Satu bahan hanya boleh ada pada satu sesi opname aktif.
CREATE UNIQUE INDEX uq_stock_opname_items_active
    ON stock_opname_items (ingredient_id) WHERE is_active_session;

CREATE TABLE waste_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    waste_type TEXT NOT NULL CHECK (waste_type IN ('INGREDIENT', 'PRODUCT')),
    ingredient_id UUID REFERENCES ingredients(id) ON DELETE RESTRICT,
    variant_id UUID REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    unit_id SMALLINT REFERENCES units(id) ON DELETE RESTRICT,
    reason_code TEXT,
    note TEXT,
    cost_value NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (cost_value >= 0),
    order_id UUID REFERENCES orders(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'POSTED' CHECK (status IN ('POSTED', 'REVERSED', 'PENDING_APPROVAL')),
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reversal_of UUID REFERENCES waste_records(id) ON DELETE RESTRICT,
    recorded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_waste_records_target CHECK (
        (waste_type = 'INGREDIENT' AND ingredient_id IS NOT NULL)
        OR (waste_type = 'PRODUCT' AND variant_id IS NOT NULL)
    )
);
CREATE INDEX idx_waste_records_outlet_recorded ON waste_records (outlet_id, recorded_at);
CREATE INDEX idx_waste_records_reason ON waste_records (reason_code);

CREATE TABLE expense_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_expense_categories_business_name ON expense_categories (business_id, lower(name));

CREATE TABLE expenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    category_id UUID NOT NULL REFERENCES expense_categories(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    expense_date DATE NOT NULL DEFAULT current_date,
    payment_method_code TEXT,
    paid_from_drawer BOOLEAN NOT NULL DEFAULT false,
    shift_id UUID REFERENCES shifts(id) ON DELETE RESTRICT,
    note TEXT,
    attachment_url TEXT,
    is_voided BOOLEAN NOT NULL DEFAULT false,
    void_reason TEXT,
    recorded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_expenses_outlet_date ON expenses (outlet_id, expense_date);

ALTER TABLE cash_movements
    ADD CONSTRAINT fk_cash_movements_expense
    FOREIGN KEY (expense_id) REFERENCES expenses(id) ON DELETE RESTRICT;
