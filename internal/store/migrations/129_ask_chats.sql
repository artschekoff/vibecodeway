-- +goose Up

-- Ask AI conversations, stored whole (the frontend's log JSON) so a chat can be
-- reopened and continued, and dumped via the CLI for prompt debugging.
CREATE TABLE IF NOT EXISTS ask_chats (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL DEFAULT '',
    log_json   TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ask_chats_updated_at ON ask_chats(updated_at);

-- How long Ask AI chats are kept before being pruned (days).
ALTER TABLE app_settings ADD COLUMN ask_chat_retention_days INTEGER NOT NULL DEFAULT 30;

-- +goose Down

DROP TABLE IF EXISTS ask_chats;
ALTER TABLE app_settings DROP COLUMN ask_chat_retention_days;
