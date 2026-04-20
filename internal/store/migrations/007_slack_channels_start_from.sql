-- +goose Up

-- Add optional start_from date (YYYY-MM-DD) to Slack channel configs.
-- When set, it is passed as --start-from to slack-cli load-context on refresh.
ALTER TABLE slack_channels ADD COLUMN start_from TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; leave as-is.
