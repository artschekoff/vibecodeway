-- +goose Up
ALTER TABLE vcs_repos ADD COLUMN local_path TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE vcs_repos DROP COLUMN local_path;
