-- +goose Up

-- Fix the leading slash command in the seeded "agent_task" prompt: /writing-plans
-- is not a resolvable command (the skill is a plugin skill, invoked as
-- /superpowers:writing-plans). Only upgrade rows still equal to the old default;
-- always refresh default_body. Mirrors the rename pattern in migration 088.

UPDATE ai_prompts
SET body = '/superpowers:writing-plans
/context-consolidator-load-context {{context_key}}

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
SET default_body = '/superpowers:writing-plans
/context-consolidator-load-context {{context_key}}

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
updated_at = '2026-08-03T00:00:00Z'
WHERE id = 'agent_task';

-- +goose Down

UPDATE ai_prompts
SET body = '/writing-plans
/context-consolidator-load-context {{context_key}}

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
/context-consolidator-load-context {{context_key}}

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
updated_at = '2026-07-28T00:00:00Z'
WHERE id = 'agent_task';
