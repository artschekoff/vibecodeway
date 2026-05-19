-- +goose Up

-- Generic reasoning chain: one table for all entity types (person_assignment, context, …)
CREATE TABLE IF NOT EXISTS reasoning_chain_steps (
    id               TEXT PRIMARY KEY,
    entity_type      TEXT NOT NULL,  -- 'person_assignment' | 'context'
    entity_id        TEXT NOT NULL,  -- person_assignment.id or context.id
    step_index       INTEGER NOT NULL,
    -- 'initial' | 'message' | 'instruction'
    source_type      TEXT NOT NULL DEFAULT 'message',
    -- which context message triggered this step (optional)
    message_id       TEXT,
    -- snapshot of the content fed to the AI (self-contained for chain replay)
    message_content  TEXT NOT NULL DEFAULT '',
    -- for source_type='instruction'
    user_instruction TEXT NOT NULL DEFAULT '',
    -- state before and after this step (markdown body, JSON events, etc.)
    input_state      TEXT NOT NULL DEFAULT '',
    output_state     TEXT NOT NULL DEFAULT '',
    -- rolling compressed summary of all output up to this point (replaces full input_state in long chains)
    compressed_summary TEXT NOT NULL DEFAULT '',
    -- JSON snapshot of events at step creation time (fixes replay isolation — no live re-fetch)
    events_snapshot  TEXT NOT NULL DEFAULT '',
    -- which prompt template version produced this step (immutable for replay audit)
    prompt_version   TEXT NOT NULL DEFAULT 'v1',
    -- LLM-as-judge quality score 0.0–1.0 (NULL = not yet scored)
    quality_score    REAL,
    -- judge rationale (completeness / specificity / delta-accuracy notes)
    quality_notes    TEXT NOT NULL DEFAULT '',
    -- exactly one step per (entity_type, entity_id) should have is_current=1
    is_current       INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entity_type, entity_id, step_index)
);

CREATE INDEX IF NOT EXISTS rcs_entity_idx ON reasoning_chain_steps (entity_type, entity_id);

-- Watermark and outdated flag on person assignments
ALTER TABLE context_person_assignments ADD COLUMN analyzed_through TEXT;
ALTER TABLE context_person_assignments ADD COLUMN is_outdated      INTEGER NOT NULL DEFAULT 0;

-- +goose Down
DROP TABLE IF EXISTS reasoning_chain_steps;
-- SQLite cannot DROP COLUMN; down migration intentionally leaves the new columns
