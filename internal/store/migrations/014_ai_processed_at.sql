-- +goose Up
ALTER TABLE jira_issues        ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;
ALTER TABLE jira_comments      ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;
ALTER TABLE slack_threads      ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;
ALTER TABLE slack_dm_configs   ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;
ALTER TABLE vcs_pull_requests  ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;
ALTER TABLE vcs_pr_comments    ADD COLUMN ai_processed_at DATETIME DEFAULT NULL;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; recreate tables if needed.
