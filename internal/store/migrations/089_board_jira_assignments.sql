-- +goose Up
-- Persisted "my assigned Jira issues" to-do list. Refreshed from acli via
-- `assignee = currentUser()`. `seen` is 0 (new/unread) until the user views the
-- Activity Feed, then 1. Refresh replaces the set but preserves `seen`.
CREATE TABLE IF NOT EXISTS board_jira_assignments (
    jira_key   TEXT PRIMARY KEY,
    summary    TEXT NOT NULL,
    status     TEXT NOT NULL,
    issue_type TEXT NOT NULL,
    priority   TEXT NOT NULL,
    created    TEXT NOT NULL,
    seen       INTEGER NOT NULL DEFAULT 0,
    fetched_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE board_jira_assignments;
