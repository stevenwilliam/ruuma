-- 0007 materialised slots and phase-2 delivery zones (BR-2.3.x, D16)

CREATE TABLE slots (
  id                     UUID PRIMARY KEY,
  store_id               UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  business_date          DATE NOT NULL,
  fulfilment_type        TEXT NOT NULL CHECK (fulfilment_type IN ('pickup','delivery')),
  starts_at              TIMESTAMPTZ NOT NULL,
  ends_at                TIMESTAMPTZ NOT NULL,
  max_orders             INT NOT NULL CHECK (max_orders >= 0),
  max_kitchen_units      INT NOT NULL CHECK (max_kitchen_units >= 0),
  reserved_orders        INT NOT NULL DEFAULT 0 CHECK (reserved_orders >= 0),
  reserved_kitchen_units INT NOT NULL DEFAULT 0 CHECK (reserved_kitchen_units >= 0),
  is_locked              BOOLEAN NOT NULL DEFAULT false,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- one row per physical window (BR-2.3.4)
  UNIQUE (store_id, business_date, fulfilment_type, starts_at),
  -- the database's own refusal to oversell (BR-2.3.9); the application takes
  -- SELECT ... FOR UPDATE first (BR-2.3.8), so this is the backstop
  CONSTRAINT slots_no_oversell_orders CHECK (reserved_orders <= max_orders),
  CONSTRAINT slots_no_oversell_units  CHECK (reserved_kitchen_units <= max_kitchen_units),
  CHECK (ends_at > starts_at)
);
CREATE INDEX idx_slots_lookup ON slots (store_id, business_date, fulfilment_type, starts_at);
CREATE INDEX idx_slots_starts ON slots (starts_at);

-- Phase 2 (D16): modelled now, disabled by fulfilment.delivery_enabled.
CREATE TABLE delivery_zones (
  id             UUID PRIMARY KEY,
  store_id       UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  fee            BIGINT NOT NULL CHECK (fee >= 0),
  min_order      BIGINT NOT NULL DEFAULT 0 CHECK (min_order >= 0),
  free_threshold BIGINT CHECK (free_threshold IS NULL OR free_threshold >= 0),
  is_active      BOOLEAN NOT NULL DEFAULT true,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, name)
);
