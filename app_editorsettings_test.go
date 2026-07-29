package main

import (
	"testing"

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

func TestSetEditorSettings_preservesOtherSettings(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.SetAutoLinkRepos(false))

	require.NoError(t, a.SetEditorSettings(`{"theme":"light"}`))

	got, err := a.db.GetAppSettings(a.ctx)
	require.NoError(t, err)
	assert.False(t, got.AutoLinkReposOnContextCreate, "editor settings save must preserve other fields")
	assert.Equal(t, `{"theme":"light"}`, got.EditorSettings)
}
