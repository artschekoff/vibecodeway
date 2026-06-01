-- +goose Up
ALTER TABLE workflow_statuses ADD COLUMN ai_hint TEXT NOT NULL DEFAULT '';

UPDATE workflow_statuses SET ai_hint = 'Use when no work has begun. Transition to in_progress when active development starts, or scoping if requirements need clarification first.'
WHERE slug = 'not_started';

UPDATE workflow_statuses SET ai_hint = 'Use when active work is ongoing and there are no blockers. Transition to needs_attention if a blocker or urgent issue appears, or ready when all work is done.'
WHERE slug = 'in_progress';

UPDATE workflow_statuses SET ai_hint = 'Use when requirements or scope are still being defined, or open questions remain unresolved. Transition to in_progress once scope is clear.'
WHERE slug = 'scoping';

UPDATE workflow_statuses SET ai_hint = 'Use when an issue requires immediate attention: a just-resolved blocker now needs action, CI failures, or a critical open question that must be addressed. Transition to in_progress once addressed.'
WHERE slug = 'needs_attention';

UPDATE workflow_statuses SET ai_hint = 'Use when all work is complete and no open issues remain. Terminal status.'
WHERE slug = 'ready';

INSERT OR IGNORE INTO workflow_statuses (id, slug, label, is_system, ai_hint) VALUES (
    lower(hex(randomblob(16))), 'blocked', 'Blocked', 1,
    'Use when the context cannot proceed because it is waiting for an external prerequisite: another service to be built, a decision to be made, or a dependency to be resolved. Always state the specific blocker in status_comment. Transition to needs_attention when new artifacts show the blocker has been resolved.'
);

-- +goose Down
UPDATE workflow_statuses SET ai_hint = '' WHERE slug IN ('not_started','in_progress','scoping','needs_attention','ready');
DELETE FROM workflow_statuses WHERE slug = 'blocked';
-- SQLite cannot drop columns; ai_hint column remains but is empty after rollback
