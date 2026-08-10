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

// TestAskConfirmsWriteTool verifies a write tool only runs after the user
// approves the confirm event, and the final answer flows through.
func TestAskConfirmsWriteTool(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ran := false
	reg := &tooling.Registry{}
	reg.Register(tooling.Tool{Name: "edit_context", Write: true,
		Handler: func(_ context.Context, _ map[string]any, _ tooling.Ctx) (any, error) {
			ran = true
			return map[string]any{"ok": true}, nil
		}})

	fake := &fakeChat{turns: []*openai.Response{
		{ToolCalls: []openai.ToolCall{{ID: "call-1", Name: "edit_context", Arguments: `{"contextId":"x","status":"archived"}`}}},
		{Content: "done"},
	}}
	a := &Agent{ai: fake, reg: reg, store: s, logger: zap.NewNop(), pending: map[string]chan bool{}}

	// The confirm event's channel is registered before it's emitted, so approving
	// synchronously in the progress callback is safe (buffered channel).
	var sawConfirm bool
	ans, err := a.Ask(context.Background(), "archive it", "", func(ev ProgressEvent) {
		if ev.Phase == "confirm" {
			sawConfirm = true
			assert.Equal(t, "edit_context", ev.Tool)
			assert.NotEmpty(t, ev.CallID)
			a.ResolveConfirmation(ev.CallID, true)
		}
	})
	require.NoError(t, err)
	assert.True(t, sawConfirm, "expected a confirm progress event for the write tool")
	assert.True(t, ran, "approved write tool must execute")
	assert.Equal(t, "done", ans.Content)
}

// TestAskDeclinedWriteToolNotRun verifies a cancelled write does not execute and
// the model receives a declined result.
func TestAskDeclinedWriteToolNotRun(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ran := false
	reg := &tooling.Registry{}
	reg.Register(tooling.Tool{Name: "edit_context", Write: true,
		Handler: func(_ context.Context, _ map[string]any, _ tooling.Ctx) (any, error) {
			ran = true
			return map[string]any{"ok": true}, nil
		}})

	fake := &fakeChat{turns: []*openai.Response{
		{ToolCalls: []openai.ToolCall{{ID: "call-1", Name: "edit_context", Arguments: `{}`}}},
		{Content: "ok, skipped"},
	}}
	a := &Agent{ai: fake, reg: reg, store: s, logger: zap.NewNop(), pending: map[string]chan bool{}}

	_, err = a.Ask(context.Background(), "archive it", "", func(ev ProgressEvent) {
		if ev.Phase == "confirm" {
			a.ResolveConfirmation(ev.CallID, false)
		}
	})
	require.NoError(t, err)
	assert.False(t, ran, "declined write tool must NOT execute")
}
