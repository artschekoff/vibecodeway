-- +goose Up
-- Agent hydration becomes explicit: the <context-hydration> placeholder in a
-- terminal command is substituted with the "Agent Context Hydration" prompt
-- (ai_prompts id 'agent_hydration') at insert time. Previously the prompt was
-- auto-appended for a hardcoded list of agent launches (cursor-agent, codex);
-- rewrite those rows so behavior stays identical but is now visible and
-- editable in Settings -> Terminal commands.
UPDATE terminal_commands
SET command    = command || ' <context-hydration>',
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE (command LIKE '%cursor-agent%' OR command LIKE '%codex%')
  AND command NOT LIKE '%<context-hydration>%';

-- +goose Down
UPDATE terminal_commands
SET command    = replace(command, ' <context-hydration>', ''),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
WHERE command LIKE '%<context-hydration>%';
