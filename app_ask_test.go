package main

import (
	"context"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/agent"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAgentTools(t *testing.T) {
	// The catalogue is read from the registry; nil deps are fine because
	// ToolCatalogue only reads tool names/descriptions, never invokes handlers.
	reg := tooling.NewRegistry(tooling.Deps{})
	a := &App{agent: agent.New(nil, reg, nil, nil)}
	tools := a.ListAgentTools()
	require.NotEmpty(t, tools)
	names := map[string]bool{}
	for _, ti := range tools {
		names[ti.Name] = true
	}
	assert.True(t, names["search_contexts"])
}

// TestAskAIChatRejectsMalformedJSON verifies AskAIChat never reaches the agent
// with a broken history payload — it should fail fast with a wrapped error
// instead of asking the LLM with a nonsensical prior conversation.
func TestAskAIChatRejectsMalformedJSON(t *testing.T) {
	reg := tooling.NewRegistry(tooling.Deps{})
	a := &App{agent: agent.New(nil, reg, nil, nil), ctx: context.Background()}

	_, err := a.AskAIChat("{not valid json", "question", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid chat history")
}

// TestSummarizeAskChatRejectsMalformedJSON mirrors the AskAIChat case for the
// summarize binding, which shares the same history-decoding helper.
func TestSummarizeAskChatRejectsMalformedJSON(t *testing.T) {
	reg := tooling.NewRegistry(tooling.Deps{})
	a := &App{agent: agent.New(nil, reg, nil, nil), ctx: context.Background()}

	_, err := a.SummarizeAskChat("[{\"role\": }]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid chat history")
}
