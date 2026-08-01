-- 0008 promotions with an explicit store list — no implicit "all stores" (D15)

CREATE TABLE promotions (
  id                     UUID PRIMARY KEY,
  code                   CITEXT NOT NULL UNIQUE,
  name                   TEXT NOT NULL,
  discount_type          TEXT NOT NULL CHECK (discount_type IN ('percent','fixed')),
  value_bps              INT CHECK (value_bps IS NULL OR value_bps BETWEEN 0 AND 10000),
  value_amount           BIGINT CHECK (value_amount IS NULL OR value_amount >= 0),
  max_discount           BIGINT CHECK (max_discount IS NULL OR max_discount >= 0),
  min_spend              BIGINT NOT NULL DEFAULT 0 CHECK (min_spend >= 0),
  starts_at              TIMESTAMPTZ NOT NULL,
  ends_at                TIMESTAMPTZ NOT NULL,
  usage_cap_total        INT CHECK (usage_cap_total IS NULL OR usage_cap_total > 0),
  usage_cap_per_customer INT CHECK (usage_cap_per_customer IS NULL OR usage_cap_per_customer > 0),
  used_count             INT NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  is_active              BOOLEAN NOT NULL DEFAULT true,
  created_by             UUID REFERENCES users(id),
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at),
  CHECK ((discount_type = 'percent' AND value_bps IS NOT NULL)
      OR (discount_type = 'fixed'   AND value_amount IS NOT NULL)),
  CHECK (usage_cap_total IS NULL OR used_count <= usage_cap_total)
);

CREATE TABLE promotion_stores (
  id           UUID PRIMARY KEY,
  promotion_id UUID NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
  store_id     UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  UNIQUE (promotion_id, store_id)
);

CREATE TABLE promotion_categories (
  id           UUID PRIMARY KEY,
  promotion_id UUID NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
  category_id  UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  UNIQUE (promotion_id, category_id)
);
