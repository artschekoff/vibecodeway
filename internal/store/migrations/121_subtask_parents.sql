-- +goose Up
-- Cache of subtask → parent issue key. A subtask's parent never changes, so
-- rows are permanent; resolving one costs a ~5s `acli workitem view` call.
CREATE TABLE jira_subtask_parents (
    subtask_key TEXT PRIMARY KEY,
    parent_key  TEXT NOT NULL
);

-- +goose Down
DROP TABLE jira_subtask_parents;
