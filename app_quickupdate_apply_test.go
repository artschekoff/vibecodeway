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
}
