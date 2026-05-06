-- +goose Up
CREATE TABLE workflow_statuses (
    id         TEXT PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    label      TEXT NOT NULL,
    is_system  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Seed the five built-in statuses as system (protected from deletion).
INSERT INTO workflow_statuses (id, slug, label, is_system) VALUES
    (lower(hex(randomblob(16))), 'not_started',     'Not started',     1),
    (lower(hex(randomblob(16))), 'in_progress',     'In progress',     1),
    (lower(hex(randomblob(16))), 'scoping',         'Scoping',         1),
    (lower(hex(randomblob(16))), 'needs_attention', 'Needs attention', 1),
    (lower(hex(randomblob(16))), 'ready',           'Ready',           1);

-- +goose Down
DROP TABLE workflow_statuses;
