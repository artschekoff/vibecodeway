-- +goose Up

-- Number of unresolved review threads on the PR (GitHub "Resolve conversation"
-- state), recomputed on every reviews sync.
ALTER TABLE vcs_review_requests ADD COLUMN unresolved_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE vcs_review_requests DROP COLUMN unresolved_count;
