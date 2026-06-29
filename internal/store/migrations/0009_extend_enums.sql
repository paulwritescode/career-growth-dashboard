-- +goose Up
-- +goose NO TRANSACTION
-- 0009_extend_enums.sql — widen career_events.source and post_tiers.tier via table rebuild.

-- Step 1: Rebuild career_events with extended source enum.
CREATE TABLE career_events_new (
    id          INTEGER PRIMARY KEY,
    occurred_at TEXT NOT NULL,
    kind        TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT 'form' CHECK (source IN ('form','chat','system','api')),
    sprint_id   INTEGER,
    post_id     INTEGER,
    summary     TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE SET NULL,
    FOREIGN KEY (post_id)   REFERENCES posts(id)   ON DELETE SET NULL
);

INSERT INTO career_events_new SELECT * FROM career_events;
DROP TABLE career_events;
ALTER TABLE career_events_new RENAME TO career_events;

CREATE INDEX ix_career_events_time ON career_events(occurred_at);
CREATE INDEX ix_career_events_kind ON career_events(kind);

-- Step 2: Rebuild post_tiers with extended tier enum.
CREATE TABLE post_tiers_new (
    id           INTEGER PRIMARY KEY,
    post_id      INTEGER NOT NULL,
    tier         TEXT NOT NULL CHECK (tier IN ('blog','linkedin','x','instagram','tiktok')),
    status       TEXT NOT NULL DEFAULT 'none' CHECK (status IN ('none','drafted','published')),
    content      TEXT NOT NULL DEFAULT '',
    url          TEXT,
    published_at TEXT,
    visual_kind  TEXT NOT NULL DEFAULT 'none' CHECK (visual_kind IN ('none','adr','diagram','screenshot')),
    adr_id       INTEGER,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (adr_id)  REFERENCES adrs(id)  ON DELETE SET NULL
);

INSERT INTO post_tiers_new SELECT * FROM post_tiers;
DROP TABLE post_tiers;
ALTER TABLE post_tiers_new RENAME TO post_tiers;

CREATE UNIQUE INDEX ux_post_tiers_post_tier ON post_tiers(post_id, tier);
CREATE INDEX ix_post_tiers_status ON post_tiers(status);

-- +goose Down
-- Reverse: rebuild tables back to original CHECK constraints.
-- career_events back to original 3-value source enum.
CREATE TABLE career_events_old (
    id          INTEGER PRIMARY KEY,
    occurred_at TEXT NOT NULL,
    kind        TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT 'form' CHECK (source IN ('form','chat','system')),
    sprint_id   INTEGER,
    post_id     INTEGER,
    summary     TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE SET NULL,
    FOREIGN KEY (post_id)   REFERENCES posts(id)   ON DELETE SET NULL
);

INSERT INTO career_events_old SELECT * FROM career_events WHERE source != 'api';
DROP TABLE career_events;
ALTER TABLE career_events_old RENAME TO career_events;
CREATE INDEX ix_career_events_time ON career_events(occurred_at);
CREATE INDEX ix_career_events_kind ON career_events(kind);

-- post_tiers back to original 3-value tier enum.
CREATE TABLE post_tiers_old (
    id           INTEGER PRIMARY KEY,
    post_id      INTEGER NOT NULL,
    tier         TEXT NOT NULL CHECK (tier IN ('blog','linkedin','x')),
    status       TEXT NOT NULL DEFAULT 'none' CHECK (status IN ('none','drafted','published')),
    content      TEXT NOT NULL DEFAULT '',
    url          TEXT,
    published_at TEXT,
    visual_kind  TEXT NOT NULL DEFAULT 'none' CHECK (visual_kind IN ('none','adr','diagram','screenshot')),
    adr_id       INTEGER,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (adr_id)  REFERENCES adrs(id)  ON DELETE SET NULL
);

INSERT INTO post_tiers_old SELECT * FROM post_tiers WHERE tier IN ('blog','linkedin','x');
DROP TABLE post_tiers;
ALTER TABLE post_tiers_old RENAME TO post_tiers;
CREATE UNIQUE INDEX ux_post_tiers_post_tier ON post_tiers(post_id, tier);
CREATE INDEX ix_post_tiers_status ON post_tiers(status);
