package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppSettings_AutoLinkReposDefaultAndRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Default (no row yet) is ON.
	got, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.True(t, got.AutoLinkReposOnContextCreate, "default should be true")

	// Round-trip false.
	got.AutoLinkReposOnContextCreate = false
	require.NoError(t, s.SetAppSettings(ctx, *got))
	got2, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.False(t, got2.AutoLinkReposOnContextCreate)
}

func TestAppSettingsPRStaleThresholdDefaultAndSet(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	got, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 7, got.PRStaleThresholdDays, "default threshold is 7")

	got.PRStaleThresholdDays = 14
	require.NoError(t, s.SetAppSettings(ctx, *got))
	again, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 14, again.PRStaleThresholdDays)
}
