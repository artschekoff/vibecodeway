package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertAndListStatEventsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "pr_merged",
		ExternalID: "https://github.com/org/repo/pull/1",
		Org:        "juna-api",
		Repo:       "juna-api",
		JiraKey:    "JH-1",
		OccurredAt: "2026-08-15T10:00:00Z",
	}))

	got, err := s.ListStatEventsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "pr_merged", got[0].Kind)
	assert.Equal(t, "https://github.com/org/repo/pull/1", got[0].ExternalID)
	assert.Equal(t, "juna-api", got[0].Org)
	assert.Equal(t, "juna-api", got[0].Repo)
	assert.Equal(t, "JH-1", got[0].JiraKey)
	assert.Equal(t, "2026-08-15T10:00:00Z", got[0].OccurredAt)
}

func TestInsertStatEventDuplicateIsNoOp(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := StatEvent{
		Kind:       "comment_authored",
		ExternalID: "comment-1",
		OccurredAt: "2026-08-15T10:00:00Z",
	}
	require.NoError(t, s.InsertStatEvent(ctx, e))
	require.NoError(t, s.InsertStatEvent(ctx, e))

	got, err := s.ListStatEventsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestInsertStatEventSameExternalIDDifferentKindIsSeparateRow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "pr_merged",
		ExternalID: "shared-id",
		OccurredAt: "2026-08-15T10:00:00Z",
	}))
	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "comment_authored",
		ExternalID: "shared-id",
		OccurredAt: "2026-08-15T10:00:00Z",
	}))

	got, err := s.ListStatEventsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListStatEventsSinceFiltersAndOrders(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "pr_merged",
		ExternalID: "old-1",
		OccurredAt: "2026-01-01T10:00:00Z",
	}))
	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "pr_merged",
		ExternalID: "new-2",
		OccurredAt: "2026-08-20T10:00:00Z",
	}))
	require.NoError(t, s.InsertStatEvent(ctx, StatEvent{
		Kind:       "pr_merged",
		ExternalID: "new-1",
		OccurredAt: "2026-08-15T10:00:00Z",
	}))

	got, err := s.ListStatEventsSince(ctx, "2026-08-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "new-1", got[0].ExternalID)
	assert.Equal(t, "new-2", got[1].ExternalID)
}
