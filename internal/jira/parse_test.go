package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIssueDetailPriority(t *testing.T) {
	raw := `{"key":"ABC-1","fields":{"summary":"s","status":{"name":"Open"},"priority":{"name":"High"}}}`
	d, err := parseIssueDetail(raw)
	require.NoError(t, err)
	assert.Equal(t, "High", d.Priority)
}

func TestParseSearchResultsPriorityCreated(t *testing.T) {
	raw := `[{"key":"ABC-1","fields":{"summary":"s","status":{"name":"Open"},"assignee":{"displayName":"Me"},"issuetype":{"name":"Task"},"priority":{"name":"High"},"created":"2026-07-20T10:00:00.000+0000"}}]`
	got, err := parseSearchResults(raw)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "High", got[0].Priority)
	assert.Equal(t, "2026-07-20T10:00:00.000+0000", got[0].Created)
	assert.Equal(t, "ABC-1", got[0].Key)
}
