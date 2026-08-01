-- 0010 payments and their append-only event log (BR-2.6.x)
-- Phase 1 is manual bank transfer; qris and gateway exist in the enum so the
-- provider port has somewhere to land (D25).

CREATE TABLE payments (
  id                UUID PRIMARY KEY,
  order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  store_id          UUID NOT NULL REFERENCES stores(id),
  method            TEXT NOT NULL DEFAULT 'manual_transfer'
                    CHECK (method IN ('manual_transfer','qris','gateway')),
  status            TEXT NOT NULL CHECK (status IN
                    ('PENDING','SUBMITTED','VERIFIED','REJECTED','REFUNDED')),
  amount_due        BIGINT NOT NULL CHECK (amount_due >= 0),
  declared_amount   BIGINT CHECK (declared_amount IS NULL OR declared_amount >= 0),
  sender_name       TEXT,
  bank_account_id   UUID REFERENCES store_bank_accounts(id),
  proof_object_key  TEXT,
  proof_uploaded_at TIMESTAMPTZ,
  verified_by       UUID REFERENCES users(id),
  verified_at       TIMESTAMPTZ,
  mismatch_accepted BOOLEAN NOT NULL DEFAULT false,
  mismatch_reason   TEXT,
  rejection_reason  TEXT CHECK (rejection_reason IS NULL OR rejection_reason IN
                    ('AMOUNT_MISMATCH','PROOF_UNREADABLE','NOT_RECEIVED','DUPLICATE','OTHER')),
  rejection_note    TEXT,
  rejected_by       UUID REFERENCES users(id),
  rejected_at       TIMESTAMPTZ,
  refunded_amount   BIGINT CHECK (refunded_amount IS NULL OR refunded_amount >= 0),
  refund_reference  TEXT,
  refund_proof_key  TEXT,
  refunded_by       UUID REFERENCES users(id),
  refunded_at       TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- a rejection without a reason is not a rejection (BR-2.6.8)
  CHECK (status <> 'REJECTED' OR rejection_reason IS NOT NULL),
  CHECK (status <> 'VERIFIED' OR (verified_by IS NOT NULL AND verified_at IS NOT NULL))
);
CREATE INDEX idx_payments_queue ON payments (store_id, status, proof_uploaded_at);
CREATE INDEX idx_payments_order ON payments (order_id);

-- append-only (BR-2.6.10)
CREATE TABLE payment_events (
  id         UUID PRIMARY KEY,
  payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
  order_id   UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN
             ('PROOF_SUBMITTED','VERIFIED','REJECTED','MISMATCH_ACCEPTED','REFUNDED')),
  actor_id   UUID,
  actor_role TEXT,
  amount     BIGINT,
  reason     TEXT,
  metadata   JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payment_events_payment ON payment_events (payment_id, created_at);
