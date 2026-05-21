-- +goose Up

-- Add text and file as source types
INSERT INTO sources (id, label) VALUES
    ('text', 'Text Note'),
    ('file', 'Uploaded File');

-- Link context texts and uploaded files to their message records
ALTER TABLE context_texts            ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE context_uploaded_files   ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;

-- +goose Down
-- SQLite: additive-only; no rollback for ALTER TABLE or INSERT into dictionary.
