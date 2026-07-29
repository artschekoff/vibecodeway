-- +goose Up

-- Rename slash commands in seeded agent prompts from /arts-load-context to
-- /context-consolidator-load-context (CLI-only command surface). Only upgrade
-- rows still equal to the old default; always refresh default_body.

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

UPDATE ai_prompts
SET body = '/context-consolidator-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}'
WHERE id = 'pr_review' AND body = default_body;

UPDATE ai_prompts
SET default_body = '/context-consolidator-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}',
updated_at = '2026-07-28T00:00:00Z'
WHERE id = 'pr_review';

-- +goose Down

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

UPDATE ai_prompts
SET body = '/arts-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}'
WHERE id = 'pr_review' AND body = default_body;

UPDATE ai_prompts
SET default_body = '/arts-load-context {{context_key}}

You are a senior code reviewer. Review pull request #{{pr_number}} — {{pr_title}}.

The PR branch is checked out as a git worktree below. Diff it against its base branch (e.g. `git diff main...HEAD` inside the worktree), then review the changed files for correctness bugs, security issues, edge cases, and maintainability.

Do NOT modify the code. Produce a review: list findings as file:line — severity — issue — suggested fix, then a short overall assessment.

## Worktrees on disk (the PR branch is checked out here)
{{repos}}',
updated_at = '2026-07-27T00:00:00Z'
WHERE id = 'pr_review';
