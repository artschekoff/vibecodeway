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
'You are an AI analyst generating task assignments for each participant in a project.

## Context key: {{context_key}}

## Org team members
The following people are the confirmed members of this organisation. Generate an assignment entry for EACH of them (do not add extra people):
{{org_users}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
For each person listed in "Org team members" produce an entry in the JSON array below.
If a person has no relevant work found in the source data, still include them with a brief "No active tasks found" body.

The "body" field must be Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: which features, services, or domain areas they need to work in — named at the capability level (e.g. "authentication service", "notification feature") — do NOT name specific file paths, function names, class names, or directory paths
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]',
'You are an AI analyst generating task assignments for each participant in a project.

## Context key: {{context_key}}

## Org team members
The following people are the confirmed members of this organisation. Generate an assignment entry for EACH of them (do not add extra people):
{{org_users}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
For each person listed in "Org team members" produce an entry in the JSON array below.
If a person has no relevant work found in the source data, still include them with a brief "No active tasks found" body.

The "body" field must be Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: which features, services, or domain areas they need to work in — named at the capability level (e.g. "authentication service", "notification feature") — do NOT name specific file paths, function names, class names, or directory paths
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]',
'2026-05-27T00:00:00Z'
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

Apply the user instruction. Preserve business and task details (Jira issue keys, acceptance criteria, blockers, deadlines, domain rules).
Do NOT add or preserve specific file paths, directory paths, function names, class names, or component paths.
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

Apply the user instruction. Preserve business and task details (Jira issue keys, acceptance criteria, blockers, deadlines, domain rules).
Do NOT add or preserve specific file paths, directory paths, function names, class names, or component paths.
Return ONLY the updated Markdown for this person''s body — no JSON, no preamble.',
'2026-05-27T00:00:00Z'
);

-- +goose Down
DROP INDEX IF EXISTS context_person_assignments_ctx_idx;
DROP TABLE IF EXISTS context_person_assignments;
DELETE FROM ai_prompts WHERE id IN ('person_assignment', 'person_assignment_refine');
