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
	"sync"
	"time"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
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
	now    func() time.Time // injectable so prompt-content tests are deterministic

	mu      sync.Mutex
	pending map[string]chan bool // write confirmations keyed by tool-call ID
}

// New builds an Agent from the shared LLM client, tool registry, and store.
func New(ai *openai.Client, reg *tooling.Registry, store *appstore.Store, logger *zap.Logger) *Agent {
	return &Agent{ai: ai, reg: reg, store: store, logger: logger, now: time.Now, pending: map[string]chan bool{}}
}

// registerConfirm creates and stores a pending-confirmation channel for a write
// tool call. Registering BEFORE the confirm event is emitted means the frontend
// can never resolve a call we aren't yet listening for.
func (a *Agent) registerConfirm(callID string) chan bool {
	ch := make(chan bool, 1)
	a.mu.Lock()
	if a.pending == nil {
		a.pending = map[string]chan bool{}
	}
	a.pending[callID] = ch
	a.mu.Unlock()
	return ch
}

// ResolveConfirmation delivers the user's approve/cancel decision for a pending
// write tool call. No-op if the call is unknown or already resolved.
func (a *Agent) ResolveConfirmation(callID string, approved bool) {
	a.mu.Lock()
	ch := a.pending[callID]
	delete(a.pending, callID)
	a.mu.Unlock()
	if ch != nil {
		ch <- approved
	}
}

// awaitConfirm blocks until the pending write is resolved or ctx ends.
func (a *Agent) awaitConfirm(ctx context.Context, callID string, ch chan bool) bool {
	select {
	case ok := <-ch:
		return ok
	case <-ctx.Done():
		a.mu.Lock()
		delete(a.pending, callID)
		a.mu.Unlock()
		return false
	}
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
// Phase is "thinking" (about to call the model), "tool" (a tool just ran), or
// "confirm" (a write tool is proposed and awaiting the user's decision).
type ProgressEvent struct {
	Phase  string `json:"phase"`
	Tool   string `json:"tool,omitempty"`
	Args   string `json:"args,omitempty"`
	Result string `json:"result,omitempty"`
	CallID string `json:"callId,omitempty"` // set on "confirm" so the UI can resolve it
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
		{Role: "system", Content: dateSystemMessage(a.clock())},
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
			var result string
			if tool, ok := a.reg.Get(tc.Name); ok && tool.Write {
				// Register the waiter before emitting so the UI can't resolve early.
				ch := a.registerConfirm(tc.ID)
				emit(ProgressEvent{Phase: "confirm", Tool: tc.Name, Args: tc.Arguments, CallID: tc.ID})
				if a.awaitConfirm(ctx, tc.ID, ch) {
					result = a.dispatchTool(ctx, tc.Name, tc.Arguments, orgID)
				} else {
					result = `{"status":"declined_by_user"}`
				}
			} else {
				result = a.dispatchTool(ctx, tc.Name, tc.Arguments, orgID)
			}
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

// clock is the agent's time source. It tolerates a nil now, because several
// existing tests build &Agent{...} as a struct literal instead of calling New;
// without this guard Ask panics on them with a nil-pointer dereference.
func (a *Agent) clock() time.Time {
	if a.now == nil {
		return time.Now()
	}
	return a.now()
}

// dateSystemMessage tells the model what "today" is. Without it a question like
// "what did I do last Friday" has no anchor, and the model's only remaining move
// is to treat the weekday as a search keyword — which is exactly the bug this
// fixes.
//
// It is a separate system message rather than part of defaultAskAIPrompt because
// that prompt is user-editable and loaded from the DB: an install that has saved
// its own copy would never pick up a change to the constant.
func dateSystemMessage(now time.Time) string {
	return fmt.Sprintf(
		"Today is %s (%s), local time zone %s.\n"+
			"Resolve every relative date in the question against this date BEFORE calling a tool, "+
			"and pass absolute YYYY-MM-DD values. \"Last Friday\", \"yesterday\" and \"this week\" are date "+
			"ranges, never search keywords — never pass a word like \"Friday\" or \"пятница\" as a query.",
		now.Format("Monday, 2006-01-02"),
		now.Format(time.RFC3339),
		now.Format("MST-07:00"),
	)
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
6. list_commits to read git history by date — use this for "what did I do on <date>" questions instead of searching contexts for a date word.

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
