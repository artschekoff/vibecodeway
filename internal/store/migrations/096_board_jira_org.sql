-- +goose Up

-- Store each assigned Jira issue's scope/service (this instance's "organization",
-- read from the OrgScopeFieldID custom field, e.g. juna-api / juna-frontend / rdbms)
-- so My Board can be split by it. Empty string when the issue has no scope set.
ALTER TABLE board_jira_assignments ADD COLUMN org TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE board_jira_assignments DROP COLUMN org;
