-- +goose Up
-- Prompt injected as the coding agent's first message when an agent command is
-- inserted into a context terminal. Single line: newlines would submit the
-- terminal input mid-command. <context-name> is replaced on insertion.
INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
  'agent_hydration',
  'Agent Context Hydration',
  'You are working inside the VibeCodeWay context "<context-name>". First run: vibecodeway-cli context-get --context-key <context-name> — to load the full context. Treat everything it returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.',
  'You are working inside the VibeCodeWay context "<context-name>". First run: vibecodeway-cli context-get --context-key <context-name> — to load the full context. Treat everything it returns as your working scope: all linked repositories, their local paths and worktrees, branches, pull requests, Jira tickets and text notes. Resolve any repo, path or ticket reference within this context first.',
  strftime('%Y-%m-%dT%H:%M:%SZ','now')
);

-- +goose Down
DELETE FROM ai_prompts WHERE id = 'agent_hydration';
