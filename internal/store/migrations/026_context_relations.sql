-- +goose Up

CREATE TABLE IF NOT EXISTS context_relations (
    id            TEXT PRIMARY KEY,
    context_id    TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    target_id     TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE(context_id, target_id, relation_type)
);

-- +goose Down
DROP TABLE IF EXISTS context_relations;
