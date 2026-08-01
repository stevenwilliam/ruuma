DROP TRIGGER IF EXISTS audit_log_append_only ON audit_log;
DROP TRIGGER IF EXISTS payment_events_append_only ON payment_events;
DROP TRIGGER IF EXISTS order_events_append_only ON order_events;
DROP FUNCTION IF EXISTS refuse_mutation();
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS audit_log;
