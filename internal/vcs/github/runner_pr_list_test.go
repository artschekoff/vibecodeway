package github

import (
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGHPRList_ParsesMergeStateStatus(t *testing.T) {
	data := []byte(`[{"number":7,"title":"t","author":{"login":"a"},"state":"OPEN",
	"isDraft":false,"reviewDecision":"","url":"u","headRefName":"feat/x",
	"mergeable":"MERGEABLE","mergeStateStatus":"BEHIND","statusCheckRollup":[],
	"createdAt":"2026-08-07T00:00:00Z","updatedAt":"2026-08-07T00:00:00Z"}]`)
	prs, err := parseGHPRList(data, "test")
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "BEHIND", prs[0].MergeStateStatus)
}

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

func TestParseGHPRSingle_ParsesReviewers(t *testing.T) {
	// `gh pr view --json ...,reviews` uses author.login/submittedAt (not the REST
	// user.login/submitted_at shape). The author's self-review is excluded and the
	// latest decision per reviewer wins.
	input := []byte(`{
		"number": 278,
		"title": "Feat",
		"author": {"login": "nazar"},
		"state": "OPEN",
		"reviewDecision": "APPROVED",
		"url": "u",
		"headRefName": "b",
		"mergeable": "MERGEABLE",
		"statusCheckRollup": [],
		"createdAt": "2026-01-01T00:00:00Z",
		"updatedAt": "2026-01-02T00:00:00Z",
		"reviews": [
			{"author": {"login": "me"}, "state": "COMMENTED", "submittedAt": "2026-01-01T08:00:00Z"},
			{"author": {"login": "me"}, "state": "APPROVED", "submittedAt": "2026-01-02T09:00:00Z"},
			{"author": {"login": "nazar"}, "state": "COMMENTED", "submittedAt": "2026-01-02T09:30:00Z"}
		]
	}`)

	got, err := parseGHPRSingle(input)
	require.NoError(t, err)
	require.Len(t, got.Reviewers, 1, "author self-review excluded; one reviewer remains")
	assert.Equal(t, "me", got.Reviewers[0].Login)
	assert.Equal(t, "approved", got.Reviewers[0].State, "APPROVED overrides the earlier COMMENTED")
}

func TestParseGHPRSingle_MalformedJSON(t *testing.T) {
	_, err := parseGHPRSingle([]byte(`not json`))
	assert.Error(t, err)
}

func TestLatestReviewStateFromJSON(t *testing.T) {
	raw := []byte(`[
		{"user":{"login":"me"},"state":"APPROVED","submitted_at":"2026-01-01T00:00:00Z"},
		{"user":{"login":"me"},"state":"DISMISSED","submitted_at":"2026-01-02T00:00:00Z"},
		{"user":{"login":"other"},"state":"CHANGES_REQUESTED","submitted_at":"2026-01-03T00:00:00Z"}
	]`)
	assert.Equal(t, "DISMISSED", latestReviewStateFromJSON(raw, "me"))
	assert.Equal(t, "CHANGES_REQUESTED", latestReviewStateFromJSON(raw, "other"))
	assert.Equal(t, "", latestReviewStateFromJSON(raw, "nobody"))
	assert.Equal(t, "", latestReviewStateFromJSON([]byte("not json"), "me"))
}

func TestLatestReviewStateFromJSON_ReapprovedWins(t *testing.T) {
	// Approved, dismissed, then approved again → latest is APPROVED, not flagged.
	raw := []byte(`[
		{"user":{"login":"me"},"state":"DISMISSED","submitted_at":"2026-01-02T00:00:00Z"},
		{"user":{"login":"me"},"state":"APPROVED","submitted_at":"2026-01-05T00:00:00Z"}
	]`)
	assert.Equal(t, "APPROVED", latestReviewStateFromJSON(raw, "me"))
}

func TestApplyReReviewStatus(t *testing.T) {
	prs := []vcs.PRData{
		{Number: 1, ReviewStatus: "review_required"},   // dismissed approval → re-review
		{Number: 2, ReviewStatus: "review_required"},   // absent from reviews → unchanged
		{Number: 3, ReviewStatus: "changes_requested"}, // re-requested after I asked for changes → re-review
		{Number: 4, ReviewStatus: "review_required"},   // re-requested after I approved → re-review
		{Number: 5, ReviewStatus: "review_required"},   // no decisive review yet → first review
	}
	reviewsByNumber := map[int][]byte{
		1: []byte(`[{"user":{"login":"me"},"state":"DISMISSED","submitted_at":"2026-01-02T00:00:00Z"}]`),
		// PR 2 intentionally absent → left unchanged.
		3: []byte(`[{"user":{"login":"me"},"state":"CHANGES_REQUESTED","submitted_at":"2026-01-02T00:00:00Z"}]`),
		4: []byte(`[{"user":{"login":"me"},"state":"APPROVED","submitted_at":"2026-01-02T00:00:00Z"}]`),
		5: []byte(`[{"user":{"login":"me"},"state":"COMMENTED","submitted_at":"2026-01-02T00:00:00Z"}]`),
	}
	applyReReviewStatus(prs, reviewsByNumber, "me")
	assert.Equal(t, "re_review_required", prs[0].ReviewStatus)
	assert.Equal(t, "review_required", prs[1].ReviewStatus)
	assert.Equal(t, "re_review_required", prs[2].ReviewStatus)
	assert.Equal(t, "re_review_required", prs[3].ReviewStatus)
	assert.Equal(t, "review_required", prs[4].ReviewStatus) // comment-only isn't a decisive prior review
}

func TestNormalizeMyReviewState(t *testing.T) {
	assert.Equal(t, "approved", normalizeMyReviewState("APPROVED"))
	assert.Equal(t, "changes_requested", normalizeMyReviewState("CHANGES_REQUESTED"))
	assert.Equal(t, "commented", normalizeMyReviewState("COMMENTED"))
	// A dismissed or still-pending review leaves no standing decision.
	assert.Equal(t, "", normalizeMyReviewState("DISMISSED"))
	assert.Equal(t, "", normalizeMyReviewState("PENDING"))
	assert.Equal(t, "", normalizeMyReviewState(""))
}

func TestApplyMyReviewStateUsesLatestOwnReview(t *testing.T) {
	reviews := []byte(`[
		{"user":{"login":"me"},"state":"CHANGES_REQUESTED","submitted_at":"2026-08-01T00:00:00Z"},
		{"user":{"login":"bob"},"state":"APPROVED","submitted_at":"2026-08-02T00:00:00Z"},
		{"user":{"login":"me"},"state":"APPROVED","submitted_at":"2026-08-03T00:00:00Z"}
	]`)
	prs := []vcs.PRData{{Number: 1}, {Number: 2}}
	applyMyReviewState(prs, map[int][]byte{1: reviews}, "me")

	require.NotNil(t, prs[0].MyReviewState)
	assert.Equal(t, "approved", *prs[0].MyReviewState)
	// PR 2 had no reviews payload fetched — left nil so callers do not overwrite.
	assert.Nil(t, prs[1].MyReviewState)
}

func TestApplyMyReviewStateEmptyWhenUserNeverReviewed(t *testing.T) {
	reviews := []byte(`[{"user":{"login":"bob"},"state":"APPROVED","submitted_at":"2026-08-02T00:00:00Z"}]`)
	prs := []vcs.PRData{{Number: 1}}
	applyMyReviewState(prs, map[int][]byte{1: reviews}, "me")

	require.NotNil(t, prs[0].MyReviewState)
	assert.Equal(t, "", *prs[0].MyReviewState)
}
