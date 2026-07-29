-- +goose Up
-- Monaco editor settings, stored as one opaque JSON blob (theme, wordWrap,
-- fontSize, etc). The frontend owns the shape; Go never parses it. New options
-- then need no migration. Empty string = "unset, use frontend defaults".
ALTER TABLE app_settings ADD COLUMN editor_settings TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_settings DROP COLUMN editor_settings;
