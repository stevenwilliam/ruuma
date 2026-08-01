-- 0011 audit log, notifications, idempotency, and the append-only enforcement
-- (BR-2.10.1, BR-2.10.2, docs/04 §1)

CREATE TABLE audit_log (
  id          UUID PRIMARY KEY,
  actor_type  TEXT NOT NULL CHECK (actor_type IN ('customer','staff','system')),
  actor_id    UUID,
  actor_email TEXT,
  action      TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id   UUID,
  store_id    UUID REFERENCES stores(id),
  before      JSONB,
  after       JSONB,
  ip          INET,
  user_agent  TEXT,
  request_id  TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX idx_audit_actor ON audit_log (actor_id, created_at DESC);
CREATE INDEX idx_audit_store ON audit_log (store_id, created_at DESC);
CREATE INDEX idx_audit_action ON audit_log (action, created_at DESC);

CREATE TABLE notifications (
  id           UUID PRIMARY KEY,
  order_id     UUID REFERENCES orders(id) ON DELETE SET NULL,
  customer_id  UUID REFERENCES customers(id) ON DELETE SET NULL,
  channel      TEXT NOT NULL CHECK (channel IN ('whatsapp','email')),
  provider     TEXT NOT NULL,
  event        TEXT NOT NULL,
  target       TEXT NOT NULL,
  template_key TEXT NOT NULL,
  language     TEXT NOT NULL DEFAULT 'id',
  status       TEXT NOT NULL CHECK (status IN ('queued','sent','failed','skipped')),
  attempts     INT NOT NULL DEFAULT 0,
  last_error   TEXT,
  next_attempt_at TIMESTAMPTZ,
  sent_at      TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_order ON notifications (order_id, created_at);
CREATE INDEX idx_notifications_pending ON notifications (status, next_attempt_at)
  WHERE status IN ('queued','failed');

CREATE TABLE idempotency_keys (
  id            UUID PRIMARY KEY,
  key           TEXT NOT NULL,
  subject_type  TEXT NOT NULL,
  subject_id    UUID NOT NULL,
  endpoint      TEXT NOT NULL,
  request_hash  TEXT NOT NULL,
  response_code INT,
  response_body JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (key, subject_type, subject_id, endpoint)
);
CREATE INDEX idx_idempotency_created ON idempotency_keys (created_at);

-- Append-only enforcement (BR-2.10.2). A trigger, not a GRANT: the owner role
-- would bypass a revoke, and these three tables are financial and legal
-- evidence. Corrections are made by appending a new row, never by editing one.
CREATE OR REPLACE FUNCTION refuse_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'table % is append-only (BR-2.10.2)', TG_TABLE_NAME
    USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TRIGGER order_events_append_only
  BEFORE UPDATE OR DELETE ON order_events
  FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

CREATE TRIGGER payment_events_append_only
  BEFORE UPDATE OR DELETE ON payment_events
  FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

CREATE TRIGGER audit_log_append_only
  BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION refuse_mutation();
