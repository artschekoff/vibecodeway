-- +goose Up
ALTER TABLE vcs_pull_requests ADD COLUMN is_draft INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite cannot drop columns; is_draft remains but is unused after rollback
