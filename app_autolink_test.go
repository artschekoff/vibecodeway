package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAutoLinkTestApp(t *testing.T) *App {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return &App{db: s, ctx: context.Background(), logger: newTestLogger()}
}

func TestSetAutoLinkRepos_persists(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.SetAutoLinkRepos(false))
	got, err := a.db.GetAppSettings(a.ctx)
	require.NoError(t, err)
	assert.False(t, got.AutoLinkReposOnContextCreate)

	require.NoError(t, a.SetAutoLinkRepos(true))
	got, err = a.db.GetAppSettings(a.ctx)
	require.NoError(t, err)
	assert.True(t, got.AutoLinkReposOnContextCreate)
}

func TestSetAppSettings_preservesAutoLink(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.SetAutoLinkRepos(false)) // turn OFF via dedicated setter

	// A normal settings save (which does not carry the auto-link flag) must not re-enable it.
	require.NoError(t, a.SetAppSettings(true, 15, false, 20, "/term", "cursor"))
	got, err := a.db.GetAppSettings(a.ctx)
	require.NoError(t, err)
	assert.False(t, got.AutoLinkReposOnContextCreate, "SetAppSettings must preserve the auto-link flag")
	assert.True(t, got.ContextRefetchEnabled) // other fields still saved
}

func TestAutoLinkReposOnce_disabledIsNoop(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.SetAutoLinkRepos(false))
	// Disabled → returns (0, nil) without touching AIMatchBranches/vcs (a.analysisSvc/a.vcsStore are nil here).
	added, err := a.autoLinkReposOnce("ctx-1")
	require.NoError(t, err)
	assert.Equal(t, 0, added)
}
