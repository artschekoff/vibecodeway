-- +goose Up

-- Seed the "agent_task" prompt used by BuildAgentTask to assemble a coding-agent
-- instruction string from a context's linked tickets and on-disk repos. This is
-- pure string assembly (no LLM call) — the prompt is handed to an external
-- coding agent, not sent to a chat model. INSERT OR IGNORE keeps this safe if a
-- row already exists.

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'agent_task',
'Agent Task',
'You are a coding agent. Implement the task below using the repositories checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Repositories on disk (read and modify the code here)
{{repos}}

Work directly in these repositories to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.',
'You are a coding agent. Implement the task below using the repositories checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Repositories on disk (read and modify the code here)
{{repos}}

Work directly in these repositories to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.',
'2026-01-01T00:00:00Z'
);

-- +goose Down

DELETE FROM ai_prompts WHERE id='agent_task';
