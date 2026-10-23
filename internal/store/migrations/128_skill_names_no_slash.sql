-- +goose Up

-- Codex does not resolve "/skill" slash commands, so prompts handed to external
-- agents must reference skills by bare name. Patch every live prompt row
-- (mirrors the REPLACE pattern in migration 101).
UPDATE ai_prompts
SET body = REPLACE(REPLACE(REPLACE(body,
        '/superpowers:writing-plans', 'superpowers:writing-plans'),
        '/vibecodeway-load-context', 'vibecodeway-load-context'),
        '/vibecodeway-prime', 'vibecodeway-prime'),
    default_body = REPLACE(REPLACE(REPLACE(default_body,
        '/superpowers:writing-plans', 'superpowers:writing-plans'),
        '/vibecodeway-load-context', 'vibecodeway-load-context'),
        '/vibecodeway-prime', 'vibecodeway-prime')
WHERE body LIKE '%/superpowers:writing-plans%' OR body LIKE '%/vibecodeway-%'
   OR default_body LIKE '%/superpowers:writing-plans%' OR default_body LIKE '%/vibecodeway-%';

-- Skills live outside the worktrees (~/.claude, ~/.codex), which the agent_task
-- scope lock forbids; append a user-approved read-only exception for them.
UPDATE ai_prompts
SET body = REPLACE(body,
        'If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.',
        'If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.

Exception — skills: it is OK to load and read agent skills, prompts, and plugin files that live outside these worktrees (e.g. under ~/.claude or ~/.codex) — the user explicitly approves loading skills from outside the working paths. This exception is read-only: never write outside the worktrees.'),
    default_body = REPLACE(default_body,
        'If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.',
        'If the task appears to require a file outside these worktrees, STOP and ask the user instead of proceeding.

Exception — skills: it is OK to load and read agent skills, prompts, and plugin files that live outside these worktrees (e.g. under ~/.claude or ~/.codex) — the user explicitly approves loading skills from outside the working paths. This exception is read-only: never write outside the worktrees.'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = 'agent_task'
  AND body NOT LIKE '%Exception — skills:%';

-- +goose Down

UPDATE ai_prompts
SET body = REPLACE(body, '

Exception — skills: it is OK to load and read agent skills, prompts, and plugin files that live outside these worktrees (e.g. under ~/.claude or ~/.codex) — the user explicitly approves loading skills from outside the working paths. This exception is read-only: never write outside the worktrees.', ''),
    default_body = REPLACE(default_body, '

Exception — skills: it is OK to load and read agent skills, prompts, and plugin files that live outside these worktrees (e.g. under ~/.claude or ~/.codex) — the user explicitly approves loading skills from outside the working paths. This exception is read-only: never write outside the worktrees.', '')
WHERE id = 'agent_task';

UPDATE ai_prompts
SET body = REPLACE(REPLACE(REPLACE(body,
        'superpowers:writing-plans', '/superpowers:writing-plans'),
        'vibecodeway-load-context', '/vibecodeway-load-context'),
        'vibecodeway-prime', '/vibecodeway-prime'),
    default_body = REPLACE(REPLACE(REPLACE(default_body,
        'superpowers:writing-plans', '/superpowers:writing-plans'),
        'vibecodeway-load-context', '/vibecodeway-load-context'),
        'vibecodeway-prime', '/vibecodeway-prime')
WHERE body LIKE '%superpowers:writing-plans%' OR body LIKE '%vibecodeway-load-context%' OR body LIKE '%vibecodeway-prime%'
   OR default_body LIKE '%superpowers:writing-plans%' OR default_body LIKE '%vibecodeway-load-context%' OR default_body LIKE '%vibecodeway-prime%';
