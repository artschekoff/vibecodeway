-- +goose Up
ALTER TABLE context_vcs_branches ADD COLUMN label TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE context_vcs_branches DROP COLUMN label;
