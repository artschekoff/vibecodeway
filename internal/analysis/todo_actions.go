package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/openai"
)

// Default bodies for the per-kind TODO action prompts. These mirror the seed in
// migration 103_todo_action_prompts.sql (keep both byte-identical). Placeholders
// {{title}}, {{refKey}}, {{rationale}} are substituted at draft time.
const (
	defaultRerequestPRPrompt = `Write a short, friendly, professional message (2-3 sentences) asking reviewers to review a stalled pull request. Reference the PR by its key. Do not invent names. Output only the message text, no preamble.

PR: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}`

	defaultMakePRPrompt = `Draft a concise pull request for work that is ready. Produce a one-line PR title, then a 2-4 sentence body summarizing the change and how to test it. Output only the title and body, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}`

	defaultAskQuestionPrompt = `Write 1-3 specific, polite clarifying questions that would unblock work on a ticket that lacks detail. Be concrete. Output only the questions, no preamble.

Ticket: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}`

	defaultCheckStatusPrompt = `Write a brief, polite status-check message (1-2 sentences) for work that is waiting on someone else. Output only the message text, no preamble.

Ref: {{refKey}}
Todo: {{title}}
Why now: {{rationale}}`
)

// todoActionPrompt maps a todo kind to its configurable prompt id and built-in
// default body. Kinds absent here have no draft action.
var todoActionPrompt = map[string]struct{ id, fallback string }{
	"rerequest_pr": {"todo_action_rerequest_pr", defaultRerequestPRPrompt},
	"make_pr":      {"todo_action_make_pr", defaultMakePRPrompt},
	"ask_question": {"todo_action_ask_question", defaultAskQuestionPrompt},
	"check_status": {"todo_action_check_status", defaultCheckStatusPrompt},
}

// renderTodoActionPrompt substitutes the todo fields into the prompt template.
func renderTodoActionPrompt(tmpl, title, refKey, rationale string) string {
	if refKey == "" {
		refKey = "(none)"
	}
	out := strings.ReplaceAll(tmpl, "{{title}}", title)
	out = strings.ReplaceAll(out, "{{refKey}}", refKey)
	out = strings.ReplaceAll(out, "{{rationale}}", rationale)
	return out
}

// DraftTodoMessage drafts a copy-ready message for a todo action using the
// kind's configurable prompt. Returns an error for kinds with no draft action.
func (s *Service) DraftTodoMessage(ctx context.Context, kind, title, refKey, rationale string) (string, error) {
	spec, ok := todoActionPrompt[kind]
	if !ok {
		return "", fmt.Errorf("no draft action for kind %q", kind)
	}
	tmpl := s.loadPrompt(ctx, spec.id, spec.fallback)
	prompt := renderTodoActionPrompt(tmpl, title, refKey, rationale)
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", fmt.Errorf("draft todo message: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}
