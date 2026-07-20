-- +goose Up
CREATE TABLE IF NOT EXISTS context_event_hooks (
    id             TEXT PRIMARY KEY,
    context_id     TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    event_category TEXT NOT NULL,
    action_type    TEXT NOT NULL,
    script_command TEXT NOT NULL DEFAULT '',
    enabled        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    UNIQUE (context_id, event_category, action_type)
);

CREATE INDEX IF NOT EXISTS idx_context_event_hooks_context ON context_event_hooks(context_id);

-- +goose Down
DROP INDEX IF EXISTS idx_context_event_hooks_context;
DROP TABLE IF EXISTS context_event_hooks;
