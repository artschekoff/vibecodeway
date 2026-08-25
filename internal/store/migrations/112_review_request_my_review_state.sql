-- +goose Up

ALTER TABLE vcs_review_requests ADD COLUMN my_review_state TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE vcs_review_requests DROP COLUMN my_review_state;
