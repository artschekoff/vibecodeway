-- +goose Up

-- Provider / origin dictionary. id is the slug used as FK everywhere.
CREATE TABLE sources (
    id         TEXT PRIMARY KEY,   -- "jira" | "slack" | "github" | "user"
    label      TEXT NOT NULL,      -- human-readable, e.g. "Jira"
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO sources (id, label) VALUES
    ('jira',   'Jira'),
    ('slack',  'Slack'),
    ('github', 'GitHub'),
    ('user',   'User Note');

-- Org-level people. No source-system accounts here — one row per real human.
CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    org_id       TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name TEXT NOT NULL DEFAULT '',
    email        TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Maps a (source, verbatim author name/id from that source) → org user.
-- Populated by the user via the author-resolution modal after each refresh.
CREATE TABLE author_mappings (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id    TEXT NOT NULL REFERENCES sources(id),
    external_name TEXT NOT NULL,  -- verbatim string as it appears in source data
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (source_id, external_name)
);

CREATE INDEX author_mappings_user_idx ON author_mappings(user_id);

-- Base message abstraction for all content items across all providers.
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    author_id       TEXT REFERENCES users(id) ON DELETE SET NULL,  -- NULL = unresolved
    raw_author      TEXT NOT NULL DEFAULT '',  -- verbatim author string from source (for modal + fallback display)
    context_id      TEXT REFERENCES contexts(id) ON DELETE CASCADE,
    organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    parent_id       TEXT REFERENCES messages(id) ON DELETE SET NULL,
    title           TEXT NOT NULL DEFAULT '',
    body            TEXT NOT NULL DEFAULT '',
    source_id       TEXT NOT NULL REFERENCES sources(id),
    external_id     TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    inserted_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX messages_context_idx    ON messages(context_id, created_at DESC);
CREATE INDEX messages_parent_idx     ON messages(parent_id);
CREATE INDEX messages_source_idx     ON messages(source_id, external_id);
CREATE INDEX messages_unresolved_idx ON messages(context_id, author_id) WHERE author_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS messages_unresolved_idx;
DROP INDEX IF EXISTS messages_source_idx;
DROP INDEX IF EXISTS messages_parent_idx;
DROP INDEX IF EXISTS messages_context_idx;
DROP TABLE IF EXISTS messages;
DROP INDEX IF EXISTS author_mappings_user_idx;
DROP TABLE IF EXISTS author_mappings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS sources;
