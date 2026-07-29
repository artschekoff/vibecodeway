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

	cmd, err := s.AddTerminalCommand(ctx, "Inject", "/load-context <context-name>")
	require.NoError(t, err)
	assert.NotEmpty(t, cmd.ID)
	assert.Equal(t, "/load-context <context-name>", cmd.Command)

	require.NoError(t, s.UpdateTerminalCommand(ctx, cmd.ID, "Inject ctx", "/load <context-name>"))
	list, err = s.ListTerminalCommands(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Inject ctx", list[0].Title)
	assert.Equal(t, "/load <context-name>", list[0].Command)

	require.NoError(t, s.DeleteTerminalCommand(ctx, cmd.ID))
	list, err = s.ListTerminalCommands(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}
