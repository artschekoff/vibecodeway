-- +goose Up

-- Optional identity used to attribute Jira comments to the current user for
-- comment_authored stat-event capture. The Jira client shells out via acli
-- (no REST token, no /myself lookup), so there is no other way to answer
-- "is this comment mine?" than asking the user to type their own Jira
-- display name once. Empty ⇒ capture is skipped entirely, silently.
ALTER TABLE app_settings ADD COLUMN stats_author_name TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE app_settings DROP COLUMN stats_author_name;
