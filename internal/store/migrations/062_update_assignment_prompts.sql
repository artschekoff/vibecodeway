-- +goose Up

-- Update person_assignment: remove file/function specificity, describe work at capability level
UPDATE ai_prompts SET
    body = 'You are an AI analyst generating task assignments for each participant in a project.

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
    default_body = 'You are an AI analyst generating task assignments for each participant in a project.

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
    updated_at = '2026-05-27T00:00:00Z'
WHERE id = 'person_assignment';

-- Update person_assignment_refine: stop preserving file/function specifics
UPDATE ai_prompts SET
    body = 'You are refining a single person''s task assignment document.

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
    default_body = 'You are refining a single person''s task assignment document.

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
    updated_at = '2026-05-27T00:00:00Z'
WHERE id = 'person_assignment_refine';

-- Seed person_assignment_single (not previously in DB — falls back to code constant)
INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment_single',
'Person Assignment (Single)',
'You are a senior technical lead writing a task assignment for a single developer.
The document must be detailed enough for a coding agent to execute the work, but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.

## Context key: {{context_key}}

## Developer: {{person_name}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_prompt}}
## Task
Write a comprehensive task assignment for **{{person_name}} only**.
Focus exclusively on what this person owns or has commented on.

Include ALL sections below. Write "N/A" in any section with no data — do not omit sections.

### Current Assignment
- Exact Jira issue keys, summaries, and current statuses they own
- Acceptance criteria extracted from issue descriptions and comments

### Technical Context
- Which features, services, or domain areas to work in (named at the capability level, e.g. "authentication service", "dashboard feature")
- API endpoints, HTTP methods, request/response field names and types (only if found in the source data)
- Data entities, field names, validation rules, business logic constraints (only if found in the source data)
- Active branch name(s) and open PR numbers or URLs (if mentioned)
- Do NOT name specific file paths, directory paths, function names, class names, or module paths

### Implementation Steps
Numbered step-by-step guide. For each step describe:
1. What capability, behavior, or feature area to work on
2. What outcome the change should produce (describe the behavior — do NOT specify file locations or function signatures)
3. Expected observable result or acceptance signal

### Infrastructure & Configuration
- Environment variables and feature flags required (if found in source data)
- External services, queues, webhooks, or cron jobs to configure (if mentioned)
- Required DB migrations or deployment actions (if mentioned)

### Testing Requirements
- Behaviors and scenarios to cover with tests
- How to verify the full implementation is correct end-to-end

### Blockers & Dependencies
- Unanswered questions that block this work
- Other people''s tasks this work depends on

Return ONLY the Markdown body — no JSON wrapper, no preamble.',
'You are a senior technical lead writing a task assignment for a single developer.
The document must be detailed enough for a coding agent to execute the work, but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.

## Context key: {{context_key}}

## Developer: {{person_name}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_prompt}}
## Task
Write a comprehensive task assignment for **{{person_name}} only**.
Focus exclusively on what this person owns or has commented on.

Include ALL sections below. Write "N/A" in any section with no data — do not omit sections.

### Current Assignment
- Exact Jira issue keys, summaries, and current statuses they own
- Acceptance criteria extracted from issue descriptions and comments

### Technical Context
- Which features, services, or domain areas to work in (named at the capability level, e.g. "authentication service", "dashboard feature")
- API endpoints, HTTP methods, request/response field names and types (only if found in the source data)
- Data entities, field names, validation rules, business logic constraints (only if found in the source data)
- Active branch name(s) and open PR numbers or URLs (if mentioned)
- Do NOT name specific file paths, directory paths, function names, class names, or module paths

### Implementation Steps
Numbered step-by-step guide. For each step describe:
1. What capability, behavior, or feature area to work on
2. What outcome the change should produce (describe the behavior — do NOT specify file locations or function signatures)
3. Expected observable result or acceptance signal

### Infrastructure & Configuration
- Environment variables and feature flags required (if found in source data)
- External services, queues, webhooks, or cron jobs to configure (if mentioned)
- Required DB migrations or deployment actions (if mentioned)

### Testing Requirements
- Behaviors and scenarios to cover with tests
- How to verify the full implementation is correct end-to-end

### Blockers & Dependencies
- Unanswered questions that block this work
- Other people''s tasks this work depends on

Return ONLY the Markdown body — no JSON wrapper, no preamble.',
'2026-05-27T00:00:00Z'
);

-- Seed person_assignment_incremental (not previously in DB — falls back to code constant)
INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment_incremental',
'Person Assignment Incremental',
'You are updating a developer''s task assignment based on new context.
Maximum output: {{max_tokens_hint}} words. Be surgical — only modify sections directly affected by the new messages.

## Developer: {{person_name}}

## Current Assignment
{{current_body}}

## New Messages (appeared since last update)
{{delta_content}}

## Recent Events
{{events}}

---
EXAMPLE (do not copy verbatim — illustrates edit style):

New message: "[Slack] @alice: API deadline moved to Friday, need auth endpoint done by EOD Thursday"

Before excerpt:
### Authentication Feature
- Implement login endpoint (timeline: next sprint)

After excerpt:
### Authentication Feature
- Implement login endpoint (**deadline: Thursday EOD** — Slack update from @alice)

---

Update the assignment. Preserve all unchanged sections verbatim. Do NOT add specific file paths, function names, class names, or directory paths.
Return ONLY the updated Markdown body.',
'You are updating a developer''s task assignment based on new context.
Maximum output: {{max_tokens_hint}} words. Be surgical — only modify sections directly affected by the new messages.

## Developer: {{person_name}}

## Current Assignment
{{current_body}}

## New Messages (appeared since last update)
{{delta_content}}

## Recent Events
{{events}}

---
EXAMPLE (do not copy verbatim — illustrates edit style):

New message: "[Slack] @alice: API deadline moved to Friday, need auth endpoint done by EOD Thursday"

Before excerpt:
### Authentication Feature
- Implement login endpoint (timeline: next sprint)

After excerpt:
### Authentication Feature
- Implement login endpoint (**deadline: Thursday EOD** — Slack update from @alice)

---

Update the assignment. Preserve all unchanged sections verbatim. Do NOT add specific file paths, function names, class names, or directory paths.
Return ONLY the updated Markdown body.',
'2026-05-27T00:00:00Z'
);

-- +goose Down
UPDATE ai_prompts SET body = default_body WHERE id IN ('person_assignment', 'person_assignment_refine');
DELETE FROM ai_prompts WHERE id IN ('person_assignment_single', 'person_assignment_incremental');
