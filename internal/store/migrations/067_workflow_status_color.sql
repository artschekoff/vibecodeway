-- +goose Up
ALTER TABLE workflow_statuses ADD COLUMN color TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot drop columns; color column remains but is empty after rollback
