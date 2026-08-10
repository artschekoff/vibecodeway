-- +goose Up

-- The prompt/command seeds in migrations 088/092/095 embed the old slash command
-- "/context-consolidator-load-context". Those migrations are immutable, so patch
-- the live rows here to the renamed command.
UPDATE ai_prompts
SET body = REPLACE(body, '/context-consolidator-load-context', '/vibecodeway-load-context'),
    default_body = REPLACE(default_body, '/context-consolidator-load-context', '/vibecodeway-load-context')
WHERE body LIKE '%/context-consolidator-load-context%'
   OR default_body LIKE '%/context-consolidator-load-context%';

-- +goose Down

UPDATE ai_prompts
SET body = REPLACE(body, '/vibecodeway-load-context', '/context-consolidator-load-context'),
    default_body = REPLACE(default_body, '/vibecodeway-load-context', '/context-consolidator-load-context')
WHERE body LIKE '%/vibecodeway-load-context%'
   OR default_body LIKE '%/vibecodeway-load-context%';
