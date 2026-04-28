-- +goose Up
-- ci_status stores the aggregated GitHub status-check rollup for a PR:
--   ''        = no checks configured
--   'PENDING' = at least one check still running
--   'SUCCESS' = all checks passed
--   'FAILURE' = at least one check failed or errored
ALTER TABLE vcs_pull_requests ADD COLUMN ci_status TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN; migration is irreversible.
