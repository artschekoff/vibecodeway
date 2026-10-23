package store

import (
	"context"
	"testing"

	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchContextIDs_JiraKeyAndRepo(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)

	// seed a context
	c, err := s.CreateContext(ctx, "my-feature", org.ID)
	require.NoError(t, err)

	// seed a jira issue with a key and description
	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, c.ID, "ENG-999", "Fix login bug", "todo", "", "bug", "Users cannot log in via SSO", "", "", "")
	require.NoError(t, err, "upsert issue")

	// seed a vcs repo + branch linked to the context
	vcsStore := vcsstore.NewStore(s.DB())
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "auth-service")
	require.NoError(t, err, "ensure repo")

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "Handles all SSO and OAuth flows", "")
	require.NoError(t, err, "update repo meta")

	_, err = vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "main", "", true)
	require.NoError(t, err, "add branch")

	tests := []struct {
		query string
		want  bool // whether c.ID should appear in results
	}{
		{"ENG-999", true},
		{"ENG", true},
		{"SSO", true},          // matches jira description
		{"auth-service", true}, // matches repo name
		{"acme/auth", true},    // matches owner/name
		{"OAuth", true},        // matches repo description
		{"zzznomatch", false},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			ids, err := s.SearchContextIDs(ctx, tt.query)
			require.NoErrorf(t, err, "search %q", tt.query)
			found := false
			for _, id := range ids {
				if id == c.ID {
					found = true
					break
				}
			}
			if found != tt.want {
				t.Errorf("query %q: got found=%v, want %v", tt.query, found, tt.want)
			}
		})
	}
}

func TestSearchContexts_ReturnsOrgAndSummary(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ENG-7", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, c.ID, "ENG-7", "Fix login bug", "todo", "", "bug", "", "", "", "")
	require.NoError(t, err)

	hits, err := s.SearchContexts(ctx, "ENG-7")
	require.NoError(t, err)
	require.NotEmpty(t, hits)

	var hit *ContextSearchHit
	for i := range hits {
		if hits[i].ID == c.ID {
			hit = &hits[i]
			break
		}
	}
	require.NotNil(t, hit, "seeded context should be found")
	assert.Equal(t, "ENG-7", hit.Key)
	assert.Equal(t, org.ID, hit.OrgID)
	assert.Equal(t, "acme", hit.OrgName)
	assert.Equal(t, "Fix login bug", hit.Summary)
}

func TestSearchContextItems_ContentAndBuckets(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "my-feature", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	res, err := jiraRepo.UpsertIssue(ctx, c.ID, "ENG-1", "Login page", "todo", "", "bug", "nothing special here", "", "", "")
	require.NoError(t, err)
	// The searchable term lives only in a comment body -> exercises comment indexing.
	_, err = jiraRepo.UpsertComment(ctx, res.ID, "c1", "alice", "we should add zephyrflux handling", "")
	require.NoError(t, err)

	// A context text whose term lives only in the body (not the name).
	_, err = s.UpsertContextText(ctx, "", c.ID, "Runbook", "deploy via zephyrflux pipeline")
	require.NoError(t, err)

	m, err := s.SearchContextItems(ctx, c.ID, "zephyrflux")
	require.NoError(t, err)

	require.Contains(t, m.IssueIDs, res.ID, "issue should match via its comment body")
	require.Len(t, m.TextIDs, 1, "text should match via its content body")

	// Non-matching query yields empty buckets.
	empty, err := s.SearchContextItems(ctx, c.ID, "zzznomatch")
	require.NoError(t, err)
	require.Empty(t, empty.IssueIDs)
	require.Empty(t, empty.TextIDs)
}

func TestSearchContextIDs_LikeFallback(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)

	c, err := s.CreateContext(ctx, "payments-service", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, c.ID, "PAY-42", "Stripe refund flow", "todo", "", "task", "", "", "", "")
	require.NoError(t, err)

	// "pa" is too short for FTS5 to reliably match — LIKE fallback should catch it
	ids, err := s.SearchContextIDs(ctx, "pa")
	require.NoError(t, err)

	found := false
	for _, id := range ids {
		if id == c.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected 'pa' to match context 'payments-service' via LIKE fallback")
	}
}

// TestSearchIndexesPRContent locks in that PR titles and PR comment bodies are
// indexed into the FTS table, and that both map to the PR's id under the
// PRIDs bucket (a comment match maps to its owning PR, which is what the
// in-context filter uses).
func TestSearchIndexesPRContent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "my-feature", org.ID)
	require.NoError(t, err)

	vcsStore := vcsstore.NewStore(s.DB())
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "auth-service")
	require.NoError(t, err, "ensure repo")

	branch, err := vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "feature/zeta", "", true)
	require.NoError(t, err, "add branch")

	prResult, err := vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 1,
		"Zebrafish auth refactor", "alice", "open", "", "", "", "", "", "", false, "", "")
	require.NoError(t, err, "upsert pr")
	prID := prResult.ID

	_, _, err = vcsStore.PullRequests.UpsertComment(ctx, prID, "ext-1", nil,
		"conversation", "bob", "quokka edge case", nil, nil, false, nil, "", "")
	require.NoError(t, err, "upsert pr comment")

	// Global: a PR-title token surfaces the context.
	ids, err := s.SearchContextIDs(ctx, "Zebrafish")
	require.NoError(t, err)
	assert.Contains(t, ids, c.ID)

	// In-context: the PR id is bucketed under PRIDs for both title and comment tokens.
	m, err := s.SearchContextItems(ctx, c.ID, "Zebrafish")
	require.NoError(t, err)
	assert.Contains(t, m.PRIDs, prID)

	m2, err := s.SearchContextItems(ctx, c.ID, "quokka")
	require.NoError(t, err)
	assert.Contains(t, m2.PRIDs, prID)
}

// A multi-token query where the tokens live in DIFFERENT contexts must still
// find each of them: AND-only FTS returned zero (the agent's "JH-1 JH-2 JH-3"
// lookups always came back empty), so zero AND-hits fall back to OR.
func TestSearchContextIDs_MultiTokenFallsBackToOr(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)

	c1, err := s.CreateContext(ctx, "feature-one", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "feature-two", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, c1.ID, "JH-6223", "Opportunities tab", "todo", "", "task", "", "", "", "")
	require.NoError(t, err)
	_, err = jiraRepo.UpsertIssue(ctx, c2.ID, "JH-5612", "Metric admin", "todo", "", "task", "", "", "", "")
	require.NoError(t, err)

	ids, err := s.SearchContextIDs(ctx, "JH-6223 JH-5612")
	require.NoError(t, err)
	assert.Contains(t, ids, c1.ID, "OR fallback must find the JH-6223 context")
	assert.Contains(t, ids, c2.ID, "OR fallback must find the JH-5612 context")
}
