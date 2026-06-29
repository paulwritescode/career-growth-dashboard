-- +goose Up
-- 0008_api_ingest.sql — API-pushed metric points and trace spans.

CREATE TABLE metric_points (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    value       REAL NOT NULL,
    tags        TEXT,
    occurred_at TEXT NOT NULL
);

CREATE INDEX ix_metric_points_name_time ON metric_points(name, occurred_at);

CREATE TABLE trace_spans (
    id          INTEGER PRIMARY KEY,
    sprint_id   INTEGER REFERENCES sprints(id) ON DELETE CASCADE,
    phase       INTEGER CHECK (phase BETWEEN 1 AND 4),
    name        TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    started_at  TEXT NOT NULL
);

CREATE INDEX ix_trace_spans_sprint ON trace_spans(sprint_id);

-- +goose Down
DROP INDEX IF EXISTS ix_trace_spans_sprint;
DROP TABLE IF EXISTS trace_spans;
DROP INDEX IF EXISTS ix_metric_points_name_time;
DROP TABLE IF EXISTS metric_points;
