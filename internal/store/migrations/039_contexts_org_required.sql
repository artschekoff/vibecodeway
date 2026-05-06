-- +goose Up

-- Assign orphaned contexts (no org) to the first organization.
UPDATE contexts
SET org_id = (SELECT id FROM organizations ORDER BY sort_order ASC, created_at ASC LIMIT 1)
WHERE org_id IS NULL
  AND EXISTS (SELECT 1 FROM organizations);

-- Delete any contexts still without an org (no organizations exist).
DELETE FROM contexts WHERE org_id IS NULL;

-- Recreate contexts with org_id NOT NULL and ON DELETE RESTRICT.
-- SQLite does not support ALTER COLUMN, so we use the rename-copy-drop pattern.
-- DDL statements in SQLite do not trigger FK constraint checks, so no PRAGMA change needed.
CREATE TABLE contexts_new (
    id                       TEXT PRIMARY KEY,
    key                      TEXT NOT NULL UNIQUE,
    status                   TEXT NOT NULL DEFAULT 'active',
    org_id                   TEXT NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL,
    sort_order               INTEGER NOT NULL DEFAULT 0,
    ai_proposed_status       TEXT NOT NULL DEFAULT 'not_started',
    ai_proposed_comment      TEXT,
    ai_proposed_next_step    TEXT,
    ai_proposed_at           TEXT,
    user_status_override     TEXT,
    user_comment_override    TEXT,
    user_next_step_override  TEXT,
    user_override_updated_at TEXT
);

INSERT INTO contexts_new
SELECT id, key, status, org_id, created_at, updated_at, sort_order,
       ai_proposed_status, ai_proposed_comment, ai_proposed_next_step, ai_proposed_at,
       user_status_override, user_comment_override, user_next_step_override, user_override_updated_at
FROM contexts;

DROP TABLE contexts;
ALTER TABLE contexts_new RENAME TO contexts;

-- +goose Down

CREATE TABLE contexts_old (
    id                       TEXT PRIMARY KEY,
    key                      TEXT NOT NULL UNIQUE,
    status                   TEXT NOT NULL DEFAULT 'active',
    org_id                   TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    created_at               TEXT NOT NULL,
    updated_at               TEXT NOT NULL,
    sort_order               INTEGER NOT NULL DEFAULT 0,
    ai_proposed_status       TEXT NOT NULL DEFAULT 'not_started',
    ai_proposed_comment      TEXT,
    ai_proposed_next_step    TEXT,
    ai_proposed_at           TEXT,
    user_status_override     TEXT,
    user_comment_override    TEXT,
    user_next_step_override  TEXT,
    user_override_updated_at TEXT
);

INSERT INTO contexts_old
SELECT id, key, status, org_id, created_at, updated_at, sort_order,
       ai_proposed_status, ai_proposed_comment, ai_proposed_next_step, ai_proposed_at,
       user_status_override, user_comment_override, user_next_step_override, user_override_updated_at
FROM contexts;

DROP TABLE contexts;
ALTER TABLE contexts_old RENAME TO contexts;
