-- +goose Up

-- Add is_active flag to all item tables so individual items can be excluded from
-- MCP context output without being deleted.
ALTER TABLE jira_issues   ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE jira_comments ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE slack_channels ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE slack_threads  ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE context_files  ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;

-- +goose Down

-- SQLite does not support DROP COLUMN on older versions; recreate tables if needed.
-- For simplicity the down migration is left as a no-op (data preserved, flag ignored).
SELECT 1;
