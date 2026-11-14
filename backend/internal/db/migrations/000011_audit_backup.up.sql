-- F20/F21 Audit log (append-only) & catatan backup

CREATE TABLE audit_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    business_id UUID,
    outlet_id UUID,
    user_id UUID,
    user_roles_snapshot TEXT[],
    action TEXT NOT NULL,
    entity_type TEXT,
    entity_id TEXT,
    before_data JSONB,
    after_data JSONB,
    reason TEXT,
    ip_address INET,
    user_agent TEXT,
    request_id TEXT,
    self_approved BOOLEAN NOT NULL DEFAULT false,
    prev_hash BYTEA,
    hash BYTEA
);
CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_logs_user ON audit_logs (user_id, occurred_at);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, occurred_at);
CREATE INDEX idx_audit_logs_outlet ON audit_logs (outlet_id, occurred_at, action);

CREATE OR REPLACE FUNCTION fn_deny_audit_log_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs bersifat append-only; % ditolak', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TRIGGER trg_audit_logs_append_only
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION fn_deny_audit_log_mutation();

CREATE TABLE backup_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL CHECK (type IN ('SCHEDULED', 'MANUAL')),
    status TEXT NOT NULL,
    file_name TEXT NOT NULL,
    size_bytes BIGINT CHECK (size_bytes IS NULL OR size_bytes >= 0),
    sha256 TEXT,
    encrypted BOOLEAN NOT NULL DEFAULT false,
    storage_location TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    restore_test_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_backup_records_started ON backup_records (started_at DESC);
