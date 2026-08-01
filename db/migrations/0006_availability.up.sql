-- 0006 per-store availability: overrides, 86s, rules, daily stock (BR-2.2.x)

-- Store override beats the group default; NULL inherits (BR-2.2.1).
CREATE TABLE store_menu_overrides (
  id             UUID PRIMARY KEY,
  store_id       UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  menu_item_id   UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  is_available   BOOLEAN,
  price_override BIGINT CHECK (price_override IS NULL OR price_override >= 0),
  updated_by     UUID REFERENCES users(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, menu_item_id)
);
CREATE INDEX idx_store_menu_overrides_store ON store_menu_overrides (store_id);

-- Scheduled "86": out of stock for a window, at one store (BR-2.2.3).
CREATE TABLE item_86s (
  id           UUID PRIMARY KEY,
  store_id     UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  starts_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  ends_at      TIMESTAMPTZ,
  reason       TEXT,
  created_by   UUID REFERENCES users(id),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (ends_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX idx_item_86s_lookup ON item_86s (store_id, menu_item_id, starts_at, ends_at);

-- Weekend-only or lunch-only dishes; store_id NULL means every store (BR-2.2.7).
CREATE TABLE item_availability_rules (
  id           UUID PRIMARY KEY,
  menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  store_id     UUID REFERENCES stores(id) ON DELETE CASCADE,
  weekday_mask SMALLINT NOT NULL DEFAULT 127 CHECK (weekday_mask BETWEEN 0 AND 127),
  from_time    TIME,
  to_time      TIME,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (from_time IS NULL OR to_time IS NULL OR to_time > from_time)
);
CREATE INDEX idx_item_rules_item ON item_availability_rules (menu_item_id);

-- Daily countdown; the CHECK makes the database refuse to sell past zero
-- (BR-2.2.4).
CREATE TABLE item_daily_stock (
  id            UUID PRIMARY KEY,
  store_id      UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  menu_item_id  UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  business_date DATE NOT NULL,
  stock_total   INT NOT NULL CHECK (stock_total >= 0),
  stock_used    INT NOT NULL DEFAULT 0 CHECK (stock_used >= 0),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, menu_item_id, business_date),
  CONSTRAINT item_daily_stock_no_oversell CHECK (stock_used <= stock_total)
);
