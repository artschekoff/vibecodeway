package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// TodoInput is the board-wide snapshot handed to the AI for TODO generation.
type TodoInput struct {
	JiraIssues     []appstore.BoardJiraAssignment
	MyPRs          []appstore.BoardPR
	ReviewRequests []vcsstore.ReviewRequest
	StalePRNumbers []int
	ThresholdDays  int
	Now            time.Time
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
- If a PR is listed in STALE_PRS, emit a rerequest_pr action for it.
- If a ticket lacks clarity, emit ask_question. If work is done but no PR exists, emit make_pr. If waiting on others, emit check_status. Otherwise implement.
- Keep it to the ~7 highest-leverage actions. Do not invent tickets or PRs not present in the board.

BOARD:
{{board}}

STALE_PRS (PR numbers awaiting review beyond the threshold): {{stale_prs}}`

func todoNextPrompt(tmpl string, in TodoInput) string {
	board, _ := json.MarshalIndent(map[string]any{
		"jiraIssues":     in.JiraIssues,
		"myOpenPRs":      in.MyPRs,
		"reviewRequests": in.ReviewRequests,
	}, "", "  ")
	stale := fmt.Sprintf("%v", in.StalePRNumbers)
	out := strings.ReplaceAll(tmpl, "{{board}}", string(board))
	out = strings.ReplaceAll(out, "{{stale_prs}}", stale)
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
