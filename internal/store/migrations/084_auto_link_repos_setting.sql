-- +goose Up
-- Auto-link matched repos when a new context is created. 1 = enabled (default ON).
ALTER TABLE app_settings ADD COLUMN auto_link_repos_on_context_create INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE app_settings DROP COLUMN auto_link_repos_on_context_create;
