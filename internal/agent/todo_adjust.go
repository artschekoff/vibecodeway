package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

const todoAdjustSystemPrompt = `You revise a developer's prioritized next-action list based on their instruction.
You MAY call the available read tools to discover facts (contexts, items, relations) before deciding.
When done, respond with ONLY a JSON array of the FULL revised list (no prose), each item:
{"kind":"implement|make_pr|ask_question|check_status|rerequest_pr","title":"...","rationale":"...","refType":"jira|pr|review|","refKey":"...","priority":"critical|high|medium|low"}
Apply the instruction: reorder, add, remove, or rewrite items as asked. Keep unaffected items. Most-important first.`

// readToolDefs returns only the non-write (discovery) tools. The adjust loop
// dispatches tools without the confirmation gate that Ask uses, so it must never
// expose state-mutating tools.
// ponytail: read-only by construction; if adjust ever needs a write, route it
// through the same confirmation flow as Ask rather than widening this.
func (a *Agent) readToolDefs() []openai.ToolDef {
	tools := a.reg.List()
	defs := make([]openai.ToolDef, 0, len(tools))
	for _, t := range tools {
		if t.Write {
			continue
		}
		defs = append(defs, openai.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Params})
	}
	return defs
}

// AdjustTodos revises the current list per the user's comment, allowing read-only
// tool discovery, and returns the revised items (positions reassigned by order).
func (a *Agent) AdjustTodos(ctx context.Context, current []appstore.AITodoItem, comment, orgID string) ([]appstore.AITodoItem, error) {
	curJSON, _ := json.Marshal(current)
	messages := []openai.Message{
		{Role: "system", Content: todoAdjustSystemPrompt},
		{Role: "user", Content: fmt.Sprintf("CURRENT LIST:\n%s\n\nINSTRUCTION:\n%s", string(curJSON), comment)},
	}
	tools := a.readToolDefs()

	for i := 0; i < maxIterations; i++ {
		resp, err := a.ai.ChatWithTools(ctx, messages, tools)
		if err != nil {
			return nil, err
		}
		messages = append(messages, openai.Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})

		if len(resp.ToolCalls) == 0 {
			return parseAdjustedTodos(resp.Content)
		}
		for _, tc := range resp.ToolCalls {
			result := a.dispatchTool(ctx, tc.Name, tc.Arguments, orgID)
			messages = append(messages, openai.Message{Role: "tool", Content: result, ToolCallID: tc.ID, Name: tc.Name})
		}
	}
	return nil, fmt.Errorf("adjust did not converge within %d steps", maxIterations)
}

func parseAdjustedTodos(content string) ([]appstore.AITodoItem, error) {
	// Local JSON-array extractor (strip ``` fences, seek '[').
	if idx := strings.Index(content, "```"); idx != -1 {
		content = content[idx:]
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		if end := strings.Index(content, "```"); end != -1 {
			content = content[:end]
		}
	}
	if idx := strings.IndexByte(content, '['); idx != -1 {
		content = content[idx:]
	}
	var items []appstore.AITodoItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &items); err != nil {
		return nil, fmt.Errorf("parse adjusted todos: %w", err)
	}
	for i := range items {
		items[i].Position = i
		items[i].Source = "ai"
		if items[i].Priority == "" {
			items[i].Priority = "medium"
		}
	}
	return items, nil
}
