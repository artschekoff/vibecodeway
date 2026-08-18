-- +goose Up
-- Per-link switch for PR resolution. Default 1 keeps every existing link syncing
-- PRs as before; 0 means "track this branch only" (e.g. main) so the sync never
-- fetches the branch's pull requests.
ALTER TABLE context_vcs_branches ADD COLUMN autodetect_pr INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE context_vcs_branches DROP COLUMN autodetect_pr;
