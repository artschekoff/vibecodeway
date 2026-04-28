-- +goose Up
-- reviewed_at: set when a review request disappears from GitHub's list (user submitted a review).
--              NULL means it's still pending.
-- dismissed_at: set when user manually dismisses a "waiting for author" item.
ALTER TABLE vcs_review_requests ADD COLUMN reviewed_at  TEXT;
ALTER TABLE vcs_review_requests ADD COLUMN dismissed_at TEXT;

-- +goose Down
