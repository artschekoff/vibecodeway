-- +goose Up

CREATE TABLE IF NOT EXISTS context_open_questions (
    id         TEXT PRIMARY KEY,
    context_id TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    question   TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    answer     TEXT,
    source     TEXT NOT NULL DEFAULT 'ai',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    closed_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_open_questions_context ON context_open_questions(context_id);

-- +goose Down

DROP TABLE IF EXISTS context_open_questions;
