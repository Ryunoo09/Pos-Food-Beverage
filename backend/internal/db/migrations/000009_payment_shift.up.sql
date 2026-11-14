-- F14/F16 Pembayaran, refund, idempotency, shift kas, cash movement
-- FK orders.shift_id ditambahkan di akhir setelah tabel shifts ada.

CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    requires_reference BOOLEAN NOT NULL DEFAULT false,
    is_cash BOOLEAN NOT NULL DEFAULT false,
    provider TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payment_methods_outlet_code UNIQUE (outlet_id, code)
);

CREATE TABLE shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'REVIEWED')),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    opening_cash NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (opening_cash >= 0),
    closed_at TIMESTAMPTZ,
    expected_cash NUMERIC(14,2),
    actual_cash NUMERIC(14,2),
    difference NUMERIC(14,2),
    difference_reason TEXT,
    summary JSONB,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Satu shift OPEN per kasir.
CREATE UNIQUE INDEX uq_shifts_user_open ON shifts (user_id) WHERE status = 'OPEN';
CREATE INDEX idx_shifts_outlet_status ON shifts (outlet_id, status, opened_at DESC);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE RESTRICT,
    method_code TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    cash_received NUMERIC(14,2),
    change_amount NUMERIC(14,2),
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'SUCCESS', 'FAILED', 'EXPIRED', 'VOIDED')),
    reference_no TEXT,
    external_reference TEXT,
    provider_payload JSONB,
    idempotency_key TEXT NOT NULL,
    paid_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    voided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    void_reason TEXT,
    confirmation_type TEXT CHECK (confirmation_type IN ('MANUAL', 'GATEWAY', 'MANUAL_CONFIRMATION')),
    received_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_payments_outlet_idempotency UNIQUE (outlet_id, idempotency_key)
);
CREATE UNIQUE INDEX uq_payments_external_reference ON payments (external_reference) WHERE external_reference IS NOT NULL;
CREATE INDEX idx_payments_order ON payments (order_id);
CREATE INDEX idx_payments_shift_status ON payments (shift_id, status);
CREATE INDEX idx_payments_outlet_paid ON payments (outlet_id, paid_at);

CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE RESTRICT,
    refund_type TEXT NOT NULL CHECK (refund_type IN ('FULL', 'PARTIAL')),
    method_code TEXT,
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    reason_code TEXT,
    reason_note TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING_APPROVAL'
        CHECK (status IN ('PENDING_APPROVAL', 'APPROVED', 'COMPLETED', 'REJECTED')),
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    self_approved BOOLEAN NOT NULL DEFAULT false,
    idempotency_key TEXT NOT NULL,
    restock_decision JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT uq_refunds_idempotency UNIQUE (idempotency_key)
);
CREATE INDEX idx_refunds_order ON refunds (order_id);
CREATE INDEX idx_refunds_shift ON refunds (shift_id, status);

CREATE TABLE refund_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    refund_id UUID NOT NULL REFERENCES refunds(id) ON DELETE CASCADE,
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK (quantity > 0),
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    restock BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT uq_refund_items_refund_order_item UNIQUE (refund_id, order_item_id)
);

-- scope = 'outlet_id:endpoint:user'; retensi dibersihkan job harian.
CREATE TABLE idempotency_keys (
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    response_status INT,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope, key)
);
CREATE INDEX idx_idempotency_keys_expires ON idempotency_keys (expires_at);

CREATE TABLE cash_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE RESTRICT,
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    type TEXT NOT NULL CHECK (type IN ('IN', 'OUT')),
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    category TEXT,
    reason TEXT,
    expense_id UUID,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_cash_movements_expense UNIQUE (expense_id)
);
CREATE INDEX idx_cash_movements_shift ON cash_movements (shift_id, created_at);

ALTER TABLE orders
    ADD CONSTRAINT fk_orders_shift
    FOREIGN KEY (shift_id) REFERENCES shifts(id) ON DELETE RESTRICT;
