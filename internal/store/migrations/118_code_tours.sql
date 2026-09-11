-- +goose Up

CREATE TABLE code_tours (
  id             TEXT PRIMARY KEY,          -- uuid
  context_id     TEXT NOT NULL,
  repo_path      TEXT NOT NULL,
  source         TEXT NOT NULL,             -- 'agent' | 'generated'
  title          TEXT NOT NULL DEFAULT '',
  steps_json     TEXT NOT NULL,             -- []TourStep
  step_index     INTEGER NOT NULL DEFAULT 0,
  decisions_json TEXT NOT NULL DEFAULT '{}',-- {stepIdx: 'staged'|'discarded'|'edited'|'skipped'}
  created_at     TEXT NOT NULL
);

CREATE INDEX idx_code_tours_context_repo ON code_tours(context_id, repo_path);

-- +goose Down

DROP TABLE code_tours;
