-- +goose Up
CREATE TABLE context_snapshots (
    context_id              TEXT PRIMARY KEY REFERENCES contexts(id) ON DELETE CASCADE,
    -- current step (identified by agent, can be overridden)
    current_step_desc       TEXT NOT NULL DEFAULT '',
    current_step_confidence TEXT NOT NULL DEFAULT 'low',  -- high | medium | low
    current_step_from       TEXT NOT NULL DEFAULT '[]',   -- JSON array of event IDs
    current_step_overridden INTEGER NOT NULL DEFAULT 0,
    -- three snapshot sections stored as markdown
    section_operational     TEXT NOT NULL DEFAULT '',
    section_narrative       TEXT NOT NULL DEFAULT '',
    section_engineering     TEXT NOT NULL DEFAULT '',
    -- effective status (slug from workflow_statuses) for fast list-view rendering
    status                  TEXT NOT NULL DEFAULT 'not_started',
    -- AI-generated next-step recommendations by time horizon (rebuilt on each analysis)
    ai_next_steps_short     TEXT NOT NULL DEFAULT '',  -- ~1 day
    ai_next_steps_mid       TEXT NOT NULL DEFAULT '',  -- ~1 week
    ai_next_steps_long      TEXT NOT NULL DEFAULT '',  -- ~1 month
    -- User-written next-step notes by time horizon (editable any time, no re-analysis needed)
    user_next_steps_short   TEXT NOT NULL DEFAULT '',
    user_next_steps_mid     TEXT NOT NULL DEFAULT '',
    user_next_steps_long    TEXT NOT NULL DEFAULT '',
    -- tracking
    last_analyzed_session_id TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL,
    analyzed_at             TEXT,
    is_stale                INTEGER NOT NULL DEFAULT 0,  -- 1 = user event added since last analysis
    version                 INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE context_snapshots;
