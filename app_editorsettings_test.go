package main

import (
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditorSettings_roundTripAndDefaults(t *testing.T) {
	a := newAutoLinkTestApp(t)

	// Unset => empty string (frontend then applies its own defaults).
	got, err := a.GetEditorSettings()
	require.NoError(t, err)
	assert.Equal(t, "", got)

	blob := `{"theme":"vs-dark","wordWrap":"on","fontSize":14}`
	require.NoError(t, a.SetEditorSettings(blob))

	got, err = a.GetEditorSettings()
	require.NoError(t, err)
	assert.Equal(t, blob, got)
}

func TestGitStatusLimit_readsSettingFallsBackToDefault(t *testing.T) {
	a := newAutoLinkTestApp(t)

	assert.Equal(t, vcs.DefaultStatusLimit, a.gitStatusLimit(), "unset settings -> default")

	require.NoError(t, a.SetEditorSettings(`{"theme":"dark"}`))
	assert.Equal(t, vcs.DefaultStatusLimit, a.gitStatusLimit(), "field absent -> default")

	require.NoError(t, a.SetEditorSettings(`{"statusLimit":500}`))
	assert.Equal(t, 500, a.gitStatusLimit())

	require.NoError(t, a.SetEditorSettings(`{"statusLimit":0}`))
	assert.Equal(t, 0, a.gitStatusLimit(), "0 = unlimited, passed through")

	require.NoError(t, a.SetEditorSettings(`{"statusLimit":-5}`))
	assert.Equal(t, vcs.DefaultStatusLimit, a.gitStatusLimit(), "negative -> default")

	require.NoError(t, a.SetEditorSettings(`not json`))
	assert.Equal(t, vcs.DefaultStatusLimit, a.gitStatusLimit(), "bad json -> default")
}

func TestSetEditorSettings_preservesOtherSettings(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.SetAutoLinkRepos(false))

	require.NoError(t, a.SetEditorSettings(`{"theme":"light"}`))

	got, err := a.db.GetAppSettings(a.ctx)
	require.NoError(t, err)
	assert.False(t, got.AutoLinkReposOnContextCreate, "editor settings save must preserve other fields")
	assert.Equal(t, `{"theme":"light"}`, got.EditorSettings)
}
