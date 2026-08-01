-- 0009 orders, lines, options, events and promotion redemptions
-- (BR-2.4.x, BR-2.5.x, BR-2.6.2)

CREATE TABLE orders (
  id                   UUID PRIMARY KEY,
  order_code           TEXT NOT NULL UNIQUE,
  store_id             UUID NOT NULL REFERENCES stores(id),
  customer_id          UUID NOT NULL REFERENCES customers(id),
  slot_id              UUID NOT NULL REFERENCES slots(id),
  fulfilment_type      TEXT NOT NULL CHECK (fulfilment_type IN ('pickup','delivery')),
  business_date        DATE NOT NULL,
  slot_starts_at       TIMESTAMPTZ NOT NULL,
  slot_ends_at         TIMESTAMPTZ NOT NULL,
  status               TEXT NOT NULL CHECK (status IN
                       ('DRAFT','PENDING_PAYMENT','AWAITING_VERIFICATION','PAID','ACCEPTED',
                        'IN_KITCHEN','READY','PICKED_UP','OUT_FOR_DELIVERY','DELIVERED',
                        'COMPLETED','CANCELLED','REFUNDED')),
  contact_name         TEXT NOT NULL,
  contact_phone        TEXT NOT NULL,
  address_id           UUID REFERENCES addresses(id),
  delivery_zone_id     UUID REFERENCES delivery_zones(id),
  notes                TEXT,
  subtotal             BIGINT NOT NULL CHECK (subtotal >= 0),
  discount             BIGINT NOT NULL DEFAULT 0 CHECK (discount >= 0),
  service_charge       BIGINT NOT NULL DEFAULT 0 CHECK (service_charge >= 0),
  tax                  BIGINT NOT NULL DEFAULT 0 CHECK (tax >= 0),
  delivery_fee         BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
  total                BIGINT NOT NULL CHECK (total >= 0),
  unique_code          INT NOT NULL CHECK (unique_code BETWEEN 1 AND 999),
  amount_due           BIGINT NOT NULL CHECK (amount_due >= 0),
  tax_bps              INT NOT NULL,
  service_charge_bps   INT NOT NULL,
  promotion_id         UUID REFERENCES promotions(id),
  promo_code           TEXT,
  kitchen_units        INT NOT NULL DEFAULT 0 CHECK (kitchen_units >= 0),
  cancelled_reason     TEXT,
  cancelled_by         UUID,
  capacity_released_at TIMESTAMPTZ,
  placed_at            TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (discount <= subtotal),                                              -- BR-2.5.4
  CHECK (total = subtotal - discount + service_charge + tax + delivery_fee),  -- BR-2.5.7
  CHECK (amount_due = total + unique_code)                                    -- BR-2.6.2
);
CREATE INDEX idx_orders_store_date ON orders (store_id, business_date, slot_starts_at);
CREATE INDEX idx_orders_slot ON orders (slot_id);
CREATE INDEX idx_orders_customer ON orders (customer_id, created_at DESC);
CREATE INDEX idx_orders_status ON orders (store_id, status);
CREATE INDEX idx_orders_code ON orders (order_code);
-- one live kode unik per store so finance can match a transfer (BR-2.6.2)
CREATE UNIQUE INDEX idx_orders_open_unique_code ON orders (store_id, unique_code)
  WHERE status IN ('PENDING_PAYMENT','AWAITING_VERIFICATION');

CREATE TABLE order_lines (
  id            UUID PRIMARY KEY,
  order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  menu_item_id  UUID NOT NULL REFERENCES menu_items(id),
  item_name_id  TEXT NOT NULL,
  item_name_en  TEXT NOT NULL,
  unit_price    BIGINT NOT NULL CHECK (unit_price >= 0),
  qty           INT NOT NULL CHECK (qty > 0),
  options_delta BIGINT NOT NULL DEFAULT 0,
  line_total    BIGINT NOT NULL CHECK (line_total >= 0),
  kitchen_units INT NOT NULL DEFAULT 0 CHECK (kitchen_units >= 0),
  notes         TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- the arithmetic itself is a constraint (BR-2.5.2)
  CHECK (line_total = (unit_price + options_delta) * qty)
);
CREATE INDEX idx_order_lines_order ON order_lines (order_id);

CREATE TABLE order_line_options (
  id               UUID PRIMARY KEY,
  order_line_id    UUID NOT NULL REFERENCES order_lines(id) ON DELETE CASCADE,
  option_group_id  UUID NOT NULL REFERENCES option_groups(id),
  option_choice_id UUID NOT NULL REFERENCES option_choices(id),
  group_name_id    TEXT NOT NULL,
  choice_name_id   TEXT NOT NULL,
  choice_name_en   TEXT NOT NULL,
  price_delta      BIGINT NOT NULL DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_line_options_line ON order_line_options (order_line_id);

-- append-only (BR-2.4.4); the trigger in 0011 enforces it for every role
CREATE TABLE order_events (
  id          UUID PRIMARY KEY,
  order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  from_status TEXT,
  to_status   TEXT NOT NULL,
  actor_type  TEXT NOT NULL CHECK (actor_type IN ('customer','staff','system')),
  actor_id    UUID,
  reason      TEXT,
  metadata    JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_events_order ON order_events (order_id, created_at);

CREATE TABLE promotion_redemptions (
  id           UUID PRIMARY KEY,
  promotion_id UUID NOT NULL REFERENCES promotions(id),
  order_id     UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  customer_id  UUID NOT NULL REFERENCES customers(id),
  store_id     UUID NOT NULL REFERENCES stores(id),
  discount     BIGINT NOT NULL CHECK (discount >= 0),
  released_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (order_id)
);
CREATE INDEX idx_promo_redemptions_customer ON promotion_redemptions (promotion_id, customer_id)
  WHERE released_at IS NULL;
