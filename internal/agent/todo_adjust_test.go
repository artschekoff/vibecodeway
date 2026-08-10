package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	"go.uber.org/zap"
)

func TestAdjustTodosRunsToolsThenParses(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	reg := tooling.NewRegistry(tooling.Deps{Store: s})

	fake := &fakeChat{turns: []*openai.Response{
		// Turn 1: a discovery tool call (store-backed; empty store returns no hits).
		{ToolCalls: []openai.ToolCall{{ID: "1", Name: "search_contexts", Arguments: `{"query":"deploy"}`}}},
		// Turn 2: the revised list as JSON.
		{Content: `[{"kind":"check_status","title":"Check staging deploy","priority":"high"}]`},
	}}
	a := &Agent{ai: fake, reg: reg, store: s, logger: zap.NewNop(), pending: map[string]chan bool{}}

	current := []appstore.AITodoItem{{Kind: "implement", Title: "Old", Priority: "low", Position: 0}}
	got, err := a.AdjustTodos(context.Background(), current, "focus on the deploy first", "")
	if err != nil {
		t.Fatalf("AdjustTodos: %v", err)
	}
	if len(got) != 1 || got[0].Kind != "check_status" || got[0].Position != 0 {
		t.Fatalf("AdjustTodos = %+v", got)
	}
	if got[0].Source != "ai" {
		t.Fatalf("revised items must be ai-sourced: %+v", got[0])
	}
}

func TestReadToolDefsExcludesWrites(t *testing.T) {
	reg := tooling.NewRegistry(tooling.Deps{Store: nil})
	a := &Agent{reg: reg}
	for _, d := range a.readToolDefs() {
		if tool, _ := reg.Get(d.Name); tool.Write {
			t.Fatalf("readToolDefs leaked a write tool: %s", d.Name)
		}
	}
}
