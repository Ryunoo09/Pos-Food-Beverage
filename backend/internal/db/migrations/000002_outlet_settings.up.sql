-- F02 Pengaturan outlet, pajak, dan penomoran dokumen

CREATE TABLE outlet_settings (
    outlet_id UUID PRIMARY KEY REFERENCES outlets(id) ON DELETE CASCADE,
    payment_timing_dine_in TEXT NOT NULL DEFAULT 'PAY_LATER'
        CHECK (payment_timing_dine_in IN ('PAY_FIRST', 'PAY_LATER')),
    payment_timing_takeaway TEXT NOT NULL DEFAULT 'PAY_FIRST'
        CHECK (payment_timing_takeaway IN ('PAY_FIRST', 'PAY_LATER')),
    payment_timing_pickup TEXT NOT NULL DEFAULT 'PAY_FIRST'
        CHECK (payment_timing_pickup IN ('PAY_FIRST', 'PAY_LATER')),
    payment_timing_delivery TEXT NOT NULL DEFAULT 'PAY_FIRST'
        CHECK (payment_timing_delivery IN ('PAY_FIRST', 'PAY_LATER')),
    allow_negative_stock BOOLEAN NOT NULL DEFAULT true,
    allow_multiple_open_shifts BOOLEAN NOT NULL DEFAULT false,
    require_recipe_for_sale BOOLEAN NOT NULL DEFAULT false,
    tax_inclusive BOOLEAN NOT NULL DEFAULT false,
    tax_on_service BOOLEAN NOT NULL DEFAULT false,
    rounding_mode TEXT NOT NULL DEFAULT 'NONE'
        CHECK (rounding_mode IN ('NONE', 'UP_100', 'NEAREST_100', 'NEAREST_500')),
    discount_cashier_max_percent NUMERIC(7,4) NOT NULL DEFAULT 10
        CHECK (discount_cashier_max_percent >= 0),
    discount_approval_threshold_amount NUMERIC(14,2) NOT NULL DEFAULT 50000,
    refund_approval_threshold NUMERIC(14,2) NOT NULL DEFAULT 100000,
    po_approval_threshold NUMERIC(14,2) NOT NULL DEFAULT 0,
    adjustment_approval_threshold NUMERIC(14,2) NOT NULL DEFAULT 500000,
    business_day_cutoff TIME NOT NULL DEFAULT '00:00',
    kitchen_age_warn_minutes INT NOT NULL DEFAULT 5,
    kitchen_age_late_minutes INT NOT NULL DEFAULT 10,
    mgr_can_change_price BOOLEAN NOT NULL DEFAULT false,
    mgr_can_view_profit BOOLEAN NOT NULL DEFAULT true,
    allow_self_approval_opname BOOLEAN NOT NULL DEFAULT false,
    allow_self_approval_po BOOLEAN NOT NULL DEFAULT false,
    allow_self_approval_adjustment BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tarif pajak/service tidak ditetapkan sistem; perubahan membuat baris baru (riwayat).
CREATE TABLE tax_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    type TEXT NOT NULL CHECK (type IN ('TAX', 'SERVICE')),
    name TEXT NOT NULL,
    rate_percent NUMERIC(7,4) NOT NULL CHECK (rate_percent >= 0),
    applies_to_service_types TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT true,
    effective_from DATE NOT NULL DEFAULT current_date,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tax_configs_outlet_type_active ON tax_configs (outlet_id, type, is_active);

-- Penomoran atomik via INSERT ... ON CONFLICT DO UPDATE SET last_number = last_number + 1.
CREATE TABLE document_sequences (
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
    doc_type TEXT NOT NULL,
    period TEXT NOT NULL,
    last_number INT NOT NULL DEFAULT 0 CHECK (last_number >= 0),
    PRIMARY KEY (outlet_id, doc_type, period)
);
