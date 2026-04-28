-- +goose Up
ALTER TABLE contexts ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- Initialise sort_order from current display order (newest first → lowest sort_order).
-- This ensures the first run matches what users already see.
UPDATE contexts SET sort_order = (
    SELECT COUNT(*) FROM contexts c2 WHERE c2.updated_at > contexts.updated_at
);

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is irreversible.
