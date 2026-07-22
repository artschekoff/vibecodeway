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
