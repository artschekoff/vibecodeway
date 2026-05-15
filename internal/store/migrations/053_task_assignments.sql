-- +goose Up
CREATE TABLE IF NOT EXISTS context_task_assignments (
    context_id   TEXT PRIMARY KEY,
    content      TEXT NOT NULL DEFAULT '',
    generated_at TEXT,
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'task_assignment',
'Task Assignment Generator',
'You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.',
'You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.',
'2026-01-01T00:00:00Z'
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'task_assignment_refine',
'Task Assignment Refine',
'You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.',
'You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.',
'2026-01-01T00:00:00Z'
);

-- +goose Down
DROP TABLE IF EXISTS context_task_assignments;
DELETE FROM ai_prompts WHERE id IN ('task_assignment', 'task_assignment_refine');
