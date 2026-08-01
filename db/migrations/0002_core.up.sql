-- 0002 staff users + system parameters (BR-1.4, BR-2.7.6)

CREATE TABLE users (
  id                   UUID PRIMARY KEY,
  email                CITEXT NOT NULL UNIQUE,
  password_hash        TEXT NOT NULL,
  full_name            TEXT NOT NULL,
  phone                TEXT,
  role                 TEXT NOT NULL CHECK (role IN
                       ('kitchen','counter','finance','store_manager','admin','owner')),
  is_group_scope       BOOLEAN NOT NULL DEFAULT false,
  is_active            BOOLEAN NOT NULL DEFAULT true,
  must_change_password BOOLEAN NOT NULL DEFAULT false,
  failed_attempts      INT NOT NULL DEFAULT 0,
  locked_until         TIMESTAMPTZ,
  last_login_at        TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_role ON users (role) WHERE is_active;

CREATE TABLE sys_parameters (
  id                   UUID PRIMARY KEY,
  key                  TEXT NOT NULL UNIQUE,
  value                TEXT NOT NULL,
  data_type            TEXT NOT NULL DEFAULT 'string'
                       CHECK (data_type IN ('string','int','bool','decimal','json')),
  description          TEXT,
  is_secret            BOOLEAN NOT NULL DEFAULT false,
  is_store_overridable BOOLEAN NOT NULL DEFAULT false,
  updated_by           UUID REFERENCES users(id),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sys_parameters_key ON sys_parameters (key);
