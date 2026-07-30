package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTCStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestTerminalCommands_CRUD(t *testing.T) {
	s := newTCStore(t)
	ctx := t.Context()

	list, err := s.ListTerminalCommands(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	cmd, err := s.AddTerminalCommand(ctx, "Inject", "/load-context <context-name>", true)
	require.NoError(t, err)
	assert.NotEmpty(t, cmd.ID)
	assert.Equal(t, "/load-context <context-name>", cmd.Command)
	assert.True(t, cmd.SendEnter)

	require.NoError(t, s.UpdateTerminalCommand(ctx, cmd.ID, "Inject ctx", "/load <context-name>", false))
	list, err = s.ListTerminalCommands(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Inject ctx", list[0].Title)
	assert.Equal(t, "/load <context-name>", list[0].Command)
	assert.False(t, list[0].SendEnter)

	require.NoError(t, s.DeleteTerminalCommand(ctx, cmd.ID))
	list, err = s.ListTerminalCommands(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}
