package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnVCSNotifyEmitsPREventOnce(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}

	// First detection: emits exactly one pr_review_changed event.
	require.NoError(t, a.onVCSNotify(ctx, c.ID, "pull_request", "pr-1", "pr_review_changed", "", "APPROVED"))
	evs, err := s.ListEvents(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "pr_review_changed", evs[0].Type)
	assert.Equal(t, "pr-1", evs[0].SourceID)
	assert.Equal(t, appstore.EventSourceGitHub, evs[0].SourceType)

	// Duplicate detection (same entity + eventType, still unread): CreateNotification
	// returns inserted=false, so no new event should be emitted.
	require.NoError(t, a.onVCSNotify(ctx, c.ID, "pull_request", "pr-1", "pr_review_changed", "", "APPROVED"))
	evs2, err := s.ListEvents(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, evs2, 1, "duplicate notification must not emit a second event")
}
