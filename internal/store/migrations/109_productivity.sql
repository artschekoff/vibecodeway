-- +goose Up

-- Theoretical productivity model: a workday is `workday_hours` long and every
-- closed task is assumed to cost `task_hours`. Both are user-tunable because the
-- "right" numbers differ per person and per team.
ALTER TABLE app_settings ADD COLUMN workday_hours REAL NOT NULL DEFAULT 8;
ALTER TABLE app_settings ADD COLUMN task_hours REAL NOT NULL DEFAULT 2;

-- One row per Jira key observed moving from an open status to a terminal one.
-- Keyed by jira_key so a reopen → re-close overwrites instead of double-counting.
-- completed_at is detection time (RFC3339 UTC), not Jira's resolution date.
CREATE TABLE IF NOT EXISTS task_completions (
    jira_key     TEXT PRIMARY KEY,
    issue_type   TEXT NOT NULL DEFAULT '',
    org          TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS task_completions_completed_at
    ON task_completions(completed_at);

-- +goose Down

DROP INDEX IF EXISTS task_completions_completed_at;
DROP TABLE IF EXISTS task_completions;
ALTER TABLE app_settings DROP COLUMN task_hours;
ALTER TABLE app_settings DROP COLUMN workday_hours;
