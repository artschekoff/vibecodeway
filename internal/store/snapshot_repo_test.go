package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetAndGetAnalyzedThrough(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	// No snapshot row exists yet — SetAnalyzedThrough must create one (upsert).
	require.NoError(t, s.SetAnalyzedThrough(ctx, c.ID, "2026-08-03T10:00:00Z"))
	got, err := s.GetSnapshot(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AnalyzedThrough)
	require.Equal(t, "2026-08-03T10:00:00Z", *got.AnalyzedThrough)

	// Advancing it updates in place.
	require.NoError(t, s.SetAnalyzedThrough(ctx, c.ID, "2026-08-03T12:00:00Z"))
	got2, err := s.GetSnapshot(ctx, c.ID)
	require.NoError(t, err)
	require.Equal(t, "2026-08-03T12:00:00Z", *got2.AnalyzedThrough)
}
