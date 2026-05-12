-- +goose Up
CREATE TABLE sync_sessions (
    id          TEXT PRIMARY KEY,
    context_id  TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    started_at  TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at TEXT,
    sources     TEXT NOT NULL DEFAULT '[]',  -- JSON array of source names
    items_fetched INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX sync_sessions_context_idx ON sync_sessions(context_id, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS sync_sessions_context_idx;
DROP TABLE sync_sessions;
