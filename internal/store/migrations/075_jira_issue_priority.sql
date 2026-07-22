-- +goose Up
ALTER TABLE jira_issues ADD COLUMN priority TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE jira_issues DROP COLUMN priority;
