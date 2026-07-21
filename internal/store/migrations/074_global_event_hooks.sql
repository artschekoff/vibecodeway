-- +goose Up
CREATE TABLE IF NOT EXISTS global_event_hooks (
    id             TEXT PRIMARY KEY,
    event_category TEXT NOT NULL,
    action_type    TEXT NOT NULL,
    script_command TEXT NOT NULL DEFAULT '',
    enabled        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    UNIQUE (event_category, action_type)
);

-- +goose Down
DROP TABLE IF EXISTS global_event_hooks;
