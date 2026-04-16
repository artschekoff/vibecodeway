package jirastore_test

import (
	"context"
	"path/filepath"
	"testing"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) (*appstore.Store, *jirastore.Repo) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, jirastore.NewRepo(s.DB())
}

// --- Jira provider ---

func TestUpsertAndGetJiraProvider(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "jira-ctx")
	require.NoError(t, err)

	jp, err := repo.UpsertJiraProvider(ctx, c.ID, "project = TEAM")
	require.NoError(t, err)
	assert.NotEmpty(t, jp.ID)
	assert.Equal(t, c.ID, jp.ContextID)
	assert.Equal(t, "project = TEAM", jp.SearchQuery)

	got, err := repo.GetJiraProvider(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, jp.ID, got.ID)
	assert.Equal(t, "project = TEAM", got.SearchQuery)
}

func TestUpsertJiraProviderUpdatesOnDuplicate(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "jira-ctx")
	require.NoError(t, err)

	jp1, err := repo.UpsertJiraProvider(ctx, c.ID, "project = OLD")
	require.NoError(t, err)

	jp2, err := repo.UpsertJiraProvider(ctx, c.ID, "project = NEW")
	require.NoError(t, err)
	assert.Equal(t, jp1.ID, jp2.ID, "same row on duplicate upsert")
	assert.Equal(t, "project = NEW", jp2.SearchQuery)
}

func TestGetJiraProviderNotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.GetJiraProvider(context.Background(), "no-context")
	assert.ErrorIs(t, err, jirastore.ErrNotFound)
}

func TestDeleteContextCascadesToJiraProvider(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "del-ctx")
	require.NoError(t, err)

	_, err = repo.UpsertJiraProvider(ctx, c.ID, "project = DEL")
	require.NoError(t, err)

	require.NoError(t, s.DeleteContext(ctx, c.ID))

	_, err = repo.GetJiraProvider(ctx, c.ID)
	assert.ErrorIs(t, err, jirastore.ErrNotFound, "jira_provider should cascade-delete")
}

// --- Issue CRUD ---

func TestUpsertAndGetIssues(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	issueID, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Fix login", "Open", "alice", "Bug", "desc", "")
	require.NoError(t, err)
	assert.NotEmpty(t, issueID)

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "TEST-1", issues[0].JiraKey)
	assert.Equal(t, "Fix login", issues[0].Summary)
}

func TestUpsertIssueUpdatesOnDuplicate(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	id1, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "original", "Open", "alice", "Bug", "", "")
	require.NoError(t, err)

	id2, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "updated", "Closed", "bob", "Story", "new desc", "2024-06-01T12:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "same ID on duplicate upsert")

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "updated", issues[0].Summary)
	assert.Equal(t, "2024-06-01T12:00:00Z", issues[0].JiraUpdated)
}

// --- Comment CRUD ---

func TestUpsertAndGetComments(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)
	issueID, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "alice", "Bug", "", "")
	require.NoError(t, err)

	inserted, err := repo.UpsertComment(ctx, issueID, "jira-cmt-1", "bob", "looks good", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.True(t, inserted)

	inserted2, err := repo.UpsertComment(ctx, issueID, "jira-cmt-1", "bob", "looks good", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.False(t, inserted2, "duplicate should not be inserted")

	comments, err := repo.GetCommentsByIssue(ctx, issueID, false)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "jira-cmt-1", comments[0].JiraID)
}

// --- IsActive toggle ---

func TestIssueActiveByDefault(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)
	_, err = repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "")
	require.NoError(t, err)

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.True(t, issues[0].IsActive, "new issue should be active by default")
}

func TestToggleIssueActive(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)
	id, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "")
	require.NoError(t, err)

	issue, err := repo.ToggleIssueActive(ctx, id)
	require.NoError(t, err)
	assert.False(t, issue.IsActive)

	issue, err = repo.ToggleIssueActive(ctx, id)
	require.NoError(t, err)
	assert.True(t, issue.IsActive)
}

func TestToggleIssueActive_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.ToggleIssueActive(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, jirastore.ErrNotFound)
}

func TestCommentActiveByDefault(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)
	issueID, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "")
	require.NoError(t, err)
	_, err = repo.UpsertComment(ctx, issueID, "cmt-1", "alice", "body", "2024-01-01T00:00:00Z")
	require.NoError(t, err)

	comments, err := repo.GetCommentsByIssue(ctx, issueID, false)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.True(t, comments[0].IsActive, "new comment should be active by default")
}

func TestToggleCommentActive(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)
	issueID, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "")
	require.NoError(t, err)
	_, err = repo.UpsertComment(ctx, issueID, "cmt-1", "alice", "body", "2024-01-01T00:00:00Z")
	require.NoError(t, err)

	comments, err := repo.GetCommentsByIssue(ctx, issueID, false)
	require.NoError(t, err)
	commentID := comments[0].ID

	toggled, err := repo.ToggleCommentActive(ctx, commentID)
	require.NoError(t, err)
	assert.False(t, toggled.IsActive)

	toggled, err = repo.ToggleCommentActive(ctx, commentID)
	require.NoError(t, err)
	assert.True(t, toggled.IsActive)
}

func TestToggleCommentActive_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.ToggleCommentActive(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, jirastore.ErrNotFound)
}
