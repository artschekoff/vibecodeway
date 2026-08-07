// Package agent implements an "Ask AI" agent that answers questions by
// navigating stored contexts through a small set of read-only tools: it searches
// across contexts, drills into a context's elements, reads full item text, and
// follows cross-context relations to discover dependencies.
//
// Retrieval (FTS keyword search) is exposed to the model as tools rather than
// stuffing everything into the prompt, so the model reads only what it needs.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/artschekoff/context-consolidator/internal/tooling"
	"go.uber.org/zap"
)

// maxIterations caps the tool-use loop so a misbehaving model cannot spin forever.
const maxIterations = 12

// chatClient is the subset of *openai.Client the agent needs, extracted so the
// LLM turn can be faked in tests. *openai.Client satisfies it.
type chatClient interface {
	ChatWithTools(ctx context.Context, messages []openai.Message, tools []openai.ToolDef) (*openai.Response, error)
}

// Agent answers questions over the local context store using LLM tool calling.
type Agent struct {
	ai     chatClient
	reg    *tooling.Registry
	store  *appstore.Store // retained only for loadPrompt
	logger *zap.Logger
}

// New builds an Agent from the shared LLM client, tool registry, and store.
func New(ai *openai.Client, reg *tooling.Registry, store *appstore.Store, logger *zap.Logger) *Agent {
	return &Agent{ai: ai, reg: reg, store: store, logger: logger}
}

// Step records a single tool invocation for observability / UI display.
type Step struct {
	Tool   string `json:"tool"`
	Args   string `json:"args"`
	Result string `json:"result"`
}

// Answer is the final result of an Ask run.
type Answer struct {
	Content string `json:"content"`
	Steps   []Step `json:"steps"`
}

// ProgressEvent is streamed during Ask so the UI can show the agent's work live.
// Phase is "thinking" (about to call the model) or "tool" (a tool just ran).
type ProgressEvent struct {
	Phase  string `json:"phase"`
	Tool   string `json:"tool,omitempty"`
	Args   string `json:"args,omitempty"`
	Result string `json:"result,omitempty"`
}

// Ask runs the agent loop for a natural-language question and returns the answer
// plus the trace of tool calls it made along the way. orgID filters to a single
// organisation; empty string searches across all organisations. onProgress, if
// non-nil, is called before each model turn ("thinking") and after each tool
// call ("tool") so the UI can render the work live.
func (a *Agent) Ask(ctx context.Context, question, orgID string, onProgress func(ProgressEvent)) (*Answer, error) {
	emit := func(ev ProgressEvent) {
		if onProgress != nil {
			onProgress(ev)
		}
	}
	messages := []openai.Message{
		{Role: "system", Content: a.loadPrompt(ctx, "ask_ai", defaultAskAIPrompt)},
		{Role: "user", Content: question},
	}
	tools := a.toolDefs()
	answer := &Answer{}

	for i := 0; i < maxIterations; i++ {
		emit(ProgressEvent{Phase: "thinking"})
		resp, err := a.ai.ChatWithTools(ctx, messages, tools)
		if err != nil {
			return nil, err
		}

		// Record the assistant turn (content + any tool-call requests).
		messages = append(messages, openai.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			answer.Content = resp.Content
			return answer, nil
		}

		for _, tc := range resp.ToolCalls {
			result := a.dispatchTool(ctx, tc.Name, tc.Arguments, orgID)
			answer.Steps = append(answer.Steps, Step{Tool: tc.Name, Args: tc.Arguments, Result: result})
			emit(ProgressEvent{Phase: "tool", Tool: tc.Name, Args: tc.Arguments, Result: result})
			messages = append(messages, openai.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}
	}

	answer.Content = "I couldn't complete the analysis within the allowed number of steps. Try narrowing the question."
	return answer, nil
}

// loadPrompt fetches the editable Ask AI system prompt from the store, falling
// back to the built-in default if it is missing.
func (a *Agent) loadPrompt(ctx context.Context, id, fallback string) string {
	p, err := a.store.GetAIPrompt(ctx, id)
	if err != nil {
		a.logger.Warn("agent: could not load prompt from DB, using built-in default",
			zap.String("promptID", id), zap.Error(err))
		return fallback
	}
	return p.Body
}

const defaultAskAIPrompt = `You are an analyst embedded in a tool that stores work "contexts". ` +
	`Each context groups Jira issues (with comments), Slack threads, text notes, uploaded files, ` +
	`code branches, and relations to other contexts.

Answer the user's question by USING THE TOOLS to retrieve information — never invent data.
Typical flow:
1. search_contexts to find relevant contexts by keyword.
2. get_context to see a context's elements (issues, threads, texts, files, relations).
3. search_within_context to locate specific elements inside a context.
4. get_item to read the full text of a specific element.
5. list_related_contexts to follow dependencies between contexts.

Cite the concrete items you relied on (e.g. Jira keys, text names). Be concise and factual.
If you cannot find something, say so plainly.`

// toolDefs is the tool catalogue advertised to the model, built from the shared
// registry so the agent, CLI, and docs stay in sync.
func (a *Agent) toolDefs() []openai.ToolDef {
	tools := a.reg.List()
	defs := make([]openai.ToolDef, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, openai.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Params,
		})
	}
	return defs
}

// dispatchTool executes a single tool by name through the registry, always
// returning a string payload (JSON on success, a plain error string on failure)
// so the model can react.
func (a *Agent) dispatchTool(ctx context.Context, name, argsJSON, orgID string) string {
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Sprintf("error: invalid tool arguments: %v", err)
		}
	}
	payload, err := a.reg.Call(ctx, name, args, tooling.Ctx{OrgID: orgID})
	if err != nil {
		a.logger.Warn("agent.tool_error", zap.String("tool", name), zap.Error(err))
		return "error: " + err.Error()
	}
	b, mErr := json.Marshal(payload)
	if mErr != nil {
		return "error: failed to encode result"
	}
	return string(b)
}
