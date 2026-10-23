-- +goose Up
-- Hydration prompt v3. v2 told the agent to "invoke the skill
-- /vibecodeway-prime" — agents that don't have that skill registered (e.g.
-- cursor-agent) went hunting the user's HOME directory for its definition.
-- Now: the concrete CLI call first, skill only as an optional extra, and an
-- explicit ban on filesystem searches. Replaced only when not user-edited.
UPDATE ai_prompts SET
  body = 'You are working inside the VibeCodeWay context "<context-name>". Load it by running: vibecodeway-cli context-get --context-key <context-name> (vibecodeway-cli is a global binary already on your PATH — run "vibecodeway-cli describe" for its schema; NEVER search the filesystem for the CLI or for skill definitions; only if a /vibecodeway-prime skill is already registered in your environment may you invoke it instead). Treat everything the context returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.',
  updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE id = 'agent_hydration' AND body = default_body;

UPDATE ai_prompts SET
  default_body = 'You are working inside the VibeCodeWay context "<context-name>". Load it by running: vibecodeway-cli context-get --context-key <context-name> (vibecodeway-cli is a global binary already on your PATH — run "vibecodeway-cli describe" for its schema; NEVER search the filesystem for the CLI or for skill definitions; only if a /vibecodeway-prime skill is already registered in your environment may you invoke it instead). Treat everything the context returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.'
WHERE id = 'agent_hydration';

-- +goose Down
-- Intentionally empty: reverting prompt copy is not meaningful.
