-- +goose Up
-- 0007_reviews_todos.sql — weekly reviews and todos.

CREATE TABLE weekly_reviews (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    iso_week      TEXT NOT NULL,
    what_shipped  TEXT,
    what_slipped  TEXT,
    carry_forward TEXT,
    one_learning  TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE UNIQUE INDEX ux_weekly_reviews_week ON weekly_reviews(user_id, iso_week);

CREATE TABLE todos (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sprint_id  INTEGER REFERENCES sprints(id) ON DELETE SET NULL,
    text       TEXT NOT NULL,
    priority   TEXT NOT NULL DEFAULT 'normal'
                 CHECK (priority IN ('low','normal','high')),
    status     TEXT NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open','done','dropped')),
    due_on     TEXT,
    created_at TEXT NOT NULL,
    done_at    TEXT
);

CREATE INDEX ix_todos_status ON todos(status);
CREATE INDEX ix_todos_sprint ON todos(sprint_id);

-- +goose Down
DROP INDEX IF EXISTS ix_todos_sprint;
DROP INDEX IF EXISTS ix_todos_status;
DROP TABLE IF EXISTS todos;
DROP INDEX IF EXISTS ux_weekly_reviews_week;
DROP TABLE IF EXISTS weekly_reviews;
