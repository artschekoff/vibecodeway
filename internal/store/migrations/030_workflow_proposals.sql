-- +goose Up

CREATE TABLE IF NOT EXISTS context_workflow_proposals (
    id                      TEXT PRIMARY KEY,
    context_id              TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    proposed_status         TEXT NOT NULL,
    proposed_comment        TEXT,
    proposed_next_step      TEXT,
    user_status_override    TEXT,
    user_comment_override   TEXT,
    user_next_step_override TEXT,
    prev_user_next_step     TEXT,
    resolved_at             TEXT,
    updated_at              TEXT NOT NULL DEFAULT '',
    created_at              TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_workflow_proposals_context ON context_workflow_proposals(context_id);

-- +goose Down

DROP TABLE IF EXISTS context_workflow_proposals;
