-- +goose Up
ALTER TABLE app_settings ADD COLUMN coding_agent_app TEXT NOT NULL DEFAULT 'cursor';

-- +goose Down
ALTER TABLE app_settings DROP COLUMN coding_agent_app;
