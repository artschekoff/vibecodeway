-- +goose Up

ALTER TABLE vcs_review_requests ADD COLUMN kind TEXT NOT NULL DEFAULT 'requested';
ALTER TABLE vcs_review_requests ADD COLUMN ci_status TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE vcs_review_requests DROP COLUMN kind;
ALTER TABLE vcs_review_requests DROP COLUMN ci_status;
