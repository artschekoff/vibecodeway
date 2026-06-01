-- +goose Up
CREATE TABLE IF NOT EXISTS context_status_log (
    id          TEXT PRIMARY KEY,
    context_id  TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    status_slug TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'ai',  -- 'ai' | 'user'
    prev_id     TEXT REFERENCES context_status_log(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_context_status_log_context ON context_status_log(context_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_context_status_log_context;
DROP TABLE IF EXISTS context_status_log;
