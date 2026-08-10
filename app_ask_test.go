package main

import (
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
