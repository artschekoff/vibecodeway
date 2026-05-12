-- +goose Up
ALTER TABLE ai_settings ADD COLUMN openai_key TEXT;
ALTER TABLE ai_settings ADD COLUMN anthropic_key TEXT;

-- +goose Down
SELECT 1; -- SQLite does not support DROP COLUMN on older versions; migration is effectively irreversible
