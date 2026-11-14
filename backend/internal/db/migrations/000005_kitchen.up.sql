-- F06 Kitchen Display System

CREATE TABLE kitchen_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    round_no INT NOT NULL DEFAULT 1 CHECK (round_no > 0),
    status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'COOKING', 'READY', 'SERVED', 'VOID')),
    priority TEXT NOT NULL DEFAULT 'NORMAL' CHECK (priority IN ('NORMAL', 'RUSH')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    ready_at TIMESTAMPTZ,
    served_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_kitchen_orders_order_round UNIQUE (order_id, round_no)
);
CREATE INDEX idx_kitchen_orders_outlet_status ON kitchen_orders (outlet_id, status, received_at);

CREATE TABLE kitchen_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kitchen_order_id UUID NOT NULL REFERENCES kitchen_orders(id) ON DELETE CASCADE,
    order_item_id UUID REFERENCES order_items(id) ON DELETE CASCADE,
    name_snapshot TEXT NOT NULL,
    variant_snapshot TEXT,
    modifiers_text TEXT,
    note TEXT,
    quantity INT NOT NULL CHECK (quantity > 0),
    station TEXT,
    status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'COOKING', 'READY', 'SERVED', 'VOID')),
    started_at TIMESTAMPTZ,
    ready_at TIMESTAMPTZ,
    served_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_kitchen_order_items_order_item ON kitchen_order_items (order_item_id) WHERE order_item_id IS NOT NULL;
CREATE INDEX idx_kitchen_order_items_status ON kitchen_order_items (status);
CREATE INDEX idx_kitchen_order_items_kitchen ON kitchen_order_items (kitchen_order_id);
