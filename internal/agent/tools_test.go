package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCatalogue(t *testing.T) {
	a := &Agent{} // ToolCatalogue must not need AI/db — it only reads static defs
	cat := a.ToolCatalogue()
	require.NotNil(t, cat)
	names := map[string]bool{}
	for _, ti := range cat {
		assert.NotEmpty(t, ti.Name)
		assert.NotEmpty(t, ti.Description)
		names[ti.Name] = true
	}
	// The 5 known tools are present.
	for _, want := range []string{"search_contexts", "get_context", "search_within_context", "get_item", "list_related_contexts"} {
		assert.True(t, names[want], "missing tool %s", want)
	}
}
