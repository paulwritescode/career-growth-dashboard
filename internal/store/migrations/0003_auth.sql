-- +goose Up
-- 0003_auth.sql — users, sessions, and api_keys for phase-2 auth.

CREATE TABLE users (
    id              INTEGER PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL DEFAULT '',
    avatar_initials TEXT NOT NULL DEFAULT '',
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'other'
                      CHECK (role IN ('backend','frontend','fullstack','devops',
                                      'data','indie','manager','other')),
    role_other      TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    remember     INTEGER NOT NULL DEFAULT 0 CHECK (remember IN (0,1))
);

CREATE INDEX ix_sessions_user ON sessions(user_id);
CREATE INDEX ix_sessions_expires ON sessions(expires_at);

CREATE TABLE api_keys (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label        TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    key_prefix   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    last_used_at TEXT,
    revoked_at   TEXT
);

CREATE INDEX ix_api_keys_user ON api_keys(user_id);

-- +goose Down
DROP INDEX IF EXISTS ix_api_keys_user;
DROP TABLE IF EXISTS api_keys;
DROP INDEX IF EXISTS ix_sessions_expires;
DROP INDEX IF EXISTS ix_sessions_user;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
