package vcsservice

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/vcs"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

// mockRunner is a test double for vcs.VCSRunner.
type mockRunner struct {
	platform             vcs.Platform
	reviewRequests       []vcs.PRData
	reviewRequestsErr    error
	changesRequested     []vcs.PRData
	changesRequestedErr  error
}

func (m *mockRunner) Platform() vcs.Platform { return m.platform }

func (m *mockRunner) ListReviewRequests(_ context.Context, _, _ string) ([]vcs.PRData, error) {
	return m.reviewRequests, m.reviewRequestsErr
}

func (m *mockRunner) ListChangesRequestedByUser(_ context.Context, _, _ string) ([]vcs.PRData, error) {
	return m.changesRequested, m.changesRequestedErr
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
func (m *mockRunner) GetPRByNumber(_ context.Context, _, _ string, _ int) (*vcs.PRData, error) {
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
			id          TEXT PRIMARY KEY,
			platform    TEXT NOT NULL,
			owner       TEXT NOT NULL,
			name        TEXT NOT NULL,
			local_path  TEXT NOT NULL DEFAULT '',
			label       TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
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
			requested_at  TEXT NOT NULL DEFAULT '',
			fetched_at    TEXT NOT NULL,
			reviewed_at   TEXT,
			dismissed_at  TEXT,
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
	p := NewVCSProvider(vs, logger, runner)
	return p, vs
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

func TestSyncReviewRequests_NoPlatformRunner_ReturnsError(t *testing.T) {
	vs := openTestVCSStore(t)
	logger, _ := zap.NewDevelopment()
	// Provider with no runners registered.
	p := NewVCSProvider(vs, logger)

	err := p.SyncReviewRequests(context.Background(), vcs.PlatformGitHub)
	assert.Error(t, err)
}
