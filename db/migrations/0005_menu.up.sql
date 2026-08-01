-- 0005 menu: categories, items, option groups and choices (BR-2.2.x)
-- Money is BIGINT whole rupiah; no NUMERIC, no FLOAT anywhere near a price
-- (BR-1.1.1, BR-1.1.2).

CREATE TABLE categories (
  id         UUID PRIMARY KEY,
  name_id    TEXT NOT NULL,
  name_en    TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,
  cuisine    TEXT NOT NULL CHECK (cuisine IN ('indonesian','chinese','western','other')),
  sort_order INT NOT NULL DEFAULT 0,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE menu_items (
  id               UUID PRIMARY KEY,
  category_id      UUID NOT NULL REFERENCES categories(id),
  sku              TEXT NOT NULL UNIQUE,
  name_id          TEXT NOT NULL,
  name_en          TEXT NOT NULL,
  description_id   TEXT,
  description_en   TEXT,
  base_price       BIGINT NOT NULL CHECK (base_price >= 0),
  kitchen_units    INT NOT NULL DEFAULT 1 CHECK (kitchen_units > 0),
  prep_minutes     INT NOT NULL DEFAULT 10 CHECK (prep_minutes >= 0),
  min_lead_minutes INT NOT NULL DEFAULT 0 CHECK (min_lead_minutes >= 0),
  photo_key        TEXT,
  spice_level      SMALLINT NOT NULL DEFAULT 0 CHECK (spice_level BETWEEN 0 AND 3),
  is_halal         BOOLEAN NOT NULL DEFAULT true,
  is_vegetarian    BOOLEAN NOT NULL DEFAULT false,
  contains_pork    BOOLEAN NOT NULL DEFAULT false,
  contains_alcohol BOOLEAN NOT NULL DEFAULT false,
  contains_nuts    BOOLEAN NOT NULL DEFAULT false,
  is_active        BOOLEAN NOT NULL DEFAULT true,
  sort_order       INT NOT NULL DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_menu_items_category ON menu_items (category_id) WHERE is_active;
CREATE INDEX idx_menu_items_search ON menu_items USING gin (
  to_tsvector('simple', name_id || ' ' || name_en || ' ' || coalesce(description_id,'') || ' ' || coalesce(description_en,''))
);

CREATE TABLE option_groups (
  id           UUID PRIMARY KEY,
  menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  name_id      TEXT NOT NULL,
  name_en      TEXT NOT NULL,
  selection    TEXT NOT NULL CHECK (selection IN ('single','multi')),
  is_required  BOOLEAN NOT NULL DEFAULT false,
  min_select   SMALLINT NOT NULL DEFAULT 0 CHECK (min_select >= 0),
  max_select   SMALLINT NOT NULL DEFAULT 1 CHECK (max_select >= 1),
  sort_order   INT NOT NULL DEFAULT 0,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (max_select >= min_select),
  CHECK (selection <> 'single' OR max_select = 1)
);
CREATE INDEX idx_option_groups_item ON option_groups (menu_item_id);

CREATE TABLE option_choices (
  id              UUID PRIMARY KEY,
  option_group_id UUID NOT NULL REFERENCES option_groups(id) ON DELETE CASCADE,
  name_id         TEXT NOT NULL,
  name_en         TEXT NOT NULL,
  price_delta     BIGINT NOT NULL DEFAULT 0,
  kitchen_units   INT NOT NULL DEFAULT 0 CHECK (kitchen_units >= 0),
  is_available    BOOLEAN NOT NULL DEFAULT true,
  sort_order      INT NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_option_choices_group ON option_choices (option_group_id);

CREATE TABLE favourites (
  id           UUID PRIMARY KEY,
  customer_id  UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (customer_id, menu_item_id)
);
