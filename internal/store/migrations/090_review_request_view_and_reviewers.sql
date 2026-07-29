-- +goose Up
-- viewed_at: NULL = the user has not looked at this review item yet (shown as "new").
-- reviewers: JSON array of {login,state,submittedAt} — each reviewer's latest decision,
--            refreshed every sync. Empty/NULL when no reviews yet.
ALTER TABLE vcs_review_requests ADD COLUMN viewed_at TEXT;
ALTER TABLE vcs_review_requests ADD COLUMN reviewers TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE vcs_review_requests DROP COLUMN viewed_at;
ALTER TABLE vcs_review_requests DROP COLUMN reviewers;
