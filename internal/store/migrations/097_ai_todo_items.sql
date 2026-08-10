-- +goose Up

-- ai_todo_items: the board-wide "TODO NEXT (BY AI)" list. One row per next-action.
-- source='ai' rows are replaced on each generation; source='user' rows are user-added.
CREATE TABLE ai_todo_items (
  id         TEXT PRIMARY KEY,
  kind       TEXT NOT NULL,            -- implement | make_pr | ask_question | check_status | rerequest_pr
  title      TEXT NOT NULL,
  rationale  TEXT NOT NULL DEFAULT '',
  ref_type   TEXT NOT NULL DEFAULT '', -- jira | pr | review | (empty)
  ref_key    TEXT NOT NULL DEFAULT '', -- e.g. JH-4922 or juna-health/juna-api#218
  priority   TEXT NOT NULL DEFAULT 'medium', -- critical | high | medium | low
  position   INTEGER NOT NULL DEFAULT 0,
  status     TEXT NOT NULL DEFAULT 'pending', -- pending | done | dismissed
  source     TEXT NOT NULL DEFAULT 'ai',      -- ai | user
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX idx_ai_todo_items_position ON ai_todo_items(position);

-- +goose Down

DROP TABLE ai_todo_items;
