-- +goose Up
-- mergeable: GitHub mergeability status ("MERGEABLE", "CONFLICTING", "UNKNOWN"),
-- refreshed every sync. Empty when never fetched. Lets the Code Reviews cards
-- surface a merge-conflict badge for PRs the user is reviewing (authored by others),
-- which the separate vcs_pull_requests table only tracks for the user's own PRs.
ALTER TABLE vcs_review_requests ADD COLUMN mergeable TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE vcs_review_requests DROP COLUMN mergeable;
