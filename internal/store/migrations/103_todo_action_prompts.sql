-- +goose Up

-- Seed configurable prompts for TODO NEXT per-kind draft actions.
INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES
('todo_action_rerequest_pr', 'TODO Action — Rerequest PR review',
'Write a short, friendly, professional message (2-3 sentences) asking reviewers to review a stalled pull request. Reference the PR by its key. Do not invent names. Output only the message text, no preamble.

PR: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
'Write a short, friendly, professional message (2-3 sentences) asking reviewers to review a stalled pull request. Reference the PR by its key. Do not invent names. Output only the message text, no preamble.

PR: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
datetime('now')),
('todo_action_make_pr', 'TODO Action — Draft PR description',
'Draft a concise pull request for work that is ready. Produce a one-line PR title, then a 2-4 sentence body summarizing the change and how to test it. Output only the title and body, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
'Draft a concise pull request for work that is ready. Produce a one-line PR title, then a 2-4 sentence body summarizing the change and how to test it. Output only the title and body, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
datetime('now')),
('todo_action_ask_question', 'TODO Action — Draft clarifying question',
'Write 1-3 specific, polite clarifying questions that would unblock work on a ticket that lacks detail. Be concrete. Output only the questions, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
'Write 1-3 specific, polite clarifying questions that would unblock work on a ticket that lacks detail. Be concrete. Output only the questions, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
datetime('now')),
('todo_action_check_status', 'TODO Action — Draft status ping',
'Write a brief, polite status-check message (1-2 sentences) for work that is waiting on someone else. Output only the message text, no preamble.

Ref: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
'Write a brief, polite status-check message (1-2 sentences) for work that is waiting on someone else. Output only the message text, no preamble.

Ref: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}',
datetime('now'));

-- +goose Down

DELETE FROM ai_prompts WHERE id IN ('todo_action_rerequest_pr', 'todo_action_make_pr', 'todo_action_ask_question', 'todo_action_check_status');
