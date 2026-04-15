-- +goose Up

-- Core contexts table. Intentionally provider-agnostic: no provider-specific
-- columns live here. Provider config goes in their own tables (see 002_jira, 003_slack).
CREATE TABLE IF NOT EXISTS contexts (
    id         TEXT PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS contexts;
