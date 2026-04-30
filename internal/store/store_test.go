package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err, "open store")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// --- Schema ---

func TestContextsTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "contexts")
}

func TestJiraTablesExist(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "jira_provider")
	assertTableExists(t, s, "jira_issues")
	assertTableExists(t, s, "jira_comments")
}

func TestSlackTablesExist(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "slack_provider")
	assertTableExists(t, s, "slack_threads")
}

func TestContextRelationsTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "context_relations")
}

func TestOldUnprefixedTablesAbsent(t *testing.T) {
	s := openTestStore(t)
	assertTableAbsent(t, s, "issues")
	assertTableAbsent(t, s, "comments")
}

func assertTableExists(t *testing.T, s *Store, name string) {
	t.Helper()
	var count int
	err := s.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "table %q should exist", name)
}

func assertTableAbsent(t *testing.T, s *Store, name string) {
	t.Helper()
	var count int
	err := s.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "table %q must not exist", name)
}

// --- Context CRUD ---

func TestCreateAndGetContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "sprint-1")
	require.NoError(t, err)
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "sprint-1", c.Key)
	assert.Equal(t, "active", c.Status)

	got, err := s.GetContext(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
	assert.Equal(t, c.Key, got.Key)
}

func TestGetContextByKey(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "my-ctx")
	require.NoError(t, err)

	got, err := s.GetContextByKey(ctx, "my-ctx")
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
}

func TestGetContextNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.GetContext(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListContexts_DefaultExcludesArchived(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.CreateContext(ctx, "active-ctx")
	require.NoError(t, err)

	archived, err := s.CreateContext(ctx, "archived-ctx")
	require.NoError(t, err)
	_, err = s.UpdateContext(ctx, archived.ID, archived.Key, "archived")
	require.NoError(t, err)

	list, err := s.ListContexts(ctx, false)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "active-ctx", list[0].Key)
}

func TestListContexts_IncludeArchivedReturnsAll(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.CreateContext(ctx, "active-ctx")
	require.NoError(t, err)

	archived, err := s.CreateContext(ctx, "archived-ctx")
	require.NoError(t, err)
	_, err = s.UpdateContext(ctx, archived.ID, archived.Key, "archived")
	require.NoError(t, err)

	list, err := s.ListContexts(ctx, true)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestToggleContextStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "toggle-ctx")
	require.NoError(t, err)
	assert.Equal(t, "active", c.Status)

	updated, err := s.ToggleContextStatus(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", updated.Status)

	updated, err = s.ToggleContextStatus(ctx, updated.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", updated.Status)
}

func TestToggleContextStatus_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ToggleContextStatus(context.Background(), "non-existent")
	assert.ErrorIs(t, err, ErrNotFound)
}
