// Package tooling defines the single registry of actions ("tools") the Ask-AI
// agent and the CLI both expose. A tool is a named, JSON-schema-described handler
// over the existing services; Write tools mutate state and are gated behind
// confirmation. The registry is the one source of truth for the agent's tool
// definitions, the CLI's agent-tool command, and the generated docs/TOOLING.md.
package tooling

import (
	"context"
	"fmt"
)

// Ctx carries per-invocation scope shared by all handlers.
type Ctx struct {
	OrgID string // restricts context search to one organisation; "" = all
}

// Handler executes a tool. args are the model/CLI-supplied arguments; the return
// value is JSON-marshalled by the caller.
type Handler func(ctx context.Context, args map[string]any, tctx Ctx) (any, error)

// Tool is one registered action.
type Tool struct {
	Name        string         // stable identifier, snake_case
	Description string         // one/two sentences for the model and the docs
	Params      map[string]any // JSON schema: {"type":"object","properties":{...},"required":[...]}
	Write       bool           // true = mutates state; requires confirmation / --yes
	Handler     Handler        `json:"-"`
}

// Registry holds tools in registration order.
type Registry struct {
	order []string
	tools map[string]Tool
}

// Register adds a tool. A later registration with the same name overwrites the
// handler but keeps the original position.
func (r *Registry) Register(t Tool) {
	if r.tools == nil {
		r.tools = map[string]Tool{}
	}
	if _, exists := r.tools[t.Name]; !exists {
		r.order = append(r.order, t.Name)
	}
	r.tools[t.Name] = t
}

// List returns tools in registration order.
func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Call dispatches to a tool's handler.
func (r *Registry) Call(ctx context.Context, name string, args map[string]any, tctx Ctx) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	if t.Handler == nil {
		return nil, fmt.Errorf("tool %q has no handler", name)
	}
	return t.Handler(ctx, args, tctx)
}
