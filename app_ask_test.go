package main

import (
	"testing"

	"github.com/artschekoff/context-consolidator/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAgentTools(t *testing.T) {
	a := &App{agent: agent.New(nil, nil, nil, nil)} // mirror the real agent.New signature; nil deps OK for static catalogue
	tools := a.ListAgentTools()
	require.NotEmpty(t, tools)
	names := map[string]bool{}
	for _, ti := range tools {
		names[ti.Name] = true
	}
	assert.True(t, names["search_contexts"])
}
