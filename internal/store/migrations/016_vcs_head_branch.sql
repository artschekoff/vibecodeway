-- +goose Up
ALTER TABLE vcs_pull_requests ADD COLUMN head_branch TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is irreversible.
