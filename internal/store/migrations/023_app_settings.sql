-- +goose Up
-- app_settings is a singleton row (id = 1) for application-level preferences.
-- context_refetch_enabled / reviews_refetch_enabled: 1 = background refetch active.
-- *_interval_min: how often (in minutes) to run the background refresh.
CREATE TABLE IF NOT EXISTS app_settings (
    id                           INTEGER PRIMARY KEY,
    context_refetch_enabled      INTEGER NOT NULL DEFAULT 0,
    context_refetch_interval_min INTEGER NOT NULL DEFAULT 30,
    reviews_refetch_enabled      INTEGER NOT NULL DEFAULT 0,
    reviews_refetch_interval_min INTEGER NOT NULL DEFAULT 30,
    updated_at                   TEXT    NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS app_settings;
