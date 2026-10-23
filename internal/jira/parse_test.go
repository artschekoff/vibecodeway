package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testOrgField is a sample scope/service custom field id used by the fixtures.
const testOrgField = "customfield_10160"

func TestParseIssueDetailPriority(t *testing.T) {
	raw := `{"key":"ABC-1","fields":{"summary":"s","status":{"name":"Open"},"priority":{"name":"High"}}}`
	d, err := parseIssueDetail(raw)
	require.NoError(t, err)
	assert.Equal(t, "High", d.Priority)
}

func TestParseSearchResultsPriorityCreated(t *testing.T) {
	raw := `[{"key":"ABC-1","fields":{"summary":"s","status":{"name":"Open"},"assignee":{"displayName":"Me"},"issuetype":{"name":"Task"},"priority":{"name":"High"},"created":"2026-07-20T10:00:00.000+0000"}}]`
	got, err := parseSearchResults(raw, testOrgField)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "High", got[0].Priority)
	assert.Equal(t, "2026-07-20T10:00:00.000+0000", got[0].Created)
	assert.Equal(t, "ABC-1", got[0].Key)
}

func TestParseSearchResultsOrgScope(t *testing.T) {
	// The org scope field arrives as a Jira option object; parseString pulls its "value".
	raw := `[{"key":"JH-1","fields":{"summary":"s","` + testOrgField + `":{"value":"acme-api","id":"10063"}}}]`
	got, err := parseSearchResults(raw, testOrgField)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "acme-api", got[0].Org)

	// Unset scope field -> empty org, no error.
	got, err = parseSearchResults(`[{"key":"JH-2","fields":{"summary":"s"}}]`, testOrgField)
	require.NoError(t, err)
	assert.Equal(t, "", got[0].Org)
}

// acli's `workitem search` rejects any field outside its allowlist and fails the
// whole search, so searchFields must never carry a custom field or "created".
func TestSearchFieldsStayInsideAcliAllowlist(t *testing.T) {
	assert.NotContains(t, searchFields, "customfield")
	assert.NotContains(t, searchFields, "created")
}

func TestParseIssueOrg(t *testing.T) {
	// `workitem view --fields customfield_X` shape: the field is an option object.
	got, err := parseIssueOrg(`{"key":"JH-5369","fields":{"`+testOrgField+`":{"value":"rdbms","id":"10066"}}}`, testOrgField)
	assert.NoError(t, err)
	assert.Equal(t, "rdbms", got)

	// Field unset on the issue -> empty, not an error.
	got, err = parseIssueOrg(`{"key":"JH-1","fields":{}}`, testOrgField)
	assert.NoError(t, err)
	assert.Equal(t, "", got)
}
