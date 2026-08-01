-- 0003 stores and store master data (BR-2.1.x)
-- Every store-scoped table below carries store_id NOT NULL with an index and
-- per-store uniqueness — store scope is a tenancy boundary (BR-2.1.1, BR-2.7.8).

CREATE TABLE stores (
  id            UUID PRIMARY KEY,
  code          TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL,
  slug          TEXT NOT NULL UNIQUE,
  address_line  TEXT NOT NULL,
  city          TEXT NOT NULL,
  province      TEXT,
  postal_code   TEXT,
  latitude      NUMERIC(9,6),
  longitude     NUMERIC(9,6),
  phone         TEXT NOT NULL,
  timezone      TEXT NOT NULL DEFAULT 'Asia/Jakarta',
  is_active     BOOLEAN NOT NULL DEFAULT true,
  ticket_header TEXT,
  ticket_footer TEXT,
  sort_order    INT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_stores_active ON stores (is_active);

CREATE TABLE store_fulfilment_modes (
  id              UUID PRIMARY KEY,
  store_id        UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  fulfilment_type TEXT NOT NULL CHECK (fulfilment_type IN ('pickup','delivery')),
  is_enabled      BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, fulfilment_type)
);

-- Weekday x mode x block. A closed weekday generates no slots at all (BR-2.1.4);
-- modes may differ within a store (BR-2.1.5).
CREATE TABLE store_hours (
  id              UUID PRIMARY KEY,
  store_id        UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  weekday         SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
  fulfilment_type TEXT NOT NULL CHECK (fulfilment_type IN ('pickup','delivery')),
  block_index     SMALLINT NOT NULL DEFAULT 0,
  is_closed       BOOLEAN NOT NULL DEFAULT false,
  opens_at        TIME,
  closes_at       TIME,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, weekday, fulfilment_type, block_index),
  CHECK (is_closed OR (opens_at IS NOT NULL AND closes_at IS NOT NULL AND closes_at > opens_at))
);
CREATE INDEX idx_store_hours_lookup ON store_hours (store_id, weekday, fulfilment_type);

-- One specific date differs from the weekday pattern (BR-2.1.6, D18).
CREATE TABLE store_date_overrides (
  id              UUID PRIMARY KEY,
  store_id        UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  business_date   DATE NOT NULL,
  fulfilment_type TEXT NOT NULL CHECK (fulfilment_type IN ('pickup','delivery')),
  block_index     SMALLINT NOT NULL DEFAULT 0,
  is_closed       BOOLEAN NOT NULL DEFAULT false,
  opens_at        TIME,
  closes_at       TIME,
  reason          TEXT,
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, business_date, fulfilment_type, block_index),
  CHECK (is_closed OR (opens_at IS NOT NULL AND closes_at IS NOT NULL AND closes_at > opens_at))
);
CREATE INDEX idx_store_date_overrides_lookup ON store_date_overrides (store_id, business_date);

-- Emergency closure; may target the current date (BR-2.1.7, D27). Deliberately
-- no CHECK against past/today: closing today is the whole point.
CREATE TABLE store_blackout_dates (
  id            UUID PRIMARY KEY,
  store_id      UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  business_date DATE NOT NULL,
  reason        TEXT NOT NULL,
  created_by    UUID REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, business_date)
);
CREATE INDEX idx_store_blackouts_lookup ON store_blackout_dates (store_id, business_date);

CREATE TABLE store_bank_accounts (
  id             UUID PRIMARY KEY,
  store_id       UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  bank_name      TEXT NOT NULL,
  account_name   TEXT NOT NULL,
  account_number TEXT NOT NULL,
  is_primary     BOOLEAN NOT NULL DEFAULT false,
  is_active      BOOLEAN NOT NULL DEFAULT true,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_store_bank_primary ON store_bank_accounts (store_id) WHERE is_primary;

-- Per-store override of a sys_parameters key (BR-1.4.4).
CREATE TABLE store_parameters (
  id         UUID PRIMARY KEY,
  store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  key        TEXT NOT NULL,
  value      TEXT NOT NULL,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, key)
);

CREATE TABLE staff_store_assignments (
  id         UUID PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  store_id   UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, store_id)
);
CREATE INDEX idx_staff_assignments_user ON staff_store_assignments (user_id);
CREATE INDEX idx_staff_assignments_store ON staff_store_assignments (store_id);
