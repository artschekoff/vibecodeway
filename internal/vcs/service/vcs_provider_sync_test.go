package vcsservice

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// mockRunner is a test double for vcs.VCSRunner.
type mockRunner struct {
	platform            vcs.Platform
	reviewRequests      []vcs.PRData
	reviewRequestsErr   error
	changesRequested    []vcs.PRData
	changesRequestedErr error
	authored            []vcs.PRData
	authoredErr         error
	prByNumber          map[int]*vcs.PRData // served by GetPRByNumber (reconcile path)
}

func (m *mockRunner) Platform() vcs.Platform { return m.platform }

func (m *mockRunner) ListReviewRequests(_ context.Context, _, _ string) ([]vcs.PRData, error) {
	return m.reviewRequests, m.reviewRequestsErr
}

func (m *mockRunner) ListChangesRequestedByUser(_ context.Context, _, _ string) ([]vcs.PRData, error) {
	return m.changesRequested, m.changesRequestedErr
}

func (m *mockRunner) ListAuthoredPRs(_ context.Context, _, _, _ string) ([]vcs.PRData, error) {
	return m.authored, m.authoredErr
}

func (m *mockRunner) ListPRsForBranch(_ context.Context, _, _, _ string) ([]vcs.PRData, error) {
	return nil, nil
}
func (m *mockRunner) GetPRComments(_ context.Context, _, _ string, _ int) ([]vcs.CommentData, error) {
	return nil, nil
}
func (m *mockRunner) ListRepoBranches(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockRunner) StartDeviceAuth(_ context.Context) (*vcs.DeviceCodeInfo, error) {
	return nil, nil
}
func (m *mockRunner) CompleteDeviceAuth(_ context.Context, _ string, _ int) (vcs.AuthResult, error) {
	return vcs.AuthResult{}, nil
}
func (m *mockRunner) GetAuthUser(_ context.Context) (string, error) { return "me", nil }
func (m *mockRunner) SetToken(_ string)                             {}
func (m *mockRunner) GetRepoDescription(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockRunner) UpdatePRBranch(_ context.Context, _, _ string, _ int) error { return nil }
func (m *mockRunner) GetPRByNumber(_ context.Context, _, _ string, prNumber int) (*vcs.PRData, error) {
	if m.prByNumber != nil {
		return m.prByNumber[prNumber], nil
	}
	return nil, nil
}

// openTestVCSStore creates an in-memory SQLite database with only the VCS tables
// needed for the review request tests, avoiding the full goose migration chain.
func openTestVCSStore(t *testing.T) *vcsstore.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vcs_repos (
			id                   TEXT PRIMARY KEY,
			platform             TEXT NOT NULL,
			owner                TEXT NOT NULL,
			name                 TEXT NOT NULL,
			local_path           TEXT NOT NULL DEFAULT '',
			label                TEXT NOT NULL DEFAULT '',
			description          TEXT NOT NULL DEFAULT '',
			on_checkout_command  TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL,
			UNIQUE (platform, owner, name)
		);
		CREATE TABLE IF NOT EXISTS vcs_review_requests (
			id            TEXT PRIMARY KEY,
			repo_id       TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
			pr_number     INTEGER NOT NULL,
			pr_title      TEXT NOT NULL DEFAULT '',
			pr_author     TEXT NOT NULL DEFAULT '',
			pr_url        TEXT NOT NULL DEFAULT '',
			review_status TEXT NOT NULL DEFAULT '',
			head_branch   TEXT NOT NULL DEFAULT '',
			requested_at  TEXT NOT NULL DEFAULT '',
			fetched_at    TEXT NOT NULL,
			reviewed_at   TEXT,
			dismissed_at  TEXT,
			viewed_at     TEXT,
			reviewers     TEXT NOT NULL DEFAULT '',
			mergeable     TEXT NOT NULL DEFAULT '',
			kind          TEXT NOT NULL DEFAULT 'requested',
			ci_status     TEXT NOT NULL DEFAULT '',
			UNIQUE (repo_id, pr_number)
		);
	`)
	require.NoError(t, err)

	return vcsstore.NewStore(db)
}

func newTestProvider(t *testing.T, runner vcs.VCSRunner) (*VCSProvider, *vcsstore.Store) {
	t.Helper()
	vs := openTestVCSStore(t)
	logger, _ := zap.NewDevelopment()
	p := NewVCSProvider(vs, logger, nil, runner)
	return p, vs
}

func TestSyncAuthoredPRsUpsertsAuthoredKind(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		authored: []vcs.PRData{{Number: 5, Title: "mine", Author: "me", State: "open",
			ReviewStatus: "changes_requested", URL: "u", HeadBranch: "b", CreatedAt: "2026-01-01T00:00:00Z",
			Mergeable: "CONFLICTING", CIStatus: "FAILURE",
			Reviewers: []vcs.ReviewerDecision{{Login: "bob", State: "changes_requested"}}}},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	if err := p.SyncAuthoredPRs(ctx, vcs.PlatformGitHub); err != nil {
		t.Fatalf("SyncAuthoredPRs: %v", err)
	}
	list, _ := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	if len(list) != 1 || list[0].Kind != "authored" || list[0].CIStatus != "FAILURE" {
		t.Fatalf("authored row not stored correctly: %+v", list)
	}
	if len(list[0].Reviewers) != 1 || list[0].Reviewers[0].Login != "bob" {
		t.Fatalf("authored reviewers not stored: %+v", list[0])
	}
	// Authored rows must never be marked "reviewed" by the requester-only sweep.
	if list[0].ReviewedAt != nil {
		t.Fatalf("authored PR must stay pending, got reviewedAt=%v", *list[0].ReviewedAt)
	}
}

// --- SyncReviewRequests tests ---

func TestSyncReviewRequests_MergesBothQueries(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{
			{Number: 1, Title: "PR one", Author: "alice", URL: "u1", ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z"},
		},
		changesRequested: []vcs.PRData{
			{Number: 2, Title: "PR two", Author: "bob", URL: "u2", ReviewStatus: "changes_requested", CreatedAt: "2026-01-02T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	// Ensure the repo exists in the store.
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, reviews, 2, "both PRs from both queries should be stored")

	byNumber := make(map[int]vcsstore.ReviewRequest, len(reviews))
	for _, r := range reviews {
		byNumber[r.PRNumber] = r
	}
	assert.Equal(t, "PR one", byNumber[1].PRTitle)
	assert.Equal(t, "review_required", byNumber[1].ReviewStatus)
	assert.Equal(t, "PR two", byNumber[2].PRTitle)
	assert.Equal(t, "changes_requested", byNumber[2].ReviewStatus)
	assert.Nil(t, byNumber[1].ReviewedAt, "active PRs must have reviewed_at = NULL")
	assert.Nil(t, byNumber[2].ReviewedAt, "active PRs must have reviewed_at = NULL")
}

func TestSyncReviewRequests_DeduplicatesOverlap(t *testing.T) {
	// PR #5 appears in both queries; the first-query status wins.
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{
			{Number: 5, Title: "Overlap", Author: "alice", URL: "u5", ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z"},
		},
		changesRequested: []vcs.PRData{
			{Number: 5, Title: "Overlap", Author: "alice", URL: "u5", ReviewStatus: "changes_requested", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	_, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, "")
	require.NoError(t, err)
	require.Len(t, reviews, 1, "duplicate PR must be stored only once")
	// review-requested takes precedence (first-seen wins).
	assert.Equal(t, "review_required", reviews[0].ReviewStatus)
}

func TestSyncReviewRequests_MarksPRsReviewedWhenAbsentFromBothQueries(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		// First sync: PR #10 is active.
		reviewRequests: []vcs.PRData{
			{Number: 10, Title: "Old PR", Author: "alice", URL: "u10", ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	_, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)

	// First sync — PR #10 inserted as active.
	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	// Second sync — PR #10 disappears from both queries.
	runner.reviewRequests = nil
	runner.changesRequested = nil
	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, "")
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.NotNil(t, reviews[0].ReviewedAt, "PR absent from both lists must have reviewed_at set")
}

func TestSyncReviewRequests_ChangesRequestedQueryErrorIsNonFatal(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{
			{Number: 7, Title: "Fallback PR", Author: "alice", URL: "u7", ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z"},
		},
		changesRequestedErr: errors.New("gh: timeout"),
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	_, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)

	// Must not return an error even if the second query fails.
	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, "")
	require.NoError(t, err)
	require.Len(t, reviews, 1, "PR from first query is stored despite second query failing")
	assert.Equal(t, 7, reviews[0].PRNumber)
}

func TestSyncReviewRequests_PersistsReviewerDecisions(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{
			{
				Number: 3, Title: "With reviewers", Author: "author", URL: "u3",
				ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z",
				Reviewers: []vcs.ReviewerDecision{
					{Login: "alice", State: "approved", SubmittedAt: "2026-01-01T10:00:00Z"},
				},
			},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	require.Len(t, reviews[0].Reviewers, 1, "reviewer decision must round-trip through the DB")
	assert.Equal(t, "alice", reviews[0].Reviewers[0].Login)
	assert.Equal(t, "approved", reviews[0].Reviewers[0].State)
}

// When the user approves a PR, it drops out of review-requested:@me BEFORE the
// review-requested fetch could capture the approval, so the stored reviewers
// snapshot is frozen without the user's own decision. The reconcile pass must
// refresh reviewers for such dropped-but-open PRs.
func TestSyncReviewRequests_RefreshesReviewersForApprovedDroppedPR(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		// First sync: PR #278 is review-requested; snapshot has only roshchyn's
		// approval — the user ("me") has not approved yet.
		reviewRequests: []vcs.PRData{
			{
				Number: 278, Title: "Feat", Author: "nazar", URL: "u278",
				ReviewStatus: "review_required", CreatedAt: "2026-01-01T00:00:00Z",
				Reviewers: []vcs.ReviewerDecision{
					{Login: "roshchyn", State: "approved", SubmittedAt: "2026-01-01T10:00:00Z"},
				},
			},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "org", "repo")
	require.NoError(t, err)
	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	// Second sync: the user approved, so #278 disappears from both lists but is
	// still open, and GetPRByNumber now reports both approvals.
	runner.reviewRequests = nil
	runner.changesRequested = nil
	runner.prByNumber = map[int]*vcs.PRData{
		278: {
			Number: 278, Title: "Feat", Author: "nazar", State: "open", URL: "u278",
			Mergeable: "CONFLICTING", // a conflict appeared after the user reviewed
			Reviewers: []vcs.ReviewerDecision{
				{Login: "roshchyn", State: "approved", SubmittedAt: "2026-01-01T10:00:00Z"},
				{Login: "me", State: "approved", SubmittedAt: "2026-01-02T09:00:00Z"},
			},
		},
	}
	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	reviews, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.NotNil(t, reviews[0].ReviewedAt, "approved PR is waiting for author")
	assert.Equal(t, "CONFLICTING", reviews[0].Mergeable, "conflict on a waiting PR must be captured by the reconcile pass")
	logins := map[string]string{}
	for _, d := range reviews[0].Reviewers {
		logins[d.Login] = d.State
	}
	assert.Equal(t, "approved", logins["me"], "the user's own approval must be captured after it drops from review-requested")
	assert.Equal(t, "approved", logins["roshchyn"])
}

func TestSyncReviewRequests_NoPlatformRunner_ReturnsError(t *testing.T) {
	vs := openTestVCSStore(t)
	logger, _ := zap.NewDevelopment()
	// Provider with no runners registered.
	p := NewVCSProvider(vs, logger, nil)

	err := p.SyncReviewRequests(context.Background(), vcs.PlatformGitHub)
	assert.Error(t, err)
}
