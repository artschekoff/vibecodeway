-- +goose Up
ALTER TABLE jira_issues        ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE jira_comments      ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE slack_threads      ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE slack_dm_configs   ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE vcs_pull_requests  ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE vcs_pr_comments    ADD COLUMN viewed_at DATETIME DEFAULT NULL;
ALTER TABLE jira_provider      ADD COLUMN base_url TEXT NOT NULL DEFAULT '';
