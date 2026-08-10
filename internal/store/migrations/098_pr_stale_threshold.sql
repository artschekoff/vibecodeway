-- +goose Up

ALTER TABLE app_settings ADD COLUMN pr_stale_threshold_days INTEGER NOT NULL DEFAULT 7;

-- +goose Down

ALTER TABLE app_settings DROP COLUMN pr_stale_threshold_days;
