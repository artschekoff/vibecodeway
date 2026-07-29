package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusLog_AppendAndList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	// First entry has no prev
	e1, err := s.AppendStatusLog(ctx, c.ID, "in_progress", "Started work", "user", nil)
	require.NoError(t, err)
	require.NotEmpty(t, e1.ID)
	require.Nil(t, e1.PrevID)

	// Second entry links back to first
	e2, err := s.AppendStatusLog(ctx, c.ID, "blocked", "Waiting for Service A", "ai", &e1.ID)
	require.NoError(t, err)
	require.Equal(t, &e1.ID, e2.PrevID)

	// List returns both in chronological order
	entries, err := s.ListStatusLog(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "in_progress", entries[0].StatusSlug)
	require.Equal(t, "blocked", entries[1].StatusSlug)

	// GetLatest returns the newest
	latest, err := s.GetLatestStatusLogEntry(ctx, c.ID)
	require.NoError(t, err)
	require.Equal(t, "blocked", latest.StatusSlug)
	require.Equal(t, "Waiting for Service A", latest.Comment)
}

func TestFlagNeedsAttention_RecordsReasonOnTransitionOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	// Start from a non-attention status.
	require.NoError(t, s.SetSnapshotStatus(ctx, c.ID, "in_progress"))

	// First flip to needs_attention records the reason (source "system").
	require.NoError(t, s.FlagNeedsAttention(ctx, c.ID, "New jira update (To Do → In Progress)"))
	sn, err := s.GetSnapshot(ctx, c.ID)
	require.NoError(t, err)
	require.Equal(t, "needs_attention", sn.Status)
	latest, err := s.GetLatestStatusLogEntry(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, "needs_attention", latest.StatusSlug)
	require.Equal(t, "New jira update (To Do → In Progress)", latest.Comment)
	require.Equal(t, "system", latest.Source)

	// A second flip while already needs_attention does NOT append another entry
	// (avoids log spam on every sync notification).
	require.NoError(t, s.FlagNeedsAttention(ctx, c.ID, "another update"))
	entries, err := s.ListStatusLog(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "New jira update (To Do → In Progress)", entries[0].Comment)
}

func TestStatusLog_LatestNilWhenEmpty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	latest, err := s.GetLatestStatusLogEntry(ctx, c.ID)
	require.NoError(t, err)
	require.Nil(t, latest)
}
