package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListContextIDsByJiraStatus(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	closed, err := s.CreateContext(ctx, "ctx-closed", org.ID)
	require.NoError(t, err)
	open, err := s.CreateContext(ctx, "ctx-open", org.ID)
	require.NoError(t, err)

	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES ('ji-1', ?, 'JH-1', 's', 'Closed', '', datetime('now')),
		        ('ji-2', ?, 'JH-2', 's', 'In Progress', '', datetime('now'))`, closed.ID, open.ID)
	require.NoError(t, err)

	// Case-insensitive match returns only the closed context.
	ids, err := s.ListContextIDsByJiraStatus(ctx, "closed")
	require.NoError(t, err)
	assert.Equal(t, []string{closed.ID}, ids)

	// No match → empty.
	ids, err = s.ListContextIDsByJiraStatus(ctx, "Done")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestArchiveContexts(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	a, err := s.CreateContext(ctx, "a", org.ID)
	require.NoError(t, err)
	b, err := s.CreateContext(ctx, "b", org.ID)
	require.NoError(t, err)

	n, err := s.ArchiveContexts(ctx, []string{a.ID, b.ID})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	got, err := s.GetContext(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", got.Status)

	// Idempotent: archiving already-archived contexts changes nothing.
	n, err = s.ArchiveContexts(ctx, []string{a.ID, b.ID})
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// Empty input is a no-op.
	n, err = s.ArchiveContexts(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
