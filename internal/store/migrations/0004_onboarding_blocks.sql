-- +goose Up
-- 0004_onboarding_blocks.sql — onboarding state, user blocks, and user platforms.

CREATE TABLE onboarding_state (
    user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','role_done','blocks_done','complete')),
    updated_at TEXT NOT NULL
);

CREATE TABLE user_blocks (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    block_key  TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, block_key)
);

CREATE TABLE user_platforms (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform  TEXT NOT NULL
                CHECK (platform IN ('linkedin','x','blog','instagram','tiktok')),
    PRIMARY KEY (user_id, platform)
);

-- +goose Down
DROP TABLE IF EXISTS user_platforms;
DROP TABLE IF EXISTS user_blocks;
DROP TABLE IF EXISTS onboarding_state;
