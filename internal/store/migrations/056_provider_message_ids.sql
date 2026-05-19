-- +goose Up
ALTER TABLE jira_issues       ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE jira_comments     ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE slack_threads     ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE vcs_pull_requests ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE vcs_pr_comments   ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;

-- +goose Down
-- SQLite: additive-only; no rollback needed for nullable FK columns.
