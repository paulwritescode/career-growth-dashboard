-- +goose Up
-- 0001_init.sql — core tables for local-scava.
-- Pragmas (WAL, foreign_keys, busy_timeout) are set by the Go layer on connect,
-- not here.

CREATE TABLE sprints (
    id                  INTEGER PRIMARY KEY,
    month_label         TEXT NOT NULL,
    skill_name          TEXT NOT NULL,
    skill_rationale     TEXT NOT NULL DEFAULT '',
    microapp_one_liner  TEXT NOT NULL,
    core_feature        TEXT NOT NULL,
    out_of_scope        TEXT NOT NULL DEFAULT '',
    deploy_platform     TEXT NOT NULL DEFAULT '',
    current_phase       INTEGER NOT NULL DEFAULT 1 CHECK (current_phase BETWEEN 1 AND 4),
    status              TEXT NOT NULL DEFAULT 'planned'
                          CHECK (status IN ('planned','active','shipped','abandoned')),
    live_url            TEXT,
    declaration_post_id INTEGER,
    retro_worked        TEXT NOT NULL DEFAULT '',
    retro_differently   TEXT NOT NULL DEFAULT '',
    retro_learned       TEXT NOT NULL DEFAULT '',
    retro_live_link     TEXT NOT NULL DEFAULT '',
    started_on          TEXT,
    ended_on            TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (declaration_post_id) REFERENCES posts(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_sprints_single_active ON sprints (status) WHERE status = 'active';
CREATE INDEX ix_sprints_status ON sprints(status);

CREATE TABLE checklist_items (
    id          INTEGER PRIMARY KEY,
    sprint_id   INTEGER NOT NULL,
    phase       INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 4),
    label       TEXT NOT NULL,
    is_done     INTEGER NOT NULL DEFAULT 0 CHECK (is_done IN (0,1)),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    done_at     TEXT,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE CASCADE
);

CREATE INDEX ix_checklist_sprint_phase ON checklist_items(sprint_id, phase, sort_order);

CREATE TABLE daily_logs (
    id               INTEGER PRIMARY KEY,
    sprint_id        INTEGER,
    log_date         TEXT NOT NULL,
    worked_on        TEXT NOT NULL,
    what_happened    TEXT NOT NULL DEFAULT '',
    insight          TEXT NOT NULL DEFAULT '',
    next_up          TEXT NOT NULL DEFAULT '',
    blocker          TEXT NOT NULL DEFAULT '',
    blocker_decision TEXT CHECK (blocker_decision IN ('solve','workaround','cut') OR blocker_decision IS NULL),
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_daily_logs_sprint_date ON daily_logs(sprint_id, log_date);
CREATE INDEX ix_daily_logs_date ON daily_logs(log_date);

CREATE TABLE posts (
    id             INTEGER PRIMARY KEY,
    sprint_id      INTEGER,
    source_log_id  INTEGER,
    post_date      TEXT NOT NULL,
    post_type      TEXT NOT NULL DEFAULT 'daily' CHECK (post_type IN ('daily','recap')),
    title          TEXT NOT NULL DEFAULT '',
    is_declaration INTEGER NOT NULL DEFAULT 0 CHECK (is_declaration IN (0,1)),
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    FOREIGN KEY (sprint_id)     REFERENCES sprints(id)    ON DELETE SET NULL,
    FOREIGN KEY (source_log_id) REFERENCES daily_logs(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_posts_date ON posts(post_date);
CREATE INDEX ix_posts_sprint ON posts(sprint_id);

CREATE TABLE adrs (
    id           INTEGER PRIMARY KEY,
    sprint_id    INTEGER,
    number       INTEGER NOT NULL,
    title        TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed','decided','superseded')),
    decided_on   TEXT,
    problem      TEXT NOT NULL DEFAULT '',
    options      TEXT NOT NULL DEFAULT '',
    decision     TEXT NOT NULL DEFAULT '',
    why          TEXT NOT NULL DEFAULT '',
    consequences TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (sprint_id) REFERENCES sprints(id) ON DELETE SET NULL
);

CREATE INDEX ix_adrs_sprint ON adrs(sprint_id);

CREATE TABLE post_tiers (
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

CREATE UNIQUE INDEX ux_post_tiers_post_tier ON post_tiers(post_id, tier);
CREATE INDEX ix_post_tiers_status ON post_tiers(status);

-- +goose Down
DROP TABLE IF EXISTS post_tiers;
DROP TABLE IF EXISTS adrs;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS daily_logs;
DROP TABLE IF EXISTS checklist_items;
DROP TABLE IF EXISTS sprints;
