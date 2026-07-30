-- +goose Up
-- When set, selecting the command also sends Enter so it runs immediately
-- instead of just being typed into the terminal.
ALTER TABLE terminal_commands ADD COLUMN send_enter INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE terminal_commands DROP COLUMN send_enter;
