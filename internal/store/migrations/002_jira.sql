-- +goose Up

-- Jira provider config — one row per context (optional).
CREATE TABLE IF NOT EXISTS jira_provider (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL UNIQUE REFERENCES contexts(id) ON DELETE CASCADE,
    search_query TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- Jira issues fetched via the saved JQL query.
CREATE TABLE IF NOT EXISTS jira_issues (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    jira_key     TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT '',
    assignee     TEXT NOT NULL DEFAULT '',
    issue_type   TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    jira_updated TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    UNIQUE(context_id, jira_key)
);

-- Jira comments fetched for each issue.
CREATE TABLE IF NOT EXISTS jira_comments (
    id           TEXT PRIMARY KEY,
    issue_id     TEXT NOT NULL REFERENCES jira_issues(id) ON DELETE CASCADE,
    jira_id      TEXT NOT NULL,
    author       TEXT NOT NULL DEFAULT '',
    body         TEXT NOT NULL DEFAULT '',
    jira_created TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    UNIQUE(issue_id, jira_id)
);

-- +goose Down
DROP TABLE IF EXISTS jira_comments;
DROP TABLE IF EXISTS jira_issues;
DROP TABLE IF EXISTS jira_provider;
