-- +goose Up
-- 0006_habits.sql — habit tracker block tables.

CREATE TABLE habits (
    id             INTEGER PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    icon           TEXT,
    sprint_linked  INTEGER NOT NULL DEFAULT 0 CHECK (sprint_linked IN (0,1)),
    archived       INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1)),
    created_at     TEXT NOT NULL
);

CREATE TABLE habit_entries (
    habit_id   INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    entry_date TEXT NOT NULL,
    PRIMARY KEY (habit_id, entry_date)
);

-- +goose Down
DROP TABLE IF EXISTS habit_entries;
DROP TABLE IF EXISTS habits;
