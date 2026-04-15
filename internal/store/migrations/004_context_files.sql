-- +goose Up

-- Plain text documents associated with a context (1 context : many files).
-- Content is stored as TEXT which SQLite handles as unlimited size.
CREATE TABLE IF NOT EXISTS context_files (
    id         TEXT PRIMARY KEY,
    context_id TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    name       TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Index to efficiently list files by context.
CREATE INDEX IF NOT EXISTS idx_context_files_context_id ON context_files(context_id);

-- +goose Down
DROP INDEX IF EXISTS idx_context_files_context_id;
DROP TABLE IF EXISTS context_files;
