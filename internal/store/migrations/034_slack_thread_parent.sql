-- +goose Up
ALTER TABLE slack_threads ADD COLUMN parent_thread_ts TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; no-op rollback
