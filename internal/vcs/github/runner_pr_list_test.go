package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGHPRList_HappyPath(t *testing.T) {
	input := []byte(`[
		{
			"number": 42,
			"title": "Fix the thing",
			"author": {"login": "alice"},
			"state": "OPEN",
			"reviewDecision": "CHANGES_REQUESTED",
			"url": "https://github.com/org/repo/pull/42",
			"headRefName": "fix-the-thing",
			"mergeable": "MERGEABLE",
			"statusCheckRollup": [{"state": "SUCCESS"}, {"state": "SUCCESS"}],
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-02T10:00:00Z"
		},
		{
			"number": 99,
			"title": "Another PR",
			"author": {"login": "bob"},
			"state": "OPEN",
			"reviewDecision": "REVIEW_REQUIRED",
			"url": "https://github.com/org/repo/pull/99",
			"headRefName": "another-pr",
			"mergeable": "UNKNOWN",
			"statusCheckRollup": [{"state": "PENDING"}],
			"createdAt": "2026-04-03T10:00:00Z",
			"updatedAt": "2026-04-03T12:00:00Z"
		}
	]`)

	got, err := parseGHPRList(input, "test")
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, 42, got[0].Number)
	assert.Equal(t, "Fix the thing", got[0].Title)
	assert.Equal(t, "alice", got[0].Author)
	assert.Equal(t, "open", got[0].State)
	assert.Equal(t, "changes_requested", got[0].ReviewStatus)
	assert.Equal(t, "https://github.com/org/repo/pull/42", got[0].URL)
	assert.Equal(t, "fix-the-thing", got[0].HeadBranch)
	assert.Equal(t, "MERGEABLE", got[0].Mergeable)
	assert.Equal(t, "SUCCESS", got[0].CIStatus)

	assert.Equal(t, 99, got[1].Number)
	assert.Equal(t, "review_required", got[1].ReviewStatus)
	assert.Equal(t, "PENDING", got[1].CIStatus)
}

func TestParseGHPRList_EmptyArray(t *testing.T) {
	got, err := parseGHPRList([]byte(`[]`), "empty")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseGHPRList_InvalidJSON(t *testing.T) {
	_, err := parseGHPRList([]byte(`not json`), "bad")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestParseGHPRList_MixedCIStatus_FailureWins(t *testing.T) {
	input := []byte(`[{
		"number": 1,
		"title": "T",
		"author": {"login": "u"},
		"state": "OPEN",
		"reviewDecision": "",
		"url": "",
		"headRefName": "",
		"mergeable": "",
		"statusCheckRollup": [{"state": "SUCCESS"}, {"state": "FAILURE"}],
		"createdAt": "",
		"updatedAt": ""
	}]`)
	got, err := parseGHPRList(input, "mixed")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "FAILURE", got[0].CIStatus)
}

func TestParseGHPRList_NoChecks_EmptyCIStatus(t *testing.T) {
	input := []byte(`[{
		"number": 5,
		"title": "No CI",
		"author": {"login": "u"},
		"state": "OPEN",
		"reviewDecision": "APPROVED",
		"url": "",
		"headRefName": "",
		"mergeable": "",
		"statusCheckRollup": [],
		"createdAt": "",
		"updatedAt": ""
	}]`)
	got, err := parseGHPRList(input, "noci")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "approved", got[0].ReviewStatus)
	assert.Equal(t, "", got[0].CIStatus)
}

func TestParseGHPRSingle_HappyPath(t *testing.T) {
	input := []byte(`{
		"number": 123,
		"title": "Add login feature",
		"author": {"login": "alice"},
		"state": "OPEN",
		"reviewDecision": "REVIEW_REQUIRED",
		"url": "https://github.com/org/repo/pull/123",
		"headRefName": "feature/login",
		"mergeable": "MERGEABLE",
		"statusCheckRollup": [{"state": "SUCCESS"}, {"state": "SUCCESS"}],
		"createdAt": "2026-04-20T09:00:00Z",
		"updatedAt": "2026-04-21T10:00:00Z"
	}`)

	got, err := parseGHPRSingle(input)
	require.NoError(t, err)
	assert.Equal(t, 123, got.Number)
	assert.Equal(t, "Add login feature", got.Title)
	assert.Equal(t, "alice", got.Author)
	assert.Equal(t, "open", got.State)
	assert.Equal(t, "review_required", got.ReviewStatus)
	assert.Equal(t, "feature/login", got.HeadBranch)
	assert.Equal(t, "MERGEABLE", got.Mergeable)
	assert.Equal(t, "SUCCESS", got.CIStatus)
}

func TestParseGHPRSingle_MalformedJSON(t *testing.T) {
	_, err := parseGHPRSingle([]byte(`not json`))
	assert.Error(t, err)
}
