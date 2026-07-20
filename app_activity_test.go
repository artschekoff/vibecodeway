package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActivityFeed(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)
	require.NoError(t, s.CreateNotification(ctx, c1.ID, "pull_request", "pr-1", "pr_merged", "", "merged"))
	require.NoError(t, s.CreateNotification(ctx, c1.ID, "jira_issue", "j-1", "jira_status_changed", "To Do", "Done"))

	a := &App{db: s, ctx: ctx}

	t.Run("limit <= 0 returns all recent items with context key", func(t *testing.T) {
		items, err := a.GetActivityFeed(0)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "PROJ-1", items[0].ContextKey)
	})

	t.Run("limit caps the result", func(t *testing.T) {
		items, err := a.GetActivityFeed(1)
		require.NoError(t, err)
		assert.Len(t, items, 1)
	})
}
