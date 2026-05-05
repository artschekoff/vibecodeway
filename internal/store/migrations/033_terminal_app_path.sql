-- +goose Up
ALTER TABLE app_settings ADD COLUMN terminal_app_path TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_settings DROP COLUMN terminal_app_path;
