-- +goose Up
ALTER TABLE jira_issues        ADD COLUMN sync_session_id TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL;
ALTER TABLE slack_threads       ADD COLUMN sync_session_id TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL;
ALTER TABLE vcs_pull_requests  ADD COLUMN sync_session_id TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL;

-- +goose Down
SELECT 1; -- SQLite cannot drop columns; this migration is effectively irreversible
