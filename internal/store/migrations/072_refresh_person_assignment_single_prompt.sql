-- +goose Up

-- Refresh the stored person_assignment_single prompt to the softened wording introduced in
-- the Go constant defaultSinglePersonAssignmentPrompt. loadPrompt prefers the ai_prompts DB
-- row over the constant, so without this refresh the softened instruction is inert.
--
-- Surgical REPLACE of the single changed line only: idempotent (a no-op once the row already
-- carries the new text, or if a user customized this line away), and it preserves every other
-- part of the prompt. Both body and default_body are refreshed so a later "reset to default"
-- also yields the new wording.
UPDATE ai_prompts
SET body = REPLACE(
        body,
        'Include ALL sections below. Write "N/A" in any section with no data — do not omit sections.',
        'You are given an authoritative "Jira issues" block (with exact keys, current statuses, and comment bodies) plus an event log below. Extract concrete specifics from them: cite exact issue keys (e.g. JH-4264), current statuses, acceptance criteria, field/endpoint names, and branch names verbatim wherever they appear in the source. Include ALL sections below; write "N/A" in a section ONLY when the source genuinely contains nothing for it — never as a shortcut when specifics are present.'
    ),
    default_body = REPLACE(
        default_body,
        'Include ALL sections below. Write "N/A" in any section with no data — do not omit sections.',
        'You are given an authoritative "Jira issues" block (with exact keys, current statuses, and comment bodies) plus an event log below. Extract concrete specifics from them: cite exact issue keys (e.g. JH-4264), current statuses, acceptance criteria, field/endpoint names, and branch names verbatim wherever they appear in the source. Include ALL sections below; write "N/A" in a section ONLY when the source genuinely contains nothing for it — never as a shortcut when specifics are present.'
    ),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = 'person_assignment_single';
