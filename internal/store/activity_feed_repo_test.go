package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRecentActivity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "PROJ-2", org.ID)
	require.NoError(t, err)

	// Seed notifications directly with explicit created_at so ordering is deterministic.
	seed := func(id, ctxID, entityType, eventType, created string) {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO notifications (id, context_id, entity_type, entity_id, event_type, old_value, new_value, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, id, ctxID, entityType, id, eventType, "", "", created)
		require.NoError(t, err)
	}
	seed("n-old", c1.ID, "jira_issue", "jira_status_changed", "2026-07-20T09:00:00Z")
	seed("n-new", c2.ID, "pull_request", "pr_merged", "2026-07-20T10:00:00Z")

	t.Run("returns newest first with context key joined", func(t *testing.T) {
		items, err := s.ListRecentActivity(ctx, 0)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "n-new", items[0].ID)
		assert.Equal(t, "PROJ-2", items[0].ContextKey)
		assert.Equal(t, "pull_request", items[0].EntityType)
		assert.Equal(t, "pr_merged", items[0].EventType)
		assert.Equal(t, "n-old", items[1].ID)
		assert.Equal(t, "PROJ-1", items[1].ContextKey)
	})

	t.Run("respects the limit", func(t *testing.T) {
		items, err := s.ListRecentActivity(ctx, 1)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "n-new", items[0].ID)
	})

	t.Run("empty store returns non-nil empty slice", func(t *testing.T) {
		empty := openTestStore(t)
		items, err := empty.ListRecentActivity(ctx, 10)
		require.NoError(t, err)
		assert.NotNil(t, items)
		assert.Len(t, items, 0)
	})
}
