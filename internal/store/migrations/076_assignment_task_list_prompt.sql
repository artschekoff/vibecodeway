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
    '  - [ ] Validate the auth token before refresh `auth/service.go:42`' || char(10) ||
    '  - [ ] Add a regression test for expiry `auth/service_test.go`'
),
    default_body = replace(
    default_body,
    '- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures',
    '- **Next concrete actions**: write these as a GFM task list — one `- [ ] ` line per step, imperative, each an outcome/behavior (e.g. "add pagination to the user list"). When a specific file is implicated, append an inline code reference like `path/to/file.go:42`. Example:' || char(10) ||
    '  - [ ] Validate the auth token before refresh `auth/service.go:42`' || char(10) ||
    '  - [ ] Add a regression test for expiry `auth/service_test.go`'
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

-- +goose Down
-- No-op: prompt wording is not safety-critical and the Up is a text replace.
-- Re-running earlier migrations restores prior text if needed.
SELECT 1;
