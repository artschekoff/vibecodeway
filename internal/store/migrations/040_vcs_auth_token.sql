-- +goose Up
ALTER TABLE vcs_auth ADD COLUMN token TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
