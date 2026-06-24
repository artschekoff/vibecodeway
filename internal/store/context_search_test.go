package store

import (
	"context"
	"testing"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
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
	_, err = jiraRepo.UpsertIssue(ctx, c.ID, "ENG-999", "Fix login bug", "todo", "", "bug", "Users cannot log in via SSO", "", "")
	require.NoError(t, err, "upsert issue")

	// seed a vcs repo + branch linked to the context
	vcsStore := vcsstore.NewStore(s.DB())
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "auth-service")
	require.NoError(t, err, "ensure repo")

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "Handles all SSO and OAuth flows", "")
	require.NoError(t, err, "update repo meta")

	_, err = vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "main", "")
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

func TestSearchContextIDs_LikeFallback(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)

	c, err := s.CreateContext(ctx, "payments-service", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, c.ID, "PAY-42", "Stripe refund flow", "todo", "", "task", "", "", "")
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
