package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKeybindings_ReturnsBothDefaults(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	bindings, err := s.GetKeybindings(ctx)
	require.NoError(t, err)
	// quick_attach + send_chat (052_keybindings.sql) plus the full hotkey
	// registry seeded in 097_hotkeys_registry.sql.
	assert.GreaterOrEqual(t, len(bindings), 2)
	var qa Keybinding
	for _, b := range bindings {
		if b.Command == "quick_attach" {
			qa = b
		}
	}
	assert.Equal(t, "cmd,shift", qa.ModifierKeys)
	assert.Equal(t, "c", qa.KeyCode)
}

func TestUpdateKeybinding_PersistsNewCombo(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.UpdateKeybinding(ctx, "quick_attach", "cmd,shift", "x", "")
	require.NoError(t, err)

	bindings, err := s.GetKeybindings(ctx)
	require.NoError(t, err)

	var qa Keybinding
	for _, b := range bindings {
		if b.Command == "quick_attach" {
			qa = b
		}
	}
	assert.Equal(t, "cmd,shift", qa.ModifierKeys)
	assert.Equal(t, "x", qa.KeyCode)
	assert.NotEmpty(t, qa.UpdatedAt)
}

func TestUpdateKeybinding_UnknownCommandReturnsError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.UpdateKeybinding(ctx, "nonexistent_command", "cmd", "z", "")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestKeybindings_SeededHotkeysAndReset(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	all, err := s.GetKeybindings(ctx)
	require.NoError(t, err)
	byCmd := map[string]Keybinding{}
	for _, k := range all {
		byCmd[k.Command] = k
	}

	kb, ok := byCmd["search.global"]
	require.True(t, ok, "search.global seeded")
	assert.Equal(t, "cmd", kb.ModifierKeys) // stored as platform-neutral "cmd"; frontend maps to mod
	assert.Equal(t, "f", kb.KeyCode)
	assert.Equal(t, "global", kb.Scope)

	th, ok := byCmd["terminal.here"]
	require.True(t, ok)
	assert.Equal(t, "h", th.Sequence)

	// Rebind then reset restores the default.
	require.NoError(t, s.UpdateKeybinding(ctx, "search.global", "cmd,shift", "k", ""))
	require.NoError(t, s.ResetKeybinding(ctx, "search.global"))
	got, err := s.GetKeybindingByCommand(ctx, "search.global")
	require.NoError(t, err)
	assert.Equal(t, "cmd", got.ModifierKeys)
	assert.Equal(t, "f", got.KeyCode)
}
