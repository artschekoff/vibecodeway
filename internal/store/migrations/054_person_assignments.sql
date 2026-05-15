-- +goose Up
DROP TABLE IF EXISTS context_task_assignments;

CREATE TABLE IF NOT EXISTS context_person_assignments (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL,
    name         TEXT NOT NULL,
    body         TEXT NOT NULL DEFAULT '',
    generated_at TEXT,
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS context_person_assignments_ctx_idx ON context_person_assignments (context_id);

DELETE FROM ai_prompts WHERE id IN ('task_assignment', 'task_assignment_refine');

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment',
'Person Assignment Generator',
'You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person who is an assignee or has commented in the Jira issues above.
For each person produce an entry in the JSON array below.

The "body" field must be rich Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: exact service names, API endpoints, DB tables, functions, modules, branches, PRs relevant to their work
- **Infrastructure details**: cloud services, queues, cron jobs, feature flags, env vars they need to touch
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Methods & approaches**: specific algorithms, patterns, or architectural decisions already agreed on
- **Next concrete actions**: step-by-step tasks derived from issue state and event log
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT summarise vaguely. Include exact names, IDs, values, and references found in the source data.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]',
'You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person who is an assignee or has commented in the Jira issues above.
For each person produce an entry in the JSON array below.

The "body" field must be rich Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: exact service names, API endpoints, DB tables, functions, modules, branches, PRs relevant to their work
- **Infrastructure details**: cloud services, queues, cron jobs, feature flags, env vars they need to touch
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Methods & approaches**: specific algorithms, patterns, or architectural decisions already agreed on
- **Next concrete actions**: step-by-step tasks derived from issue state and event log
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT summarise vaguely. Include exact names, IDs, values, and references found in the source data.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]',
'2026-01-01T00:00:00Z'
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment_refine',
'Person Assignment Refine',
'You are refining a single person''s task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve all technical details (service names, endpoints, DB tables, methods, infrastructure specifics) unless the instruction explicitly says to remove them.
Return ONLY the updated Markdown for this person''s body — no JSON, no preamble.',
'You are refining a single person''s task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve all technical details (service names, endpoints, DB tables, methods, infrastructure specifics) unless the instruction explicitly says to remove them.
Return ONLY the updated Markdown for this person''s body — no JSON, no preamble.',
'2026-01-01T00:00:00Z'
);

-- +goose Down
DROP INDEX IF EXISTS context_person_assignments_ctx_idx;
DROP TABLE IF EXISTS context_person_assignments;
DELETE FROM ai_prompts WHERE id IN ('person_assignment', 'person_assignment_refine');
