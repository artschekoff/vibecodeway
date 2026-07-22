-- +goose Up

-- Seed the two prompts backing "Codebase Read Mode":
--   * person_assignment_with_code: the current person_assignment body
--     (verbatim, including migration 076's scoped file-path prohibition)
--     plus a trailing "## Codebase context" section that renders the
--     {{codebase_context}} placeholder and asks the model to ground its
--     steps in the shown code with real path:line references.
--   * codebase_file_select: a short prompt used by an earlier AI step to
--     pick which repo files are worth showing to person_assignment_with_code.
-- INSERT OR IGNORE keeps this safe if a row already exists.

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment_with_code',
'Person Assignment Generator (with code)',
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
- **Next concrete actions**: write these as a GFM task list — one `- [ ] ` line per step, imperative, each an outcome/behavior (e.g. "add pagination to the user list"). When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:
- [ ] Validate the auth token before refresh `auth/service.go:42`
- [ ] Add a regression test for expiry `auth/service_test.go`
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT include specific file paths, directory paths, function names, class names, or component paths outside of the Next concrete actions task list. Describe work at the feature and behavior level — except inside the Next concrete actions task list, where you may include an inline `path:line` reference per the format above.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]

## Codebase context
Relevant files from the linked repository are shown below. Ground your steps in this actual code — cite real symbols and use inline `path:line` references that exist in these files.
{{codebase_context}}',
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
- **Next concrete actions**: write these as a GFM task list — one `- [ ] ` line per step, imperative, each an outcome/behavior (e.g. "add pagination to the user list"). When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:
- [ ] Validate the auth token before refresh `auth/service.go:42`
- [ ] Add a regression test for expiry `auth/service_test.go`
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT include specific file paths, directory paths, function names, class names, or component paths outside of the Next concrete actions task list. Describe work at the feature and behavior level — except inside the Next concrete actions task list, where you may include an inline `path:line` reference per the format above.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]

## Codebase context
Relevant files from the linked repository are shown below. Ground your steps in this actual code — cite real symbols and use inline `path:line` references that exist in these files.
{{codebase_context}}',
'2026-01-01T00:00:00Z'
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'codebase_file_select',
'Codebase File Select',
'You are selecting which repository files are relevant to a work task.
Given the file tree and the task summary, return ONLY a JSON array of at most 12 repo-relative file paths most relevant to implementing the task. No prose.

Task summary:
{{task_summary}}

File tree:
{{file_tree}}',
'You are selecting which repository files are relevant to a work task.
Given the file tree and the task summary, return ONLY a JSON array of at most 12 repo-relative file paths most relevant to implementing the task. No prose.

Task summary:
{{task_summary}}

File tree:
{{file_tree}}',
'2026-01-01T00:00:00Z'
);

-- +goose Down

DELETE FROM ai_prompts WHERE id IN ('person_assignment_with_code', 'codebase_file_select');
