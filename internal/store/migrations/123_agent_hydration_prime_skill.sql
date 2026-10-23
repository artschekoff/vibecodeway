-- +goose Up
-- Hydration prompt v2: route the agent through the /vibecodeway-prime skill
-- (which loads the CLI schema) instead of a raw context-get invocation.
-- Body is only replaced when the user has not customized it (body = default).
UPDATE ai_prompts SET
  body = 'You are working inside the VibeCodeWay context "<context-name>". First invoke the skill /vibecodeway-prime, then load this context with /vibecodeway-load-context <context-name>. Treat everything it returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.',
  updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = 'agent_hydration' AND body = default_body;

UPDATE ai_prompts SET
  default_body = 'You are working inside the VibeCodeWay context "<context-name>". First invoke the skill /vibecodeway-prime, then load this context with /vibecodeway-load-context <context-name>. Treat everything it returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.'
WHERE id = 'agent_hydration';

-- +goose Down
-- Intentionally empty: reverting prompt copy is not meaningful.
