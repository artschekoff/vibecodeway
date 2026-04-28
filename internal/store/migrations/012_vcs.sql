-- +goose Up

-- Per-platform auth: one row per VCS platform (github/gitlab/linear).
CREATE TABLE IF NOT EXISTS vcs_auth (
    id          TEXT PRIMARY KEY,
    platform    TEXT NOT NULL UNIQUE,
    username    TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL
);

-- Tracked VCS repositories (global, not per-context; reusable for Pending Reviews).
CREATE TABLE IF NOT EXISTS vcs_repos (
    id         TEXT PRIMARY KEY,
    platform   TEXT NOT NULL,
    owner      TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (platform, owner, name)
);

-- Links branches to contexts (N:N through repo).
CREATE TABLE IF NOT EXISTS context_vcs_branches (
    id         TEXT PRIMARY KEY,
    context_id TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    repo_id    TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
    branch     TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (context_id, repo_id, branch)
);

-- PR/MR rows. A PR belongs to one repo and is linked to a branch entry.
-- external_id is TEXT to accommodate non-numeric IDs in future platforms.
CREATE TABLE IF NOT EXISTS vcs_pull_requests (
    id            TEXT PRIMARY KEY,
    repo_id       TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
    branch_id     TEXT NOT NULL REFERENCES context_vcs_branches(id) ON DELETE CASCADE,
    pr_number     INTEGER NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    author        TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT 'open',
    review_status TEXT NOT NULL DEFAULT '',
    url           TEXT NOT NULL DEFAULT '',
    pr_created_at TEXT NOT NULL DEFAULT '',
    pr_updated_at TEXT NOT NULL DEFAULT '',
    fetched_at    TEXT NOT NULL,
    UNIQUE (repo_id, pr_number)
);

-- All comments: conversation comments + review comments + review bodies.
-- parent_id is a self-referencing FK for reply threading.
CREATE TABLE IF NOT EXISTS vcs_pr_comments (
    id             TEXT PRIMARY KEY,
    pr_id          TEXT NOT NULL REFERENCES vcs_pull_requests(id) ON DELETE CASCADE,
    external_id    TEXT NOT NULL,
    parent_id      TEXT REFERENCES vcs_pr_comments(id),
    comment_type   TEXT NOT NULL DEFAULT 'conversation',
    author         TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL DEFAULT '',
    path           TEXT,
    line           INTEGER,
    is_resolved    INTEGER NOT NULL DEFAULT 0,
    in_reply_to_id TEXT,
    comment_created_at TEXT NOT NULL DEFAULT '',
    comment_updated_at TEXT NOT NULL DEFAULT '',
    UNIQUE (pr_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_vcs_pr_comments_pr_id ON vcs_pr_comments(pr_id);

-- Review requests: PRs where this user's review has been requested.
-- Used for the "Pending Code Reviews" view.
CREATE TABLE IF NOT EXISTS vcs_review_requests (
    id           TEXT PRIMARY KEY,
    repo_id      TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
    pr_number    INTEGER NOT NULL,
    pr_title     TEXT NOT NULL DEFAULT '',
    pr_author    TEXT NOT NULL DEFAULT '',
    pr_url       TEXT NOT NULL DEFAULT '',
    review_status TEXT NOT NULL DEFAULT '',
    requested_at TEXT NOT NULL DEFAULT '',
    fetched_at   TEXT NOT NULL,
    UNIQUE (repo_id, pr_number)
);

CREATE INDEX IF NOT EXISTS idx_context_vcs_branches_context_id ON context_vcs_branches(context_id);
CREATE INDEX IF NOT EXISTS idx_vcs_pull_requests_branch_id ON vcs_pull_requests(branch_id);
CREATE INDEX IF NOT EXISTS idx_vcs_pull_requests_repo_id ON vcs_pull_requests(repo_id);

-- +goose Down
DROP INDEX IF EXISTS idx_vcs_pull_requests_repo_id;
DROP INDEX IF EXISTS idx_vcs_pull_requests_branch_id;
DROP INDEX IF EXISTS idx_context_vcs_branches_context_id;
DROP INDEX IF EXISTS idx_vcs_pr_comments_pr_id;
DROP TABLE IF EXISTS vcs_review_requests;
DROP TABLE IF EXISTS vcs_pr_comments;
DROP TABLE IF EXISTS vcs_pull_requests;
DROP TABLE IF EXISTS context_vcs_branches;
DROP TABLE IF EXISTS vcs_repos;
DROP TABLE IF EXISTS vcs_auth;
