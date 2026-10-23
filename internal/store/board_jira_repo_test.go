package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBJAStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestListBoardJiraAssignments_hidesTerminalStatuses(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()

	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "open", Status: "In Progress", IssueType: "Task", Priority: "High", Created: "2026-07-20T10:00:00Z"},
		{Key: "JH-2", Summary: "done", Status: "Done", IssueType: "Story", Priority: "Highest", Created: "2026-07-21T10:00:00Z"},
		{Key: "JH-3", Summary: "closed", Status: "Closed", IssueType: "Bug", Priority: "Low", Created: "2026-07-22T10:00:00Z"},
	}))
	list, err := s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	require.Len(t, list, 1, "terminal tickets are finished work and stay off the board")
	assert.Equal(t, "JH-1", list[0].Key)
}

func TestBoardJiraAssignments_UpsertPreservesSeenAndReplaces(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()

	// Empty to start.
	list, err := s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	assert.Empty(t, list)

	// First fetch: two issues, both new (unseen).
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Open", IssueType: "Task", Priority: "High", Created: "2026-07-20T10:00:00Z"},
		{Key: "JH-2", Summary: "two", Status: "Open", IssueType: "Bug", Priority: "Low", Created: "2026-07-21T10:00:00Z"},
	}))
	list, err = s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	require.Len(t, list, 2)
	for _, it := range list {
		assert.False(t, it.Seen)
		assert.NotEmpty(t, it.FetchedAt)
	}

	// Mark all seen.
	require.NoError(t, s.MarkBoardJiraAssignmentsSeen(ctx))

	// Second fetch: JH-1 stays (updated summary), JH-2 dropped, JH-3 added (new).
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "one-updated", Status: "In Progress", IssueType: "Task", Priority: "High", Created: "2026-07-20T10:00:00Z"},
		{Key: "JH-3", Summary: "three", Status: "Open", IssueType: "Task", Priority: "Medium", Created: "2026-07-22T10:00:00Z"},
	}))
	list, err = s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	require.Len(t, list, 2) // JH-2 removed

	byKey := map[string]BoardJiraAssignment{}
	for _, it := range list {
		byKey[it.Key] = it
	}
	require.Contains(t, byKey, "JH-1")
	require.Contains(t, byKey, "JH-3")
	assert.Equal(t, "one-updated", byKey["JH-1"].Summary)
	assert.True(t, byKey["JH-1"].Seen)  // preserved across upsert
	assert.False(t, byKey["JH-3"].Seen) // brand new
}

func TestGetJiraKeyByID(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx", org.ID)
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES ('ji-1', ?, 'JH-4926', 's', 'Ready for QA', '', datetime('now'))`, c.ID)
	require.NoError(t, err)

	key, err := s.GetJiraKeyByID(ctx, "ji-1")
	require.NoError(t, err)
	assert.Equal(t, "JH-4926", key)

	// Unknown id → empty string, no error (label just falls back to the generic reason).
	key, err = s.GetJiraKeyByID(ctx, "does-not-exist")
	require.NoError(t, err)
	assert.Equal(t, "", key)
}

func TestBoardJiraAssignments_OrgRoundTrips(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()

	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Open", Priority: "High", Created: "2026-07-20T10:00:00Z", Org: "acme-api"},
		{Key: "JH-2", Summary: "two", Status: "Open", Priority: "Low", Created: "2026-07-21T10:00:00Z", Org: ""},
	}))

	list, err := s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	byKey := map[string]BoardJiraAssignment{}
	for _, it := range list {
		byKey[it.Key] = it
	}
	assert.Equal(t, "acme-api", byKey["JH-1"].Org)
	assert.Equal(t, "", byKey["JH-2"].Org)

	// Re-upsert updates the org (issue moved scope).
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Open", Priority: "High", Created: "2026-07-20T10:00:00Z", Org: "rdbms"},
	}))
	list, err = s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "rdbms", list[0].Org)
}

func TestBoardJiraAssignments_Sorting(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "low-old", Priority: "Low", Created: "2026-07-01T10:00:00Z"},
		{Key: "JH-2", Summary: "highest-new", Priority: "Highest", Created: "2026-07-25T10:00:00Z"},
		{Key: "JH-3", Summary: "medium-mid", Priority: "Medium", Created: "2026-07-10T10:00:00Z"},
	}))

	byPriority, err := s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	assert.Equal(t, []string{"JH-2", "JH-3", "JH-1"}, keysOf(byPriority)) // Highest, Medium, Low

	byCreated, err := s.ListBoardJiraAssignments(ctx, "created")
	require.NoError(t, err)
	assert.Equal(t, []string{"JH-2", "JH-3", "JH-1"}, keysOf(byCreated)) // newest created first
}

func keysOf(items []BoardJiraAssignment) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Key)
	}
	return out
}

// The org custom field can't come back from a Jira search (acli allowlist), so
// it's backfilled per key afterwards. These two calls drive that loop.
func TestBoardJiraAssignments_OrgBackfill(t *testing.T) {
	s := newBJAStore(t)
	ctx := t.Context()

	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Open", Org: "acme-api"},
		{Key: "JH-2", Summary: "two", Status: "Open"},
		{Key: "JH-3", Summary: "three", Status: "Open"},
	}))

	missing, err := s.ListBoardKeysMissingOrg(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"JH-2", "JH-3"}, missing)

	require.NoError(t, s.SetBoardAssignmentOrg(ctx, "JH-2", "rdbms"))
	// A key whose org is genuinely unset must drop out of the queue too, otherwise
	// the backfill re-fetches it on every refresh forever.
	require.NoError(t, s.SetBoardAssignmentOrg(ctx, "JH-3", ""))

	missing, err = s.ListBoardKeysMissingOrg(ctx)
	require.NoError(t, err)
	assert.Empty(t, missing)

	byKey := map[string]BoardJiraAssignment{}
	list, err := s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	for _, it := range list {
		byKey[it.Key] = it
	}
	assert.Equal(t, "rdbms", byKey["JH-2"].Org)
	assert.Equal(t, "acme-api", byKey["JH-1"].Org)

	// A later refresh (search can't return org -> empty) must not undo the backfill.
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-2", Summary: "two", Status: "Open"},
	}))
	list, err = s.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "rdbms", list[0].Org)
}
