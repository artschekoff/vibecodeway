-- +goose Up
ALTER TABLE app_logs ADD COLUMN fields TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_logs DROP COLUMN fields;
