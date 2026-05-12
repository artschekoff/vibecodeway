-- +goose Up
CREATE TABLE event_log (
    id              TEXT PRIMARY KEY,
    context_id      TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    sync_session_id TEXT REFERENCES sync_sessions(id) ON DELETE SET NULL,
    type            TEXT NOT NULL,  -- requirement_stated, work_completed, decision_made, etc.
    timestamp       TEXT NOT NULL,  -- original event timestamp (from source)
    source_type     TEXT NOT NULL,  -- jira | slack | github | chat | user
    source_id       TEXT NOT NULL DEFAULT '',  -- original item id
    author          TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',  -- AI-generated 1-2 sentence summary
    raw_reference   TEXT NOT NULL DEFAULT '',  -- original text or URL
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX event_log_context_idx      ON event_log(context_id, timestamp DESC);
CREATE INDEX event_log_session_idx      ON event_log(sync_session_id);

-- +goose Down
DROP INDEX IF EXISTS event_log_session_idx;
DROP INDEX IF EXISTS event_log_context_idx;
DROP TABLE event_log;
