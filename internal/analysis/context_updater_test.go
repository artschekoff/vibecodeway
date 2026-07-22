package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextUpdaterPrompt_fillsPlaceholders(t *testing.T) {
	tmpl := "key={{context_key}} status={{current_status}} statuses={{workflow_statuses}} text={{user_text}}"
	got := contextUpdaterPrompt(tmpl, "my-context", "in_progress", "task done", []string{"in_progress (In Progress)", "done (Done)"}, "", "")

	require.Contains(t, got, "key=my-context")
	require.Contains(t, got, "status=in_progress")
	require.Contains(t, got, "in_progress (In Progress)")
	require.Contains(t, got, "text=task done")
}

func TestContextUpdaterPrompt_emptyStatuses(t *testing.T) {
	tmpl := "statuses={{workflow_statuses}}"
	got := contextUpdaterPrompt(tmpl, "ctx", "not_started", "note", nil, "", "")
	assert.Contains(t, got, "(none configured)")
}

func TestContextUpdaterPrompt_emptyStatusList(t *testing.T) {
	tmpl := "statuses={{workflow_statuses}}"
	got := contextUpdaterPrompt(tmpl, "ctx", "not_started", "note", []string{}, "", "")
	assert.Contains(t, got, "(none configured)")
}
