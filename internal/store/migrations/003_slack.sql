-- +goose Up

-- Slack provider config — one row per context (optional).
CREATE TABLE IF NOT EXISTS slack_provider (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL UNIQUE REFERENCES contexts(id) ON DELETE CASCADE,
    workspace    TEXT NOT NULL DEFAULT '',
    search_query TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- Slack channels saved per context. Multiple channels per context are supported.
-- Must be defined before slack_threads so the FK reference resolves.
CREATE TABLE IF NOT EXISTS slack_channels (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    workspace    TEXT NOT NULL,
    channel_id   TEXT NOT NULL DEFAULT '',
    channel_name TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    UNIQUE(context_id, workspace, channel_name)
);

-- Slack threads loaded via slack-cli load-context (stored as markdown).
-- slack_channel_id links each thread to its parent slack_channels row so that
-- deleting a channel automatically removes all of its threads (ON DELETE CASCADE).
CREATE TABLE IF NOT EXISTS slack_threads (
    id               TEXT PRIMARY KEY,
    context_id       TEXT NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    slack_channel_id TEXT REFERENCES slack_channels(id) ON DELETE CASCADE,
    workspace        TEXT NOT NULL DEFAULT '',
    channel_id       TEXT NOT NULL,
    thread_ts        TEXT NOT NULL,
    channel_name     TEXT NOT NULL DEFAULT '',
    permalink        TEXT NOT NULL DEFAULT '',
    search_query     TEXT NOT NULL DEFAULT '',
    content          TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    UNIQUE(context_id, channel_id, thread_ts)
);

-- +goose Down
DROP TABLE IF EXISTS slack_threads;
DROP TABLE IF EXISTS slack_channels;
DROP TABLE IF EXISTS slack_provider;
