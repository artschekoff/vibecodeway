-- +goose Up
CREATE TABLE IF NOT EXISTS vcs_workspaces (
    id            TEXT PRIMARY KEY,
    repo_path_id  TEXT NOT NULL REFERENCES vcs_repo_paths(id) ON DELETE CASCADE,
    repo_id       TEXT NOT NULL,
    branch        TEXT NOT NULL,
    worktree_path TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    UNIQUE(worktree_path)
);

-- +goose Down
DROP TABLE IF EXISTS vcs_workspaces;
