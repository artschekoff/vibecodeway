-- +goose Up
ALTER TABLE vcs_repos ADD COLUMN on_checkout_command TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; intentional no-op.
