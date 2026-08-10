package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeChat returns scripted responses, one per ChatWithTools call.
type fakeChat struct {
	turns []*openai.Response
	i     int
}

func (f *fakeChat) ChatWithTools(_ context.Context, _ []openai.Message, _ []openai.ToolDef) (*openai.Response, error) {
	r := f.turns[f.i]
	f.i++
	return r, nil
}

func TestAsk_EmitsProgress(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	fake := &fakeChat{turns: []*openai.Response{
		// Turn 1: one tool call (store-backed, so nil svc is fine).
		{ToolCalls: []openai.ToolCall{{ID: "1", Name: "search_contexts", Arguments: `{"query":"x"}`}}},
		// Turn 2: final answer, no tool calls.
		{Content: "done"},
	}}
	reg := tooling.NewRegistry(tooling.Deps{Store: s, Logger: zap.NewNop()}) // svc nil — search_contexts doesn't use it
	a := &Agent{ai: fake, reg: reg, store: s, logger: zap.NewNop()}

	var events []ProgressEvent
	ans, err := a.Ask(context.Background(), "q", "", func(ev ProgressEvent) {
		events = append(events, ev)
	})
	require.NoError(t, err)
	assert.Equal(t, "done", ans.Content)

	var thinking, tool int
	var foundTool bool
	for _, e := range events {
		if e.Phase == "thinking" {
			thinking++
		}
		if e.Phase == "tool" {
			tool++
			if e.Tool == "search_contexts" {
				foundTool = true
			}
		}
	}
	assert.GreaterOrEqual(t, thinking, 1, "expected a thinking event before an LLM call")
	assert.Equal(t, 1, tool, "expected exactly one tool progress event")
	assert.True(t, foundTool, "expected the tool event to name search_contexts")
}
