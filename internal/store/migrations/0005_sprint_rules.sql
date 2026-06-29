-- +goose Up
-- 0005_sprint_rules.sql — sprint additions: title, goal, duration, deliverables, templates.

ALTER TABLE sprints ADD COLUMN title         TEXT;
ALTER TABLE sprints ADD COLUMN goal          TEXT;
ALTER TABLE sprints ADD COLUMN duration_days INTEGER
       CHECK (duration_days IN (3,5,7,14));
ALTER TABLE sprints ADD COLUMN starts_on     TEXT;
ALTER TABLE sprints ADD COLUMN ends_on       TEXT;

CREATE TABLE deliverables (
    id         INTEGER PRIMARY KEY,
    sprint_id  INTEGER NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    is_done    INTEGER NOT NULL DEFAULT 0 CHECK (is_done IN (0,1)),
    done_at    TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX ix_deliverables_sprint ON deliverables(sprint_id);

CREATE TABLE sprint_templates (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    duration_days INTEGER CHECK (duration_days IN (3,5,7,14)),
    phase_notes   TEXT,
    deliverables  TEXT,
    created_at    TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS sprint_templates;
DROP INDEX IF EXISTS ix_deliverables_sprint;
DROP TABLE IF EXISTS deliverables;
-- SQLite doesn't support DROP COLUMN, so we leave the ALTER TABLE additions in place on down.
