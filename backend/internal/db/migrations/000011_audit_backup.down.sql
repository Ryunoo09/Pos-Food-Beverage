DROP TABLE IF EXISTS backup_records;
DROP TRIGGER IF EXISTS trg_audit_logs_append_only ON audit_logs;
DROP FUNCTION IF EXISTS fn_deny_audit_log_mutation();
DROP TABLE IF EXISTS audit_logs;
