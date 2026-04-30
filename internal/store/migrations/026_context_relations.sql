-- +goose Up

CREATE TABLE IF NOT EXISTS context_relations (
    id            TEXT PRIMARY KEY,
    context_id    TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    target_id     TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE(context_id, target_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_context_relations_context_id ON context_relations(context_id);
CREATE INDEX IF NOT EXISTS idx_context_relations_target_id  ON context_relations(target_id);

-- +goose Down
DROP TABLE IF EXISTS context_relations;
