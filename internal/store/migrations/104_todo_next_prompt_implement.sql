-- +goose Up

-- Make the TODO NEXT generator surface NEW/actionable Jira tickets as mandatory
-- "implement" steps (mirroring how STALE_PRS drives mandatory rerequest_pr), and
-- add the {{implement_tickets}} placeholder the Go builder substitutes.

-- Refresh the body ONLY for users who haven't customized it (body still equals the
-- old factory default). Compare against default_body BEFORE we overwrite it below.
UPDATE ai_prompts
SET body = 'You are a senior engineer''s assistant. Given the developer''s current work board, produce a SHORT, PRIORITIZED list of concrete NEXT ACTIONS. Output a JSON array (no prose), each item:
{"kind":"implement|make_pr|ask_question|check_status|rerequest_pr","title":"imperative, specific","rationale":"1 sentence why now","refType":"jira|pr|review|","refKey":"e.g. JH-4922 or owner/repo#218","priority":"critical|high|medium|low"}

Rules:
- Order the array most-important first. Weigh Jira priority (Highest/High first), blocked/at-risk work, and review turnaround.
- MANDATORY: For every PR listed in STALE_PRS, emit a rerequest_pr action for it.
- MANDATORY: For every ticket key listed in IMPLEMENT_TICKETS, emit an implement action (title like "Implement <key>: <summary>"), UNLESS that ticket already has an open PR in myOpenPRs — then prefer make_pr or check_status for it instead.
- For remaining tickets: if a ticket lacks clarity, emit ask_question; if work is done but no PR exists, emit make_pr; if waiting on others (e.g. Ready for QA/Staging/Merge), emit check_status.
- Always include EVERY mandatory action above. Beyond those, keep to the ~7 highest-leverage discretionary actions. Do not invent tickets or PRs not present in the board.

BOARD:
{{board}}

STALE_PRS (PR numbers awaiting review beyond the threshold): {{stale_prs}}
IMPLEMENT_TICKETS (assigned tickets that are open work to build): {{implement_tickets}}'
WHERE id = 'todo_next_generation' AND body = default_body;

-- Always refresh the factory default so a future Reset yields the new version.
UPDATE ai_prompts
SET default_body = 'You are a senior engineer''s assistant. Given the developer''s current work board, produce a SHORT, PRIORITIZED list of concrete NEXT ACTIONS. Output a JSON array (no prose), each item:
{"kind":"implement|make_pr|ask_question|check_status|rerequest_pr","title":"imperative, specific","rationale":"1 sentence why now","refType":"jira|pr|review|","refKey":"e.g. JH-4922 or owner/repo#218","priority":"critical|high|medium|low"}

Rules:
- Order the array most-important first. Weigh Jira priority (Highest/High first), blocked/at-risk work, and review turnaround.
- MANDATORY: For every PR listed in STALE_PRS, emit a rerequest_pr action for it.
- MANDATORY: For every ticket key listed in IMPLEMENT_TICKETS, emit an implement action (title like "Implement <key>: <summary>"), UNLESS that ticket already has an open PR in myOpenPRs — then prefer make_pr or check_status for it instead.
- For remaining tickets: if a ticket lacks clarity, emit ask_question; if work is done but no PR exists, emit make_pr; if waiting on others (e.g. Ready for QA/Staging/Merge), emit check_status.
- Always include EVERY mandatory action above. Beyond those, keep to the ~7 highest-leverage discretionary actions. Do not invent tickets or PRs not present in the board.

BOARD:
{{board}}

STALE_PRS (PR numbers awaiting review beyond the threshold): {{stale_prs}}
IMPLEMENT_TICKETS (assigned tickets that are open work to build): {{implement_tickets}}'
WHERE id = 'todo_next_generation';

-- +goose Down

-- Revert to the pre-implement-tickets prompt (099_todo_next_prompt.sql). Restore the
-- body only for users who hadn't customized it (body still equals the implement default).
UPDATE ai_prompts
SET body = 'You are a senior engineer''s assistant. Given the developer''s current work board, produce a SHORT, PRIORITIZED list of concrete NEXT ACTIONS. Output a JSON array (no prose), each item:
{"kind":"implement|make_pr|ask_question|check_status|rerequest_pr","title":"imperative, specific","rationale":"1 sentence why now","refType":"jira|pr|review|","refKey":"e.g. JH-4922 or owner/repo#218","priority":"critical|high|medium|low"}

Rules:
- Order the array most-important first. Weigh Jira priority (Highest/High first), blocked/at-risk work, and review turnaround.
- If a PR is listed in STALE_PRS, emit a rerequest_pr action for it.
- If a ticket lacks clarity, emit ask_question. If work is done but no PR exists, emit make_pr. If waiting on others, emit check_status. Otherwise implement.
- Keep it to the ~7 highest-leverage actions. Do not invent tickets or PRs not present in the board.

BOARD:
{{board}}

STALE_PRS (PR numbers awaiting review beyond the threshold): {{stale_prs}}'
WHERE id = 'todo_next_generation' AND body = default_body;

UPDATE ai_prompts
SET default_body = 'You are a senior engineer''s assistant. Given the developer''s current work board, produce a SHORT, PRIORITIZED list of concrete NEXT ACTIONS. Output a JSON array (no prose), each item:
{"kind":"implement|make_pr|ask_question|check_status|rerequest_pr","title":"imperative, specific","rationale":"1 sentence why now","refType":"jira|pr|review|","refKey":"e.g. JH-4922 or owner/repo#218","priority":"critical|high|medium|low"}

Rules:
- Order the array most-important first. Weigh Jira priority (Highest/High first), blocked/at-risk work, and review turnaround.
- If a PR is listed in STALE_PRS, emit a rerequest_pr action for it.
- If a ticket lacks clarity, emit ask_question. If work is done but no PR exists, emit make_pr. If waiting on others, emit check_status. Otherwise implement.
- Keep it to the ~7 highest-leverage actions. Do not invent tickets or PRs not present in the board.

BOARD:
{{board}}

STALE_PRS (PR numbers awaiting review beyond the threshold): {{stale_prs}}'
WHERE id = 'todo_next_generation';
