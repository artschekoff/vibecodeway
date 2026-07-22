-- +goose Up
CREATE TABLE IF NOT EXISTS context_plans (
    id                   TEXT PRIMARY KEY,
    context_id           TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    person_assignment_id TEXT NOT NULL,
    body                 TEXT NOT NULL,
    created_at           TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS context_plans_by_assignment
    ON context_plans(person_assignment_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS context_plans;
