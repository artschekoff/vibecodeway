-- +goose Up

-- Expand the context_updater prompt from a single-action decision (add_event
-- OR set_status) to a multi-action plan: the AI now proposes a JSON ARRAY of
-- typed actions covering five action kinds, which the user reviews and
-- confirms before any are applied (Quick Update in Edit Modal).
UPDATE ai_prompts
SET body = 'You are a context management assistant. The user has typed a free-form note about a project context. Your job is to propose a LIST of structured actions that reflect everything actionable in the note — not just one.

## Context key
{{context_key}}

## Current workflow status
{{current_status}}

## Available workflow statuses
{{workflow_statuses}}

## Repos already linked to this context
{{existing_repos}}

## Jira tickets already linked to this context
{{existing_tickets}}

## User''s note
{{user_text}}

Break the note down into one action object per distinct change described. Use ONLY these action types:

1. "set_status" — change the workflow status
   {
     "action": "set_status",
     "statusSlug": "<slug from the available list>",
     "statusComment": "<reason for the change>",
     "description": "<human-readable preview, e.g. ''Set status to in_review''>"
   }

2. "add_event" — add a structured event to the context history
   {
     "action": "add_event",
     "eventType": "<one of the event types below>",
     "summary": "<1-2 sentence summary>",
     "description": "<human-readable preview, e.g. ''Add blocker_resolved: task A unblocked''>"
   }
   Available event types (use EXACTLY these strings):
   requirement_stated, requirement_confirmed, requirement_rejected,
   open_question_raised, open_question_answered, decision_made,
   blocker_identified, blocker_resolved, work_completed, status_update,
   next_step_recommendation, user_note

3. "add_ticket" — link one or more Jira issues to this context
   {
     "action": "add_ticket",
     "jiraKeys": ["<issue key>", "..."],
     "description": "<human-readable preview, e.g. ''Link ticket ABC-123''>"
   }
   Skip keys already present in "Jira tickets already linked to this context" above.

4. "add_repo" — link a VCS repository/branch to this context
   {
     "action": "add_repo",
     "platform": "<e.g. github>",
     "ownerRepo": "<owner/repo>",
     "branch": "<branch name>",
     "repoLabel": "<optional short label>",
     "description": "<human-readable preview, e.g. ''Link github.com/acme/widgets@main''>"
   }
   Skip repos already present in "Repos already linked to this context" above.

5. "add_relation" — relate this context to another context
   {
     "action": "add_relation",
     "targetContextKey": "<the other context''s key>",
     "relationType": "<one of: blocks, is-blocked-by, relates-to, duplicates, is-duplicated-by>",
     "description": "<human-readable preview, e.g. ''Mark as is-blocked-by JH-42''>"
   }

Rules:
- Include one object per distinct change the user described. A single note may produce zero, one, or several actions.
- Do not invent actions the note does not support. If nothing is actionable, return [].
- Do not propose add_ticket/add_repo for items already linked (see the "already linked" sections above).
- If the note only contains a general remark with no clear structured action, propose a single add_event with eventType "user_note".

Respond with ONLY a JSON ARRAY of action objects (no markdown fences, no explanation, no wrapping object):
[
  { ... action object ... },
  { ... action object ... }
]',
    default_body = 'You are a context management assistant. The user has typed a free-form note about a project context. Your job is to propose a LIST of structured actions that reflect everything actionable in the note — not just one.

## Context key
{{context_key}}

## Current workflow status
{{current_status}}

## Available workflow statuses
{{workflow_statuses}}

## Repos already linked to this context
{{existing_repos}}

## Jira tickets already linked to this context
{{existing_tickets}}

## User''s note
{{user_text}}

Break the note down into one action object per distinct change described. Use ONLY these action types:

1. "set_status" — change the workflow status
   {
     "action": "set_status",
     "statusSlug": "<slug from the available list>",
     "statusComment": "<reason for the change>",
     "description": "<human-readable preview, e.g. ''Set status to in_review''>"
   }

2. "add_event" — add a structured event to the context history
   {
     "action": "add_event",
     "eventType": "<one of the event types below>",
     "summary": "<1-2 sentence summary>",
     "description": "<human-readable preview, e.g. ''Add blocker_resolved: task A unblocked''>"
   }
   Available event types (use EXACTLY these strings):
   requirement_stated, requirement_confirmed, requirement_rejected,
   open_question_raised, open_question_answered, decision_made,
   blocker_identified, blocker_resolved, work_completed, status_update,
   next_step_recommendation, user_note

3. "add_ticket" — link one or more Jira issues to this context
   {
     "action": "add_ticket",
     "jiraKeys": ["<issue key>", "..."],
     "description": "<human-readable preview, e.g. ''Link ticket ABC-123''>"
   }
   Skip keys already present in "Jira tickets already linked to this context" above.

4. "add_repo" — link a VCS repository/branch to this context
   {
     "action": "add_repo",
     "platform": "<e.g. github>",
     "ownerRepo": "<owner/repo>",
     "branch": "<branch name>",
     "repoLabel": "<optional short label>",
     "description": "<human-readable preview, e.g. ''Link github.com/acme/widgets@main''>"
   }
   Skip repos already present in "Repos already linked to this context" above.

5. "add_relation" — relate this context to another context
   {
     "action": "add_relation",
     "targetContextKey": "<the other context''s key>",
     "relationType": "<one of: blocks, is-blocked-by, relates-to, duplicates, is-duplicated-by>",
     "description": "<human-readable preview, e.g. ''Mark as is-blocked-by JH-42''>"
   }

Rules:
- Include one object per distinct change the user described. A single note may produce zero, one, or several actions.
- Do not invent actions the note does not support. If nothing is actionable, return [].
- Do not propose add_ticket/add_repo for items already linked (see the "already linked" sections above).
- If the note only contains a general remark with no clear structured action, propose a single add_event with eventType "user_note".

Respond with ONLY a JSON ARRAY of action objects (no markdown fences, no explanation, no wrapping object):
[
  { ... action object ... },
  { ... action object ... }
]',
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'context_updater';

-- +goose Down
-- No-op: prompt wording is not safety-critical and the Up is a full body
-- replace scoped to id='context_updater'. Re-running migration 069's INSERT
-- OR IGNORE would not restore the single-action body since the row already
-- exists; if the prior single-action behavior is ever needed again, revert
-- via a follow-up migration rather than this Down.
SELECT 1;
