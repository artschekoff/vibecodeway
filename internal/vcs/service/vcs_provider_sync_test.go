package vcsservice

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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
	branchPRCalls       []string            // branches passed to ListPRsForBranch
	branchPRs           []vcs.PRData        // returned by ListPRsForBranch when branch == "" (fetch-all mode)
	branchPRsErr        error
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

func (m *mockRunner) ListPRsForBranch(_ context.Context, _, _, branch string) ([]vcs.PRData, error) {
	m.branchPRCalls = append(m.branchPRCalls, branch)
	if m.branchPRsErr != nil {
		return nil, m.branchPRsErr
	}
	return m.branchPRs, nil
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
	// modernc.org/sqlite hands each new connection its own private ":memory:"
	// database, so a pool with >1 connections can silently miss tables created
	// on a different connection. sync()'s branch worker pool now issues
	// concurrent writes (up to 4 goroutines), which surfaced this. Mirrors the
	// SetMaxOpenConns(1) already used for sqlite in internal/store/store.go.
	db.SetMaxOpenConns(1)

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
		CREATE TABLE IF NOT EXISTS context_vcs_branches (
			id            TEXT PRIMARY KEY,
			context_id    TEXT NOT NULL,
			repo_id       TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
			branch        TEXT NOT NULL,
			label         TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL,
			autodetect_pr INTEGER NOT NULL DEFAULT 1,
			UNIQUE (context_id, repo_id, branch)
		);
		CREATE TABLE IF NOT EXISTS vcs_pull_requests (
			id                  TEXT PRIMARY KEY,
			repo_id             TEXT NOT NULL REFERENCES vcs_repos(id) ON DELETE CASCADE,
			branch_id           TEXT NOT NULL REFERENCES context_vcs_branches(id) ON DELETE CASCADE,
			pr_number           INTEGER NOT NULL,
			title               TEXT NOT NULL DEFAULT '',
			author              TEXT NOT NULL DEFAULT '',
			state               TEXT NOT NULL DEFAULT '',
			is_draft            INTEGER NOT NULL DEFAULT 0,
			review_status       TEXT NOT NULL DEFAULT '',
			url                 TEXT NOT NULL DEFAULT '',
			head_branch         TEXT NOT NULL DEFAULT '',
			mergeable           TEXT NOT NULL DEFAULT '',
			merge_state_status  TEXT NOT NULL DEFAULT '',
			ci_status           TEXT NOT NULL DEFAULT '',
			pr_created_at       TEXT NOT NULL DEFAULT '',
			pr_updated_at       TEXT NOT NULL DEFAULT '',
			fetched_at          TEXT NOT NULL DEFAULT '',
			viewed_at           TEXT,
			ai_processed_at     TEXT,
			UNIQUE (repo_id, pr_number)
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
			my_review_state TEXT NOT NULL DEFAULT '',
			merge_state_status TEXT NOT NULL DEFAULT '',
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
		authored: []vcs.PRData{{
			Number: 5, Title: "mine", Author: "me", State: "open",
			ReviewStatus: "changes_requested", URL: "u", HeadBranch: "b", CreatedAt: "2026-01-01T00:00:00Z",
			Mergeable: "CONFLICTING", CIStatus: "FAILURE",
			Reviewers: []vcs.ReviewerDecision{{Login: "bob", State: "changes_requested"}},
		}},
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

// A branch linked with "Autodetect PR" off must never hit the platform for PRs —
// that's what lets a long-lived branch (main) be tracked without dragging in every
// pull request on it. Links with the flag on still resolve as before.
func TestSyncSkipsPRLookupWhenAutodetectPROff(t *testing.T) {
	runner := &mockRunner{platform: vcs.PlatformGitHub}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "main", "", false)
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/x", "", true)
	require.NoError(t, err)

	_, err = p.Refresh(ctx, "c1")
	require.NoError(t, err)

	// The worker loop now fetches all PRs for the repo once (branch == "") and
	// filters locally by HeadBranch, rather than calling per branch — so the single
	// call reflects the "main" branch link being skipped, not "feat/x" being passed
	// as an argument.
	assert.Equal(t, []string{""}, runner.branchPRCalls)
}

// Two branches tracked in the same repo must share one repo-wide PR fetch: the
// worker loop calls listRepoPRsCached per branch, which caches per owner/name, so
// ListPRsForBranch(branch="") fires once total and each branch recovers its own
// PRs via local HeadBranch filtering.
func TestSyncBatchesPRFetchAcrossBranchesInSameRepo(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		branchPRs: []vcs.PRData{
			{Number: 1, Title: "PR one", Author: "alice", State: "open", HeadBranch: "feat/a", URL: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 2, Title: "PR two", Author: "bob", State: "open", HeadBranch: "feat/b", URL: "u2", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/a", "", true)
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/b", "", true)
	require.NoError(t, err)

	stats, err := p.Refresh(ctx, "c1")
	require.NoError(t, err)

	assert.Equal(t, []string{""}, runner.branchPRCalls, "one fetch-all call must serve both branches")
	assert.Equal(t, 2, stats.ItemsRefreshed, "both branches' PRs must be upserted")

	prs, err := vs.PullRequests.ListPRsForContext(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, prs, 2)
}

// A second sync within the cache TTL must not re-fetch a repo already cached.
func TestSyncReusesCachedPRsWithinTTL(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		branchPRs: []vcs.PRData{
			{Number: 1, Title: "PR one", Author: "alice", State: "open", HeadBranch: "feat/a", URL: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/a", "", true)
	require.NoError(t, err)

	_, err = p.Refresh(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, runner.branchPRCalls, 1)

	_, err = p.Refresh(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, runner.branchPRCalls, 1, "second sync within TTL must not re-fetch")
}

// Once a cache entry's fetchedAt is older than the TTL, the next sync must refetch.
func TestSyncRefetchesPRsAfterCacheEntryExpires(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		branchPRs: []vcs.PRData{
			{Number: 1, Title: "PR one", Author: "alice", State: "open", HeadBranch: "feat/a", URL: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/a", "", true)
	require.NoError(t, err)

	_, err = p.Refresh(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, runner.branchPRCalls, 1)

	// Force the cached entry to look stale — direct map manipulation, no clock
	// injection needed for this test's purposes.
	p.prCacheMu.Lock()
	entry := p.prCache["octo/hello"]
	entry.fetchedAt = time.Now().Add(-prCacheTTL - time.Minute)
	p.prCache["octo/hello"] = entry
	p.prCacheMu.Unlock()

	_, err = p.Refresh(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, runner.branchPRCalls, 2, "an expired cache entry must be refetched")
}

// A branch whose repo has PRs, but none matching its own HeadBranch, must sync
// nothing and report no error — same as the pre-batching behavior.
func TestSyncBranchWithNoMatchingPRsSyncsNothing(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		branchPRs: []vcs.PRData{
			{Number: 1, Title: "PR one", Author: "alice", State: "open", HeadBranch: "other-branch", URL: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(ctx, "c1", repo.ID, "feat/a", "", true)
	require.NoError(t, err)

	stats, err := p.Refresh(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ItemsRefreshed, "no PR matches the tracked branch")

	prs, err := vs.PullRequests.ListPRsForContext(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, prs, 0)
}

func TestSyncReviewRequestsPersistsMyReviewState(t *testing.T) {
	approved := "approved"
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{{
			Number: 7, Title: "t", Author: "carol", State: "open",
			ReviewStatus: "review_required", URL: "u", HeadBranch: "b",
			CreatedAt: "2026-08-01T00:00:00Z", Mergeable: "MERGEABLE",
			MyReviewState: &approved,
		}},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	list, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "approved", list[0].MyReviewState)
}

func TestSyncReviewRequestsTagsChangesRequestedSearchHits(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		// PR 8 appears only in the changes-requested search, with no MyReviewState set.
		changesRequested: []vcs.PRData{{
			Number: 8, Title: "t", Author: "carol", State: "open",
			ReviewStatus: "changes_requested", URL: "u", HeadBranch: "b",
			CreatedAt: "2026-08-01T00:00:00Z", Mergeable: "MERGEABLE",
		}},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	list, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "changes_requested", list[0].MyReviewState)
}

func TestReconcileRecordsMyApprovalAfterPRDropsOffTheList(t *testing.T) {
	runner := &mockRunner{platform: vcs.PlatformGitHub}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	// A stored PR from a previous sync, with no standing decision yet.
	_, err = vs.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 9, "t", "carol",
		"u", "review_required", "b", "2026-08-01T00:00:00Z", "[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)

	// It is gone from both searches, but `gh pr view` shows it open and approved by me.
	runner.prByNumber = map[int]*vcs.PRData{9: {
		Number: 9, State: "open", Mergeable: "MERGEABLE",
		Reviewers: []vcs.ReviewerDecision{{Login: "me", State: "approved"}},
	}}

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	list, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "approved", list[0].MyReviewState)
}

// A merged authored PR observed by the reconcile pass must fire the stat-event
// callback exactly once, with kind pr_merged and the owner/name#number external_id
// convention used elsewhere in this file (see prExtID in sync()).
func TestReconcileRecordsStatEventForMergedAuthoredPR(t *testing.T) {
	runner := &mockRunner{platform: vcs.PlatformGitHub}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	// A stored authored PR from a previous sync that just dropped off the open list.
	_, err = vs.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 42, "Ship it", "me",
		"u", "review_required", "b", "2026-08-01T00:00:00Z", "[]", "MERGEABLE", "authored", "", "")
	require.NoError(t, err)

	// gh returns MergedAt in RFC3339, possibly with a non-UTC offset; the
	// recorded occurredAt must be normalized to UTC.
	runner.prByNumber = map[int]*vcs.PRData{42: {Number: 42, State: "merged", MergedAt: "2026-08-15T10:30:00-07:00"}}

	var calls []struct{ kind, externalID, org, repo, jiraKey, occurredAt string }
	p.WithRecordStatFunc(func(_ context.Context, kind, externalID, org, repoName, jiraKey, occurredAt string) {
		calls = append(calls, struct{ kind, externalID, org, repo, jiraKey, occurredAt string }{
			kind, externalID, org, repoName, jiraKey, occurredAt,
		})
	})

	require.NoError(t, p.SyncAuthoredPRs(ctx, vcs.PlatformGitHub))

	require.Len(t, calls, 1, "merged authored PR must record exactly one stat event")
	assert.Equal(t, "pr_merged", calls[0].kind)
	assert.Equal(t, "acme/widgets#42", calls[0].externalID)
	assert.Equal(t, "", calls[0].org)
	assert.Equal(t, "acme/widgets", calls[0].repo)
	assert.Equal(t, "", calls[0].jiraKey)
	assert.Equal(t, "2026-08-15T17:30:00Z", calls[0].occurredAt,
		"occurredAt must be the real merge time (pr.MergedAt), normalized to RFC3339 UTC, not observed-at")

	// Re-running the sync must not re-invoke the callback: the row is now stored as
	// review_status "merged", so the "already terminal — do not re-fetch" guard in
	// reconcileTerminalStates skips it before GetPRByNumber (and recordStat) run again.
	require.NoError(t, p.SyncAuthoredPRs(ctx, vcs.PlatformGitHub))
	require.Len(t, calls, 1, "re-running sync on an already-terminal row must not double-invoke")
}

// When gh doesn't report a MergedAt (empty/unparseable), recordStat must still fire,
// falling back to observed-at rather than dropping the merge event.
func TestReconcileRecordsStatEventWithObservedAtFallbackWhenMergedAtMissing(t *testing.T) {
	runner := &mockRunner{platform: vcs.PlatformGitHub}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	_, err = vs.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 43, "Ship it too", "me",
		"u", "review_required", "b", "2026-08-01T00:00:00Z", "[]", "MERGEABLE", "authored", "", "")
	require.NoError(t, err)

	runner.prByNumber = map[int]*vcs.PRData{43: {Number: 43, State: "merged"}} // MergedAt left empty

	var calls []string
	// RFC3339 formatting truncates to whole seconds, so give a 1s margin on
	// both sides rather than comparing against sub-second bounds.
	before := time.Now().UTC().Add(-time.Second)
	p.WithRecordStatFunc(func(_ context.Context, _, _, _, _, _, occurredAt string) {
		calls = append(calls, occurredAt)
	})

	require.NoError(t, p.SyncAuthoredPRs(ctx, vcs.PlatformGitHub))
	after := time.Now().UTC().Add(time.Second)

	require.Len(t, calls, 1)
	occurredAt, err := time.Parse(time.RFC3339, calls[0])
	require.NoError(t, err)
	assert.False(t, occurredAt.Before(before) || occurredAt.After(after),
		"fallback occurredAt should be an observed-at timestamp taken during this sync")
}

// Only kind "authored" PRs (the user's own) are merge observations; a PR the user
// merely reviewed must not record a stat event even though it also goes terminal.
func TestReconcileDoesNotRecordStatEventForReviewedPR(t *testing.T) {
	runner := &mockRunner{platform: vcs.PlatformGitHub}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	_, err = vs.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 7, "Someone else's PR", "carol",
		"u", "review_required", "b", "2026-08-01T00:00:00Z", "[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)

	runner.prByNumber = map[int]*vcs.PRData{7: {Number: 7, State: "merged"}}

	var callCount int
	p.WithRecordStatFunc(func(context.Context, string, string, string, string, string, string) {
		callCount++
	})

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))
	assert.Equal(t, 0, callCount, "a reviewed (non-authored) merged PR must not be recorded as the user's merge")
}

func TestMyStateFromReviewers(t *testing.T) {
	ds := []vcs.ReviewerDecision{
		{Login: "alice", State: "approved"},
		{Login: "me", State: "changes_requested"},
	}
	assert.Equal(t, "changes_requested", myStateFromReviewers(ds, "me"))
	assert.Equal(t, "", myStateFromReviewers(ds, "nobody"))
	assert.Equal(t, "", myStateFromReviewers(nil, "me"))
}

// The nil-vs-empty distinction is the load-bearing invariant of this feature: a PR
// whose reviews were not resolved this sync (MyReviewState == nil) must not overwrite
// a previously recorded decision with "". This pins that guard against regression.
func TestSyncReviewRequestsPreservesStoredMyReviewStateWhenUnresolved(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		// PR 11 reappears in review-requested, but the runner did not resolve
		// reviews for it this time (MyReviewState left nil).
		reviewRequests: []vcs.PRData{{
			Number: 11, Title: "t", Author: "carol", State: "open",
			ReviewStatus: "review_required", URL: "u", HeadBranch: "b",
			CreatedAt: "2026-08-01T00:00:00Z", Mergeable: "MERGEABLE",
		}},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)

	// A prior sync already recorded a standing approval for this PR.
	_, err = vs.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 11, "t", "carol",
		"u", "review_required", "b", "2026-08-01T00:00:00Z", "[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)
	require.NoError(t, vs.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, 11, "approved"))

	require.NoError(t, p.SyncReviewRequests(ctx, vcs.PlatformGitHub))

	list, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "approved", list[0].MyReviewState,
		"a PR whose reviews were not resolved this sync must keep its stored decision")
}
