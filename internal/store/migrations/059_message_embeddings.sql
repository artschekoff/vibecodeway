-- +goose Up
CREATE TABLE IF NOT EXISTS message_embeddings (
    message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    -- float32 array serialised as little-endian bytes (4 bytes per element)
    embedding  BLOB NOT NULL,
    -- model used to produce the embedding (e.g. "text-embedding-3-small")
    model      TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS message_embeddings;
