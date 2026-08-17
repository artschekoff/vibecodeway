package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTodoItems_ResolvesJiraContext(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "JH-5127", org.ID)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES ('ji-1', ?, 'JH-5127', 's', 'To Do', '', datetime('now'))`, c.ID)
	require.NoError(t, err)

	require.NoError(t, s.ReplaceTodoItems(ctx, []AITodoItem{
		{Kind: "implement", Title: "Implement JH-5127", RefType: "jira", RefKey: "JH-5127", Priority: "high", Position: 0, Source: "ai"},
		{Kind: "make_pr", Title: "PR work", RefType: "pr", RefKey: "o/r#1", Priority: "low", Position: 1, Source: "ai"},
	}))

	items, err := s.ListTodoItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, 2)
	// The jira ref resolves to its in-app context…
	assert.Equal(t, c.ID, items[0].ContextID)
	assert.Equal(t, "JH-5127", items[0].ContextKey)
	// …a pr ref (or any unlinked ref) stays empty.
	assert.Equal(t, "", items[1].ContextID)
	assert.Equal(t, "", items[1].ContextKey)
}

func TestListTodoItems_ResolvesPRContext(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "JH-3051", org.ID)
	require.NoError(t, err)

	// A PR is opened on a branch attached to the context:
	// vcs_pull_requests -> context_vcs_branches -> contexts.
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO vcs_repos (id, platform, owner, name, created_at)
		 VALUES ('repo-1', 'github', 'juna-health', 'juna-api', datetime('now'))`)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, created_at)
		 VALUES ('br-1', ?, 'repo-1', 'feature/jh-3051', datetime('now'))`, c.ID)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO vcs_pull_requests (id, repo_id, branch_id, pr_number, head_branch, fetched_at)
		 VALUES ('pr-1', 'repo-1', 'br-1', 247, 'feature/jh-3051', datetime('now'))`)
	require.NoError(t, err)

	require.NoError(t, s.ReplaceTodoItems(ctx, []AITodoItem{
		// Bare "repoName#num" form (what the board data produces today).
		{Kind: "rerequest_pr", Title: "Rerequest review for PR JH-3051", RefType: "pr", RefKey: "juna-api#247", Priority: "high", Position: 0, Source: "ai"},
		// Owner-qualified "owner/repoName#num" form (prompt example).
		{Kind: "rerequest_pr", Title: "Rerequest review for PR JH-3051", RefType: "pr", RefKey: "juna-health/juna-api#247", Priority: "high", Position: 1, Source: "ai"},
		// A pr ref with no matching PR row stays empty.
		{Kind: "make_pr", Title: "PR work", RefType: "pr", RefKey: "other#9", Priority: "low", Position: 2, Source: "ai"},
	}))

	items, err := s.ListTodoItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	// Both ref_key forms resolve to the PR's context.
	assert.Equal(t, c.ID, items[0].ContextID)
	assert.Equal(t, "JH-3051", items[0].ContextKey)
	assert.Equal(t, c.ID, items[1].ContextID)
	assert.Equal(t, "JH-3051", items[1].ContextKey)
	// An unmatched pr ref stays empty.
	assert.Equal(t, "", items[2].ContextID)
	assert.Equal(t, "", items[2].ContextKey)
}

func newTodoTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestTodoItemsReplaceAndList(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()

	err := s.ReplaceTodoItems(ctx, []AITodoItem{
		{Kind: "implement", Title: "Do JH-1", Priority: "high", Position: 0, Source: "ai"},
		{Kind: "rerequest_pr", Title: "Ping reviewers on #42", Priority: "medium", Position: 1, Source: "ai"},
	})
	if err != nil {
		t.Fatalf("ReplaceTodoItems: %v", err)
	}

	items, err := s.ListTodoItems(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListTodoItems = %d items, %v", len(items), err)
	}
	if items[0].Position != 0 || items[1].Position != 1 || items[0].ID == "" {
		t.Fatalf("items not ordered/ID'd: %+v", items)
	}
}

func TestTodoItemsReplacePreservesUser(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()

	_, err := s.AddTodoItem(ctx, AITodoItem{Kind: "implement", Title: "user item", Position: 5, Source: "user"})
	require.NoError(t, err)
	// Replacing AI items must leave the user item intact.
	require.NoError(t, s.ReplaceTodoItems(ctx, []AITodoItem{
		{Kind: "implement", Title: "ai item", Position: 0, Source: "ai"},
	}))

	items, _ := s.ListTodoItems(ctx)
	var haveUser, haveAI bool
	for _, it := range items {
		if it.Source == "user" {
			haveUser = true
		}
		if it.Source == "ai" {
			haveAI = true
		}
	}
	if !haveUser || !haveAI {
		t.Fatalf("expected both user and ai items, got %+v", items)
	}
}

func TestTodoItemsReorderAndDelete(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	_ = s.ReplaceTodoItems(ctx, []AITodoItem{
		{Kind: "implement", Title: "A", Position: 0, Source: "ai"},
		{Kind: "implement", Title: "B", Position: 1, Source: "ai"},
	})
	items, _ := s.ListTodoItems(ctx)
	a, b := items[0], items[1]

	if err := s.ReorderTodoItems(ctx, []string{b.ID, a.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	got, _ := s.ListTodoItems(ctx)
	if got[0].ID != b.ID {
		t.Fatalf("reorder did not put B first: %+v", got)
	}

	if err := s.DeleteTodoItem(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = s.ListTodoItems(ctx)
	if len(got) != 1 || got[0].ID != b.ID {
		t.Fatalf("delete failed: %+v", got)
	}
}
