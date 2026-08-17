package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
)

// TodoInput is the board-wide snapshot handed to the AI for TODO generation.
type TodoInput struct {
	JiraIssues          []appstore.BoardJiraAssignment
	MyPRs               []appstore.BoardPR
	ReviewRequests      []vcsstore.ReviewRequest
	StalePRNumbers      []int
	ImplementTicketKeys []string
	ThresholdDays       int
	Now                 time.Time
}

// Tickets whose status is terminal (done/closed/…) are noise on the board — they
// are decluttered before the AI sees them and never become implement todos. The
// canonical "done" taxonomy lives in the store package (appstore.IsTerminalTicketStatus)
// so board decluttering, context auto-archival, and the deactivate guard agree.

// downstreamTicketStatuses are states where the build work is done and the ticket is
// waiting on someone else (QA, staging, merge, review). These stay on the board so the
// AI can still emit check_status, but they are NOT open work to implement.
var downstreamTicketStatuses = map[string]bool{
	"ready for qa": true, "in qa": true, "qa": true,
	"ready for staging": true, "ready for stage": true, "in staging": true,
	"ready to merge": true, "ready for production": true, "ready for prod": true,
	"in review": true, "code review": true, "in code review": true, "pr review": true,
}

// normTicketStatus lowercases and trims a Jira status for set lookups.
func normTicketStatus(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// computeImplementTickets returns the keys of assigned tickets that are open work to
// build — actionable status (To Do / In Progress / In Development / Open / Backlog /
// etc.), excluding terminal and downstream/waiting states. These become mandatory
// implement todos, mirroring how computeStalePRs drives rerequest_pr.
func computeImplementTickets(issues []appstore.BoardJiraAssignment) []string {
	var out []string
	for _, it := range issues {
		if appstore.IsTerminalTicketStatus(it.Status) || downstreamTicketStatuses[normTicketStatus(it.Status)] {
			continue
		}
		out = append(out, it.Key)
	}
	return out
}

// activeTickets drops fully-terminal (done/closed/…) tickets so the board handed to
// the AI isn't buried under completed work. Downstream/waiting tickets are kept — they
// still warrant a check_status action.
func activeTickets(issues []appstore.BoardJiraAssignment) []appstore.BoardJiraAssignment {
	out := make([]appstore.BoardJiraAssignment, 0, len(issues))
	for _, it := range issues {
		if appstore.IsTerminalTicketStatus(it.Status) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// isPRStale reports whether a PR last updated at updatedAt is older than
// thresholdDays. Empty or unparseable timestamps are never stale.
func isPRStale(updatedAt string, now time.Time, thresholdDays int) bool {
	if updatedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return false
	}
	return now.Sub(t) >= time.Duration(thresholdDays)*24*time.Hour
}

// computeStalePRs returns the numbers of open PRs that are stale and still need
// reviewer action (not approved/merged). These become rerequest_pr todos.
func computeStalePRs(prs []appstore.BoardPR, now time.Time, thresholdDays int) []int {
	var out []int
	for _, pr := range prs {
		if pr.State != "open" {
			continue
		}
		if pr.ReviewStatus == "approved" {
			continue
		}
		if isPRStale(pr.PRUpdatedAt, now, thresholdDays) {
			out = append(out, pr.PRNumber)
		}
	}
	return out
}

const defaultTodoNextPrompt = `You are a senior engineer's assistant. Given the developer's current work board, produce a SHORT, PRIORITIZED list of concrete NEXT ACTIONS. Output a JSON array (no prose), each item:
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
IMPLEMENT_TICKETS (assigned tickets that are open work to build): {{implement_tickets}}`

func todoNextPrompt(tmpl string, in TodoInput) string {
	board, _ := json.MarshalIndent(map[string]any{
		"jiraIssues":     activeTickets(in.JiraIssues),
		"myOpenPRs":      in.MyPRs,
		"reviewRequests": in.ReviewRequests,
	}, "", "  ")
	stale := fmt.Sprintf("%v", in.StalePRNumbers)
	implement := fmt.Sprintf("%v", in.ImplementTicketKeys)
	out := strings.ReplaceAll(tmpl, "{{board}}", string(board))
	out = strings.ReplaceAll(out, "{{stale_prs}}", stale)
	out = strings.ReplaceAll(out, "{{implement_tickets}}", implement)
	return out
}

// parseTodoResponse parses the AI JSON array into items, assigning Position by
// array order and marking them AI-sourced.
func parseTodoResponse(content string) ([]appstore.AITodoItem, error) {
	raw := extractJSON(content, '[')
	var parsed []appstore.AITodoItem
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse todos: %w", err)
	}
	for i := range parsed {
		parsed[i].Position = i
		parsed[i].Source = "ai"
		if parsed[i].Priority == "" {
			parsed[i].Priority = "medium"
		}
	}
	return parsed, nil
}

// GenerateTodoNext builds the prompt from the board input, calls the LLM, parses
// the JSON list, and replaces the AI-sourced todo items in the store.
func (s *Service) GenerateTodoNext(ctx context.Context, in TodoInput) ([]appstore.AITodoItem, error) {
	in.StalePRNumbers = computeStalePRs(in.MyPRs, in.Now, in.ThresholdDays)
	in.ImplementTicketKeys = computeImplementTickets(in.JiraIssues)
	tmpl := s.loadPrompt(ctx, "todo_next_generation", defaultTodoNextPrompt)
	prompt := todoNextPrompt(tmpl, in)

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai todo generation: %w", err)
	}
	items, err := parseTodoResponse(resp.Content)
	if err != nil {
		return nil, err
	}
	if err := s.store.ReplaceTodoItems(ctx, items); err != nil {
		return nil, fmt.Errorf("persist todos: %w", err)
	}
	return s.store.ListTodoItems(ctx)
}
