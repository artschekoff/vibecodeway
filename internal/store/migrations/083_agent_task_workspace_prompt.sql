-- +goose Up

-- Upgrade the "agent_task" prompt so the generated coding-agent instruction (a)
-- begins with the /writing-plans and /arts-load-context <context-key> skill
-- commands and (b) points the agent at the per-branch git worktrees. The prompt
-- is user-editable, so only upgrade rows still equal to the old default; always
-- refresh default_body (the canonical reset target). Order matters: update body
-- BEFORE default_body so the "body = default_body" (unmodified) check compares
-- against the OLD default.

UPDATE ai_prompts
SET body = '/writing-plans
/arts-load-context {{context_key}}

You are a coding agent. Implement the task below using the git worktrees checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Workspaces on disk (read and modify the code here)
{{repos}}

Work directly in these worktrees to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.'
WHERE id = 'agent_task' AND body = default_body;

UPDATE ai_prompts
SET default_body = '/writing-plans
/arts-load-context {{context_key}}

You are a coding agent. Implement the task below using the git worktrees checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Workspaces on disk (read and modify the code here)
{{repos}}

Work directly in these worktrees to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.',
updated_at = '2026-07-27T00:00:00Z'
WHERE id = 'agent_task';

-- +goose Down

UPDATE ai_prompts
SET body = 'You are a coding agent. Implement the task below using the repositories checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Repositories on disk (read and modify the code here)
{{repos}}

Work directly in these repositories to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.'
WHERE id = 'agent_task' AND body = default_body;

UPDATE ai_prompts
SET default_body = 'You are a coding agent. Implement the task below using the repositories checked out on disk.

## Task
{{user_prompt}}

## Context
{{context_key}}

## Related tickets
{{tickets}}

## Repositories on disk (read and modify the code here)
{{repos}}

Work directly in these repositories to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.',
updated_at = '2026-01-01T00:00:00Z'
WHERE id = 'agent_task';
