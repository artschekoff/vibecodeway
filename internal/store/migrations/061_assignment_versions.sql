-- +goose Up
CREATE TABLE IF NOT EXISTS person_assignment_versions (
    id                   TEXT PRIMARY KEY,
    person_assignment_id TEXT NOT NULL REFERENCES context_person_assignments(id) ON DELETE CASCADE,
    body                 TEXT NOT NULL DEFAULT '',
    llm_prompt           TEXT NOT NULL DEFAULT '',
    -- exactly one version per person_assignment_id should have is_active=1 (maintained by application layer)
    is_active            INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS pav_pa_idx ON person_assignment_versions (person_assignment_id);

-- +goose Down
DROP TABLE IF EXISTS person_assignment_versions;
