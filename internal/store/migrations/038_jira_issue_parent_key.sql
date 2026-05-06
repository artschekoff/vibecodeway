-- +goose Up
ALTER TABLE jira_issues ADD COLUMN parent_key TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN directly in older versions; migration is intentionally irreversible.
