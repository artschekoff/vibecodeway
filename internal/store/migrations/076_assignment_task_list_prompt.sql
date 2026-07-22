-- +goose Up

-- Instruct the person_assignment and person_assignment_single prompts to emit
-- their actionable-steps section as a GFM task list (`- [ ] step`) instead of
-- prose bullets / a numbered guide. A later task renders this as an
-- interactive checklist in the UI, so the model needs to consistently emit
-- `- [ ] ` lines, optionally with an inline `path:line` code reference when a
-- specific file is implicated.

UPDATE ai_prompts
SET body = replace(
    body,
    '- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures',
    '- **Next concrete actions**: write these as a GFM task list — one `- [ ] ` line per step, imperative, each an outcome/behavior (e.g. "add pagination to the user list"). When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:' || char(10) ||
    '- [ ] Validate the auth token before refresh `auth/service.go:42`' || char(10) ||
    '- [ ] Add a regression test for expiry `auth/service_test.go`'
),
    default_body = replace(
    default_body,
    '- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures',
    '- **Next concrete actions**: write these as a GFM task list — one `- [ ] ` line per step, imperative, each an outcome/behavior (e.g. "add pagination to the user list"). When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:' || char(10) ||
    '- [ ] Validate the auth token before refresh `auth/service.go:42`' || char(10) ||
    '- [ ] Add a regression test for expiry `auth/service_test.go`'
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment';

-- The closing blanket prohibition in person_assignment was written before the
-- task-list instruction above existed, and now contradicts it (it forbids the
-- very `path:line` references the checklist steps are allowed to include).
-- Scope it: keep prose/narrative sections path-free, but exempt the task list.
UPDATE ai_prompts
SET body = replace(
    body,
    'Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.',
    'Do NOT include specific file paths, directory paths, function names, class names, or component paths outside of the Next concrete actions task list. Describe work at the feature and behavior level — except inside the Next concrete actions task list, where you may include an inline `path:line` reference per the format above.'
),
    default_body = replace(
    default_body,
    'Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.',
    'Do NOT include specific file paths, directory paths, function names, class names, or component paths outside of the Next concrete actions task list. Describe work at the feature and behavior level — except inside the Next concrete actions task list, where you may include an inline `path:line` reference per the format above.'
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment';

UPDATE ai_prompts
SET body = replace(
    body,
    '### Implementation Steps' || char(10) ||
    'Numbered step-by-step guide. For each step describe:' || char(10) ||
    '1. What capability, behavior, or feature area to work on' || char(10) ||
    '2. What outcome the change should produce (describe the behavior — do NOT specify file locations or function signatures)' || char(10) ||
    '3. Expected observable result or acceptance signal',
    '### Implementation Steps' || char(10) ||
    'Write these as a GFM task list — one `- [ ] ` line per step, imperative, each describing an outcome/behavior and its acceptance signal. When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:' || char(10) ||
    '- [ ] Add pagination to the user list endpoint `api/users.go:88`' || char(10) ||
    '- [ ] Add a regression test covering the new page size param `api/users_test.go`'
),
    default_body = replace(
    default_body,
    '### Implementation Steps' || char(10) ||
    'Numbered step-by-step guide. For each step describe:' || char(10) ||
    '1. What capability, behavior, or feature area to work on' || char(10) ||
    '2. What outcome the change should produce (describe the behavior — do NOT specify file locations or function signatures)' || char(10) ||
    '3. Expected observable result or acceptance signal',
    '### Implementation Steps' || char(10) ||
    'Write these as a GFM task list — one `- [ ] ` line per step, imperative, each describing an outcome/behavior and its acceptance signal. When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:' || char(10) ||
    '- [ ] Add pagination to the user list endpoint `api/users.go:88`' || char(10) ||
    '- [ ] Add a regression test covering the new page size param `api/users_test.go`'
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment_single';

-- person_assignment_single still carries two other blanket prohibitions that
-- predate the task-list instruction above and now contradict it: the opening
-- paragraph's "must NOT contain ..." clause and the Technical Context bullet's
-- "Do NOT name ..." line. Scope both so the Implementation Steps task list is
-- exempted, matching the format the "### Implementation Steps" section above
-- now asks for.
UPDATE ai_prompts
SET body = replace(
    body,
    'The document must be detailed enough for a coding agent to execute the work, but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.',
    'The document must be detailed enough for a coding agent to execute the work. Outside of the Implementation Steps task list, it must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself. Inside the Implementation Steps task list, you may include an inline `path:line` reference per the format in that section.'
),
    default_body = replace(
    default_body,
    'The document must be detailed enough for a coding agent to execute the work, but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.',
    'The document must be detailed enough for a coding agent to execute the work. Outside of the Implementation Steps task list, it must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself. Inside the Implementation Steps task list, you may include an inline `path:line` reference per the format in that section.'
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment_single';

UPDATE ai_prompts
SET body = replace(
    body,
    '- Do NOT name specific file paths, directory paths, function names, class names, or module paths',
    '- Do NOT name specific file paths, directory paths, function names, class names, or module paths here — except as an inline `path:line` reference inside the Implementation Steps task list'
),
    default_body = replace(
    default_body,
    '- Do NOT name specific file paths, directory paths, function names, class names, or module paths',
    '- Do NOT name specific file paths, directory paths, function names, class names, or module paths here — except as an inline `path:line` reference inside the Implementation Steps task list'
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment_single';

-- +goose Down
-- No-op: prompt wording is not safety-critical and the Up is a text replace.
-- Re-running earlier migrations restores prior text if needed.
SELECT 1;
