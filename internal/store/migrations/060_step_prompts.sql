-- +goose Up
CREATE TABLE IF NOT EXISTS step_prompts (
    id         TEXT PRIMARY KEY,
    step_id    TEXT NOT NULL UNIQUE REFERENCES reasoning_chain_steps(id) ON DELETE CASCADE,
    llm_prompt TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
-- +goose Down
DROP TABLE IF EXISTS step_prompts;
