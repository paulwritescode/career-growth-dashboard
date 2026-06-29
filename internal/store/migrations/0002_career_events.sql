-- +goose Up
-- 0002_career_events.sql — append-only audit/log stream for the SRE "logs" view.

CREATE TABLE career_events (
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

CREATE INDEX ix_career_events_time ON career_events(occurred_at);
CREATE INDEX ix_career_events_kind ON career_events(kind);

-- +goose Down
DROP TABLE IF EXISTS career_events;
