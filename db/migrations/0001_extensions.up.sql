-- 0001 extensions
-- citext gives case-insensitive email and promo-code comparison without
-- lower() wrappers that defeat indexes.
CREATE EXTENSION IF NOT EXISTS citext;
