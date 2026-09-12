-- +goose Up

CREATE TABLE ai_usage (
  id                INTEGER PRIMARY KEY,
  provider          TEXT NOT NULL DEFAULT '',
  model             TEXT NOT NULL DEFAULT '',
  prompt_tokens     INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens      INTEGER NOT NULL DEFAULT 0,
  occurred_at       TEXT NOT NULL
);

CREATE INDEX idx_ai_usage_occurred_at ON ai_usage(occurred_at);

-- +goose Down

DROP TABLE ai_usage;
