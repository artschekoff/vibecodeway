-- +goose Up

CREATE TABLE IF NOT EXISTS context_log_files (
    id         TEXT PRIMARY KEY,
    log_id     TEXT NOT NULL REFERENCES context_logs(id) ON DELETE CASCADE,
    file_id    TEXT NOT NULL REFERENCES context_uploaded_files(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    UNIQUE(log_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_context_log_files_log_id ON context_log_files(log_id);

-- +goose Down

DROP INDEX IF EXISTS idx_context_log_files_log_id;
DROP TABLE IF EXISTS context_log_files;
