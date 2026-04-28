-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    context_id TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,   -- e.g. 'pull_request'
    entity_id   TEXT NOT NULL,   -- vcs_pull_requests.id
    event_type  TEXT NOT NULL,   -- 'pr_conflict' | 'pr_review_changed'
    old_value   TEXT NOT NULL DEFAULT '',
    new_value   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    read_at     TEXT             -- NULL = unread
);

CREATE INDEX IF NOT EXISTS notifications_context_unread
    ON notifications(context_id)
    WHERE read_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS notifications_context_unread;
DROP TABLE IF EXISTS notifications;
