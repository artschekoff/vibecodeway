-- +goose Up

-- Always refresh the factory default.
UPDATE ai_prompts
SET default_body = replace(replace(replace(
    default_body,
    '"status": "not_started|in_progress|scoping|needs_attention|ready"',
    '"status": "<one slug from the available statuses listed in {{status_definitions}}>",
  "status_comment": "<one sentence: why this status was chosen, what evidence supports it>"'
),
    '- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields',
    '- Choose status from {{status_definitions}} — each definition explains exactly when to apply it
- Read {{current_status_log_entry}} to understand the prior status and its reason before deciding whether to change it
- If prior status was "blocked" and new artifacts show the blocker is resolved, transition to "needs_attention"
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields'
),
    'Full event log (chronological)',
    'Prior status context:
{{current_status_log_entry}}

Available statuses (pick one slug for the "status" field):
{{status_definitions}}

Full event log (chronological)'
)
WHERE id = 'snapshot_build';

-- Update body only if the user hasn't customized it (still contains the old hardcoded status line).
UPDATE ai_prompts
SET body = replace(replace(replace(
    body,
    '"status": "not_started|in_progress|scoping|needs_attention|ready"',
    '"status": "<one slug from the available statuses listed in {{status_definitions}}>",
  "status_comment": "<one sentence: why this status was chosen, what evidence supports it>"'
),
    '- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields',
    '- Choose status from {{status_definitions}} — each definition explains exactly when to apply it
- Read {{current_status_log_entry}} to understand the prior status and its reason before deciding whether to change it
- If prior status was "blocked" and new artifacts show the blocker is resolved, transition to "needs_attention"
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields'
),
    'Full event log (chronological)',
    'Prior status context:
{{current_status_log_entry}}

Available statuses (pick one slug for the "status" field):
{{status_definitions}}

Full event log (chronological)'
)
WHERE id = 'snapshot_build'
  AND body LIKE '%"status": "not_started|in_progress|scoping|needs_attention|ready"%';

-- +goose Down

-- Reverse the changes to default_body.
UPDATE ai_prompts
SET default_body = replace(replace(replace(
    default_body,
    '"status": "<one slug from the available statuses listed in {{status_definitions}}>",
  "status_comment": "<one sentence: why this status was chosen, what evidence supports it>"',
    '"status": "not_started|in_progress|scoping|needs_attention|ready"'
),
    '- Choose status from {{status_definitions}} — each definition explains exactly when to apply it
- Read {{current_status_log_entry}} to understand the prior status and its reason before deciding whether to change it
- If prior status was "blocked" and new artifacts show the blocker is resolved, transition to "needs_attention"
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields',
    '- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields'
),
    'Prior status context:
{{current_status_log_entry}}

Available statuses (pick one slug for the "status" field):
{{status_definitions}}

Full event log (chronological)',
    'Full event log (chronological)'
)
WHERE id = 'snapshot_build';

-- Reverse body if it was updated.
UPDATE ai_prompts
SET body = replace(replace(replace(
    body,
    '"status": "<one slug from the available statuses listed in {{status_definitions}}>",
  "status_comment": "<one sentence: why this status was chosen, what evidence supports it>"',
    '"status": "not_started|in_progress|scoping|needs_attention|ready"'
),
    '- Choose status from {{status_definitions}} — each definition explains exactly when to apply it
- Read {{current_status_log_entry}} to understand the prior status and its reason before deciding whether to change it
- If prior status was "blocked" and new artifacts show the blocker is resolved, transition to "needs_attention"
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields',
    '- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields'
),
    'Prior status context:
{{current_status_log_entry}}

Available statuses (pick one slug for the "status" field):
{{status_definitions}}

Full event log (chronological)',
    'Full event log (chronological)'
)
WHERE id = 'snapshot_build'
  AND body LIKE '%{{status_definitions}}%';
