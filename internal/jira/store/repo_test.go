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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "jira-ctx", org.ID)
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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "jira-ctx", org.ID)
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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "del-ctx", org.ID)
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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	res, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Fix login", "Open", "alice", "Bug", "desc", "", "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "TEST-1", issues[0].JiraKey)
	assert.Equal(t, "Fix login", issues[0].Summary)
}

func TestUpsertIssueUpdatesOnDuplicate(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	r1, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "original", "Open", "alice", "Bug", "", "", "", "")
	require.NoError(t, err)

	r2, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "updated", "Closed", "bob", "Story", "new desc", "2024-06-01T12:00:00Z", "", "")
	require.NoError(t, err)
	assert.Equal(t, r1.ID, r2.ID, "same ID on duplicate upsert")

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "updated", issues[0].Summary)
	assert.Equal(t, "2024-06-01T12:00:00Z", issues[0].JiraUpdated)
}

func TestUpsertIssuePersistsPriority(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertIssue(ctx, c.ID, "ABC-1", "sum", "Open", "alice", "Task", "desc", "2026-01-01", "", "High")
	require.NoError(t, err)

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "High", issues[0].Priority)
}

// --- Comment CRUD ---

func TestUpsertAndGetComments(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	issueRes, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "alice", "Bug", "", "", "", "")
	require.NoError(t, err)
	issueID := issueRes.ID

	commentID, err := repo.UpsertComment(ctx, issueID, "jira-cmt-1", "bob", "looks good", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.NotEmpty(t, commentID)

	commentID2, err := repo.UpsertComment(ctx, issueID, "jira-cmt-1", "bob", "looks good", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.Empty(t, commentID2, "duplicate should not be inserted")

	comments, err := repo.GetCommentsByIssue(ctx, issueID, false)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "jira-cmt-1", comments[0].JiraID)
}

// --- IsActive toggle ---

func TestIssueActiveByDefault(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	_, err = repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)

	issues, err := repo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.True(t, issues[0].IsActive, "new issue should be active by default")
}

func TestToggleIssueActive(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	toggleRes, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)

	issue, err := repo.ToggleIssueActive(ctx, toggleRes.ID)
	require.NoError(t, err)
	assert.False(t, issue.IsActive)

	issue, err = repo.ToggleIssueActive(ctx, toggleRes.ID)
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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	commentActiveRes, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)
	issueID := commentActiveRes.ID
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

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	toggleCommentRes, err := repo.UpsertIssue(ctx, c.ID, "TEST-1", "Summary", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)
	issueID := toggleCommentRes.ID
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

func TestGetCommentsByIssues(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx1", org.ID)
	require.NoError(t, err)

	res1, err := repo.UpsertIssue(ctx, c.ID, "PROJ-1", "issue 1", "open", "", "task", "", "2024-01-01T00:00:00Z", "", "")
	require.NoError(t, err)
	issue1 := res1.ID
	res2, err := repo.UpsertIssue(ctx, c.ID, "PROJ-2", "issue 2", "open", "", "task", "", "2024-01-01T00:00:00Z", "", "")
	require.NoError(t, err)
	issue2 := res2.ID

	_, err = repo.UpsertComment(ctx, issue1, "jira-1", "alice", "first comment", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	_, err = repo.UpsertComment(ctx, issue1, "jira-2", "bob", "second comment", "2024-01-02T00:00:00Z")
	require.NoError(t, err)
	_, err = repo.UpsertComment(ctx, issue2, "jira-3", "carol", "other issue comment", "2024-01-01T00:00:00Z")
	require.NoError(t, err)

	got, err := repo.GetCommentsByIssues(ctx, []string{issue1, issue2}, false)
	require.NoError(t, err)

	assert.Len(t, got[issue1], 2)
	assert.Len(t, got[issue2], 1)
	assert.Equal(t, "first comment", got[issue1][0].Body)
	assert.Equal(t, "other issue comment", got[issue2][0].Body)

	// Empty input returns empty map, not an error.
	empty, err := repo.GetCommentsByIssues(ctx, nil, false)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestFindContextsByJiraKeys(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	// Create two contexts and insert issues in each.
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "ctx-one", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "ctx-two", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertIssue(ctx, c1.ID, "ALPHA-1", "Alpha one", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)
	_, err = repo.UpsertIssue(ctx, c1.ID, "ALPHA-2", "Alpha two", "Open", "", "Bug", "", "", "", "")
	require.NoError(t, err)
	_, err = repo.UpsertIssue(ctx, c2.ID, "BETA-1", "Beta one", "Open", "", "Story", "", "", "", "")
	require.NoError(t, err)

	t.Run("finds keys across contexts", func(t *testing.T) {
		got, err := repo.FindContextsByJiraKeys(ctx, []string{"ALPHA-1", "BETA-1", "MISSING-9"})
		require.NoError(t, err)
		assert.Equal(t, []string{c1.ID}, got["ALPHA-1"])
		assert.Equal(t, []string{c2.ID}, got["BETA-1"])
		assert.NotContains(t, got, "MISSING-9", "absent keys must not appear")
	})

	t.Run("same key in multiple contexts returns all context IDs", func(t *testing.T) {
		// SHARED-1 exists in both c1 and c2.
		_, err := repo.UpsertIssue(ctx, c1.ID, "SHARED-1", "Shared issue", "Open", "", "Bug", "", "", "", "")
		require.NoError(t, err)
		_, err = repo.UpsertIssue(ctx, c2.ID, "SHARED-1", "Shared issue", "Open", "", "Bug", "", "", "", "")
		require.NoError(t, err)

		got, err := repo.FindContextsByJiraKeys(ctx, []string{"SHARED-1"})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{c1.ID, c2.ID}, got["SHARED-1"])
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		got, err := repo.FindContextsByJiraKeys(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("all missing returns empty map", func(t *testing.T) {
		got, err := repo.FindContextsByJiraKeys(ctx, []string{"NO-1", "NO-2"})
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestUpsertIssue_StatusChange(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx", org.ID)
	require.NoError(t, err)

	// Insert initial issue with status "In Progress"
	res1, err := repo.UpsertIssue(ctx, c.ID,
		"PROJ-1", "Summary", "In Progress", "alice", "Story", "desc", "2024-01-01", "", "")
	require.NoError(t, err)
	assert.True(t, res1.IsNew)
	assert.False(t, res1.StatusChanged)
	assert.NotEmpty(t, res1.ID)

	// Update with same status — no change
	res2, err := repo.UpsertIssue(ctx, c.ID,
		"PROJ-1", "Summary", "In Progress", "alice", "Story", "desc", "2024-01-01", "", "")
	require.NoError(t, err)
	assert.False(t, res2.IsNew)
	assert.False(t, res2.StatusChanged)
	assert.Equal(t, res1.ID, res2.ID)

	// Update with different status — change detected
	res3, err := repo.UpsertIssue(ctx, c.ID,
		"PROJ-1", "Summary", "Done", "alice", "Story", "desc", "2024-01-02", "", "")
	require.NoError(t, err)
	assert.False(t, res3.IsNew)
	assert.True(t, res3.StatusChanged)
	assert.Equal(t, "In Progress", res3.OldStatus)
	assert.Equal(t, res1.ID, res3.ID)
}
