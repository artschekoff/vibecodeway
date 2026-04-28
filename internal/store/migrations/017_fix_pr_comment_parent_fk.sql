-- +goose Up

-- Recreate vcs_pr_comments with parent_id ON DELETE SET NULL so that deleting a
-- parent comment (or cascading from vcs_pull_requests) does not cause a FK
-- violation on child reply rows — replies survive with parent_id nulled out.
PRAGMA foreign_keys = OFF;

ALTER TABLE vcs_pr_comments RENAME TO vcs_pr_comments_old;

CREATE TABLE vcs_pr_comments (
    id                 TEXT PRIMARY KEY,
    pr_id              TEXT NOT NULL REFERENCES vcs_pull_requests(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    parent_id          TEXT REFERENCES vcs_pr_comments(id) ON DELETE SET NULL,
    comment_type       TEXT NOT NULL DEFAULT 'conversation',
    author             TEXT NOT NULL DEFAULT '',
    body               TEXT NOT NULL DEFAULT '',
    path               TEXT,
    line               INTEGER,
    is_resolved        INTEGER NOT NULL DEFAULT 0,
    in_reply_to_id     TEXT,
    comment_created_at TEXT NOT NULL DEFAULT '',
    comment_updated_at TEXT NOT NULL DEFAULT '',
    viewed_at          DATETIME DEFAULT NULL,
    ai_processed_at    DATETIME DEFAULT NULL,
    UNIQUE (pr_id, external_id)
);

INSERT INTO vcs_pr_comments
SELECT id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
       is_resolved, in_reply_to_id, comment_created_at, comment_updated_at,
       viewed_at, ai_processed_at
FROM vcs_pr_comments_old;

DROP TABLE vcs_pr_comments_old;

CREATE INDEX IF NOT EXISTS idx_vcs_pr_comments_pr_id ON vcs_pr_comments(pr_id);

PRAGMA foreign_keys = ON;

-- +goose Down

PRAGMA foreign_keys = OFF;

ALTER TABLE vcs_pr_comments RENAME TO vcs_pr_comments_new;

CREATE TABLE vcs_pr_comments (
    id                 TEXT PRIMARY KEY,
    pr_id              TEXT NOT NULL REFERENCES vcs_pull_requests(id) ON DELETE CASCADE,
    external_id        TEXT NOT NULL,
    parent_id          TEXT REFERENCES vcs_pr_comments(id),
    comment_type       TEXT NOT NULL DEFAULT 'conversation',
    author             TEXT NOT NULL DEFAULT '',
    body               TEXT NOT NULL DEFAULT '',
    path               TEXT,
    line               INTEGER,
    is_resolved        INTEGER NOT NULL DEFAULT 0,
    in_reply_to_id     TEXT,
    comment_created_at TEXT NOT NULL DEFAULT '',
    comment_updated_at TEXT NOT NULL DEFAULT '',
    viewed_at          DATETIME DEFAULT NULL,
    ai_processed_at    DATETIME DEFAULT NULL,
    UNIQUE (pr_id, external_id)
);

INSERT INTO vcs_pr_comments
SELECT id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
       is_resolved, in_reply_to_id, comment_created_at, comment_updated_at,
       viewed_at, ai_processed_at
FROM vcs_pr_comments_new;

DROP TABLE vcs_pr_comments_new;

CREATE INDEX IF NOT EXISTS idx_vcs_pr_comments_pr_id ON vcs_pr_comments(pr_id);

PRAGMA foreign_keys = ON;
