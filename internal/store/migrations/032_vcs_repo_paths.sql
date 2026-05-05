-- +goose Up
CREATE TABLE IF NOT EXISTS vcs_repo_paths (
    id         TEXT PRIMARY KEY,
    repo_id    TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (repo_id, path)
);

-- Migrate existing non-empty local_path values into the new table
INSERT OR IGNORE INTO vcs_repo_paths (id, repo_id, path, created_at)
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))),
       id, local_path, created_at
FROM vcs_repos
WHERE local_path != '';

-- +goose Down
DROP TABLE IF EXISTS vcs_repo_paths;
