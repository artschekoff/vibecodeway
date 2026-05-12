-- +goose Up

ALTER TABLE context_snapshots ADD COLUMN estimated_hours           REAL;
ALTER TABLE context_snapshots ADD COLUMN estimated_completion_date TEXT;
ALTER TABLE context_snapshots ADD COLUMN estimation_confidence     TEXT NOT NULL DEFAULT '';
ALTER TABLE context_snapshots ADD COLUMN estimation_basis          TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS estimation_log (
    id                          TEXT PRIMARY KEY,
    context_id                  TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    recorded_at                 TEXT NOT NULL,
    trigger_type                TEXT NOT NULL,
    trigger_description         TEXT NOT NULL DEFAULT '',
    delta_hours                 REAL,
    estimated_hours_before      REAL,
    estimated_hours_after       REAL,
    estimated_completion_before TEXT,
    estimated_completion_after  TEXT,
    estimation_basis            TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS estimation_log_context_id
    ON estimation_log(context_id, recorded_at DESC);

-- +goose Down

DROP TABLE IF EXISTS estimation_log;
-- SQLite does not support DROP COLUMN; estimation cols remain (harmless).
