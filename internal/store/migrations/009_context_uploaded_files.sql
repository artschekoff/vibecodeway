-- +goose Up

-- Binary/document files attached to a context whose text has been extracted.
-- file_content stores the original binary so the file can be re-downloaded or
-- re-processed without re-uploading. ON DELETE CASCADE ensures all rows are
-- removed automatically when the parent context is deleted.
CREATE TABLE IF NOT EXISTS context_uploaded_files (
    id             TEXT    PRIMARY KEY,
    context_id     TEXT    NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    original_name  TEXT    NOT NULL DEFAULT '',
    file_type      TEXT    NOT NULL DEFAULT '',
    extracted_text TEXT    NOT NULL DEFAULT '',
    file_content   BLOB    NOT NULL DEFAULT '',
    file_size      INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL,
    is_active      INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_context_uploaded_files_context_id
    ON context_uploaded_files(context_id);

-- +goose Down
DROP INDEX IF EXISTS idx_context_uploaded_files_context_id;
DROP TABLE IF EXISTS context_uploaded_files;
