package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyContextUpdateActions(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx}

	t.Run("applies add_event and set_status successfully", func(t *testing.T) {
		actions := []appstore.ContextUpdateAction{
			{Action: "add_event", EventType: "work_completed", Summary: "done", Description: "logged completion"},
			{Action: "set_status", StatusSlug: "in_progress", StatusComment: "x", Description: "moved to in progress"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.Equal(t, "add_event", results[0].Action)
		assert.True(t, results[0].OK, results[0].Message)

		assert.Equal(t, "set_status", results[1].Action)
		assert.True(t, results[1].OK, results[1].Message)

		events, err := s.ListEvents(ctx, c1.ID)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "work_completed", events[0].Type)
		assert.Equal(t, "done", events[0].Summary)

		snap, err := s.GetSnapshot(ctx, c1.ID)
		require.NoError(t, err)
		require.NotNil(t, snap)
		assert.Equal(t, "in_progress", snap.Status)
	})

	t.Run("unknown action reports OK false without aborting batch", func(t *testing.T) {
		actions := []appstore.ContextUpdateAction{
			{Action: "nonsense"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].OK)
		assert.NotEmpty(t, results[0].Message)
	})

	t.Run("add_relation rejects off-vocabulary relationType without persisting", func(t *testing.T) {
		c2, err := s.CreateContext(ctx, "PROJ-2", org.ID)
		require.NoError(t, err)

		actions := []appstore.ContextUpdateAction{
			{Action: "add_relation", TargetContextKey: c2.Key, RelationType: "blocked_by", Description: "bad vocab"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].OK)
		assert.NotEmpty(t, results[0].Message)

		relations, err := s.ListContextRelations(ctx, c1.ID)
		require.NoError(t, err)
		assert.Empty(t, relations, "off-vocabulary relation must not be persisted")
	})

	t.Run("add_relation accepts canonical relationType", func(t *testing.T) {
		c3, err := s.CreateContext(ctx, "PROJ-3", org.ID)
		require.NoError(t, err)

		actions := []appstore.ContextUpdateAction{
			{Action: "add_relation", TargetContextKey: c3.Key, RelationType: "is-blocked-by", Description: "good vocab"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].OK, results[0].Message)

		relations, err := s.ListContextRelations(ctx, c1.ID)
		require.NoError(t, err)
		require.Len(t, relations, 1)
		assert.Equal(t, "is-blocked-by", relations[0].RelationType)
	})

	t.Run("add_event rejects unknown eventType", func(t *testing.T) {
		actions := []appstore.ContextUpdateAction{
			{Action: "add_event", EventType: "not_a_real_type", Summary: "bogus", Description: "bad event type"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].OK)
		assert.NotEmpty(t, results[0].Message)
	})

	t.Run("add_repo rejects missing branch", func(t *testing.T) {
		actions := []appstore.ContextUpdateAction{
			{Action: "add_repo", Platform: "github", OwnerRepo: "acme/widgets", Branch: "", Description: "missing branch"},
		}
		results, err := a.ApplyContextUpdateActions(c1.ID, actions)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].OK)
		assert.NotEmpty(t, results[0].Message)
	})
}
