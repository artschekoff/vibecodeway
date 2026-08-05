-- +goose Up

-- Add a STRICT filesystem-scope section to the seeded "agent_task" prompt so the
-- Solve-with-AI agent is told to touch files ONLY inside the context's worktrees.
-- Only upgrade rows still equal to the old default; always refresh default_body.
-- Mirrors the update pattern in migration 092.

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

## Filesystem scope (STRICT)
The worktrees listed under "Workspaces on disk" are the ONLY locations you may read from or write to. Do NOT read, edit, create, or delete files anywhere else on the filesystem, and do NOT `cd` into or run shell commands that touch paths outside them (no other repositories, no home-directory files such as ~/.ssh or ~/.config, no system paths). If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.

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

## Filesystem scope (STRICT)
The worktrees listed under "Workspaces on disk" are the ONLY locations you may read from or write to. Do NOT read, edit, create, or delete files anywhere else on the filesystem, and do NOT `cd` into or run shell commands that touch paths outside them (no other repositories, no home-directory files such as ~/.ssh or ~/.config, no system paths). If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.

Work directly in these worktrees to implement the task. Inspect the relevant files, make the changes, and keep them scoped to the task.',
updated_at = '2026-08-04T00:00:00Z'
WHERE id = 'agent_task';

-- +goose Down

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
