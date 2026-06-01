-- +goose Up
CREATE TABLE context_status_log (
    id          TEXT PRIMARY KEY,
    context_id  TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    status_slug TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'ai',  -- 'ai' | 'user'
    prev_id     TEXT REFERENCES context_status_log(id),
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_context_status_log_context ON context_status_log(context_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_context_status_log_context;
DROP TABLE context_status_log;
