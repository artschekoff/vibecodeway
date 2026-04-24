-- +goose Up

ALTER TABLE context_files RENAME TO context_texts;

DROP INDEX IF EXISTS idx_context_files_context_id;
CREATE INDEX IF NOT EXISTS idx_context_texts_context_id ON context_texts(context_id);

-- +goose Down

ALTER TABLE context_texts RENAME TO context_files;

DROP INDEX IF EXISTS idx_context_texts_context_id;
CREATE INDEX IF NOT EXISTS idx_context_files_context_id ON context_files(context_id);
