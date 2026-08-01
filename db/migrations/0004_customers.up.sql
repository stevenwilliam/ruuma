-- 0004 customers, identities and auth support (BR-2.7.1-5, D24)

CREATE TABLE customers (
  id                 UUID PRIMARY KEY,
  full_name          TEXT NOT NULL,
  email              CITEXT UNIQUE,
  email_verified_at  TIMESTAMPTZ,
  phone              TEXT UNIQUE,
  phone_verified_at  TIMESTAMPTZ,          -- gates ordering (BR-2.7.4)
  password_hash      TEXT,                 -- NULL for social-only accounts
  preferred_language TEXT NOT NULL DEFAULT 'id' CHECK (preferred_language IN ('id','en')),
  marketing_opt_in   BOOLEAN NOT NULL DEFAULT false,
  is_active          BOOLEAN NOT NULL DEFAULT true,
  failed_attempts    INT NOT NULL DEFAULT 0,
  locked_until       TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One customer, several sign-in methods. Linking requires verified_at
-- (BR-2.7.3) — an unverified claim never joins an existing account.
CREATE TABLE customer_identities (
  id               UUID PRIMARY KEY,
  customer_id      UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  provider         TEXT NOT NULL CHECK (provider IN ('password','google','instagram','phone')),
  provider_user_id TEXT NOT NULL,
  email            CITEXT,
  verified_at      TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id)
);
CREATE INDEX idx_customer_identities_customer ON customer_identities (customer_id);

CREATE TABLE addresses (
  id           UUID PRIMARY KEY,
  customer_id  UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  label        TEXT NOT NULL,
  recipient    TEXT NOT NULL,
  phone        TEXT NOT NULL,
  address_line TEXT NOT NULL,
  area         TEXT,
  city         TEXT NOT NULL,
  postal_code  TEXT,
  notes        TEXT,
  is_default   BOOLEAN NOT NULL DEFAULT false,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_addresses_customer ON addresses (customer_id);

-- OTP codes are hashed, single-use, expiring and attempt-capped (BR-2.7.5).
CREATE TABLE otp_codes (
  id          UUID PRIMARY KEY,
  phone       TEXT NOT NULL,
  code_hash   TEXT NOT NULL,
  purpose     TEXT NOT NULL CHECK (purpose IN ('signup','login','verify_phone')),
  attempts    INT NOT NULL DEFAULT 0,
  consumed_at TIMESTAMPTZ,
  expires_at  TIMESTAMPTZ NOT NULL,
  request_ip  INET,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_otp_phone ON otp_codes (phone, created_at DESC);

CREATE TABLE verification_tokens (
  id           UUID PRIMARY KEY,
  subject_type TEXT NOT NULL CHECK (subject_type IN ('customer','user')),
  subject_id   UUID NOT NULL,
  token_hash   TEXT NOT NULL UNIQUE,
  purpose      TEXT NOT NULL CHECK (purpose IN ('verify_email','reset_password')),
  consumed_at  TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_verification_subject ON verification_tokens (subject_type, subject_id);

-- Rotating, hashed, revocable refresh tokens with a rotation chain (BR-2.7.12).
CREATE TABLE refresh_tokens (
  id           UUID PRIMARY KEY,
  subject_type TEXT NOT NULL CHECK (subject_type IN ('customer','user')),
  subject_id   UUID NOT NULL,
  token_hash   TEXT NOT NULL UNIQUE,
  jti          UUID NOT NULL UNIQUE,
  parent_jti   UUID,
  user_agent   TEXT,
  ip           INET,
  revoked_at   TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_subject ON refresh_tokens (subject_type, subject_id) WHERE revoked_at IS NULL;
