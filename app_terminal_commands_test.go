package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_TerminalCommands_roundTrip(t *testing.T) {
	a := newAutoLinkTestApp(t) // *App with a real store; see app_autolink_test.go

	got, err := a.ListTerminalCommands()
	require.NoError(t, err)
	assert.Empty(t, got)

	added, err := a.AddTerminalCommand("Claude", "claude")
	require.NoError(t, err)
	require.NotNil(t, added)

	require.NoError(t, a.UpdateTerminalCommand(added.ID, "Claude Code", "claude --resume"))
	got, err = a.ListTerminalCommands()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Claude Code", got[0].Title)
	assert.Equal(t, "claude --resume", got[0].Command)

	require.NoError(t, a.DeleteTerminalCommand(added.ID))
	got, err = a.ListTerminalCommands()
	require.NoError(t, err)
	assert.Empty(t, got)
}
