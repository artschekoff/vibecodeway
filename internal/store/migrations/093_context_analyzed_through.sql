-- +goose Up
ALTER TABLE context_snapshots ADD COLUMN analyzed_through TEXT;

-- +goose Down
ALTER TABLE context_snapshots DROP COLUMN analyzed_through;
