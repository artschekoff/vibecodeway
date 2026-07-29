-- +goose Up
-- User-managed commands shown in the editor terminal's command selector. The
-- command text may contain placeholder tokens (e.g. <context-name>) substituted
-- at insert time. Selecting a command types it into the terminal (no Enter).
CREATE TABLE IF NOT EXISTS terminal_commands (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    command    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE terminal_commands;
