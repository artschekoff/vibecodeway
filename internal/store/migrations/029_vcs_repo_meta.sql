-- +goose Up

ALTER TABLE vcs_repos ADD COLUMN label       TEXT NOT NULL DEFAULT '';
ALTER TABLE vcs_repos ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE vcs_repos DROP COLUMN label;
ALTER TABLE vcs_repos DROP COLUMN description;
