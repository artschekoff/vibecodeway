-- +goose Up

-- Raw activity events feeding the analytics dashboard: one row per PR merge
-- or comment authored, deduped by (kind, external_id) so re-running a
-- refresh against the same PR/comment is a no-op instead of a duplicate.
CREATE TABLE stat_events (
  id          INTEGER PRIMARY KEY,
  kind        TEXT NOT NULL,             -- 'pr_merged' | 'comment_authored'
  external_id TEXT NOT NULL,             -- PR URL / comment id — dedup key
  org         TEXT NOT NULL DEFAULT '',
  repo        TEXT NOT NULL DEFAULT '',  -- pr_merged only
  jira_key    TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL,             -- RFC3339 UTC
  UNIQUE(kind, external_id)
);

-- Every read is a since-scan per kind or across kinds.
CREATE INDEX IF NOT EXISTS stat_events_kind_occurred_at
    ON stat_events(kind, occurred_at);

-- +goose Down

DROP INDEX IF EXISTS stat_events_kind_occurred_at;
DROP TABLE IF EXISTS stat_events;
