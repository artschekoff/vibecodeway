-- +goose Up

ALTER TABLE vcs_review_requests ADD COLUMN merge_state_status TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE vcs_review_requests DROP COLUMN merge_state_status;
