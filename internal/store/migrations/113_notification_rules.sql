-- +goose Up

-- A user-defined watch on a context: when an event of event_type arrives with a
-- new value matching match_value, the notification it produces carries `note`.
-- The Ask-AI companion is the only thing that writes rows here today.
CREATE TABLE IF NOT EXISTS notification_rules (
    id          TEXT PRIMARY KEY,
    context_id  TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,                 -- 'jira_status_changed'
    match_value TEXT NOT NULL DEFAULT '',      -- the NEW value that fires it; '' = any
    note        TEXT NOT NULL,                 -- the user's own text, shown verbatim
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS notification_rules_context
    ON notification_rules(context_id);

-- The note is copied onto the notification when the rule fires, so editing or
-- deleting the rule later cannot rewrite what the user was already told.
ALTER TABLE notifications ADD COLUMN note TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE notifications DROP COLUMN note;
DROP INDEX IF EXISTS notification_rules_context;
DROP TABLE IF EXISTS notification_rules;
