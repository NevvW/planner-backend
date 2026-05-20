CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
                       id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
                       email citext UNIQUE NOT NULL,
                       password_hash bytea NOT NULL,
                       created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_sessions (
                                  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
                                  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                  token_hash bytea UNIQUE NOT NULL,
                                  expires_at timestamptz NOT NULL,
                                  absolute_expires_at timestamptz NOT NULL,
                                  revoked_at timestamptz NULL,
                                  created_at timestamptz NOT NULL DEFAULT now(),
                                  ip text NOT NULL DEFAULT '',
                                  user_agent text NOT NULL DEFAULT '',
                                  CONSTRAINT chk_abs_ge_exp CHECK (absolute_expires_at >= expires_at)
);

CREATE INDEX idx_refresh_sessions_user_id ON refresh_sessions(user_id);
CREATE INDEX idx_refresh_sessions_expires_at ON refresh_sessions(expires_at);