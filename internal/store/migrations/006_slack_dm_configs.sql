-- +goose Up

-- DM load configurations — one row per (context, workspace).
-- Stores the start_from date used when calling `slack-cli list-dms --start-from`.
-- Content is refreshed on each Reload/Refresh cycle.
CREATE TABLE IF NOT EXISTS slack_dm_configs (
    id         TEXT PRIMARY KEY,
    context_id TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    workspace  TEXT NOT NULL,
    start_from TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    is_active  INTEGER NOT NULL DEFAULT 1,
    UNIQUE(context_id, workspace)
);

-- +goose Down
DROP TABLE IF EXISTS slack_dm_configs;
