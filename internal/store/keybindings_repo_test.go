package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeybindings_TabsCloseSeeded_106(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	kb, err := s.GetKeybindingByCommand(ctx, "tabs.close")
	require.NoError(t, err)
	assert.Equal(t, "cmd,shift", kb.ModifierKeys)
	assert.Equal(t, "w", kb.KeyCode)
	assert.Equal(t, "", kb.Sequence)
	assert.Equal(t, "global", kb.Scope)
	assert.Equal(t, "cmd,shift", kb.DefaultModifierKeys)
	assert.Equal(t, "w", kb.DefaultKeyCode)
}

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
	assert.Equal(t, "t", th.KeyCode)

	// Rebind then reset restores the default.
	require.NoError(t, s.UpdateKeybinding(ctx, "search.global", "cmd,shift", "k", ""))
	require.NoError(t, s.ResetKeybinding(ctx, "search.global"))
	got, err := s.GetKeybindingByCommand(ctx, "search.global")
	require.NoError(t, err)
	assert.Equal(t, "cmd", got.ModifierKeys)
	assert.Equal(t, "f", got.KeyCode)
}

func TestKeybindings_SwitchRemap_107(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	all, err := s.GetKeybindings(ctx)
	require.NoError(t, err)
	byCmd := map[string]Keybinding{}
	for _, k := range all {
		byCmd[k.Command] = k
	}

	// Context tabs -> cmd+shift
	tn := byCmd["tabs.next"]
	assert.Equal(t, "cmd,shift", tn.ModifierKeys)
	assert.Equal(t, "]", tn.KeyCode)
	assert.Equal(t, "cmd,shift", tn.DefaultModifierKeys)
	assert.Equal(t, "cmd,shift", byCmd["tabs.prev"].ModifierKeys)
	assert.Equal(t, "[", byCmd["tabs.prev"].KeyCode)

	// Terminal switch -> plain cmd
	term := byCmd["terminal.next"]
	assert.Equal(t, "cmd", term.ModifierKeys)
	assert.Equal(t, "]", term.KeyCode)
	assert.Equal(t, "cmd", term.DefaultModifierKeys)
	assert.Equal(t, "cmd", byCmd["terminal.prev"].ModifierKeys)
	assert.Equal(t, "[", byCmd["terminal.prev"].KeyCode)

	// Sidebar: old next/prev gone, new direct files/git under ⌘B
	_, hadNext := byCmd["sidebar.next"]
	_, hadPrev := byCmd["sidebar.prev"]
	assert.False(t, hadNext, "sidebar.next removed")
	assert.False(t, hadPrev, "sidebar.prev removed")

	f := byCmd["sidebar.files"]
	assert.Equal(t, "cmd", f.ModifierKeys)
	assert.Equal(t, "b", f.KeyCode)
	assert.Equal(t, "f", f.Sequence)
	assert.Equal(t, "editor", f.Scope)

	g := byCmd["sidebar.git"]
	assert.Equal(t, "cmd", g.ModifierKeys)
	assert.Equal(t, "b", g.KeyCode)
	assert.Equal(t, "g", g.Sequence)
}

// Migration 110 takes "here" and "insert" off the ⌘T lead so ⌘T is a lone bare
// chord and a new terminal opens on the keypress instead of after the 600ms
// leader window.
func TestKeybindings_TerminalUnleader_110(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	all, err := s.GetKeybindings(ctx)
	require.NoError(t, err)
	byCmd := map[string]Keybinding{}
	for _, k := range all {
		byCmd[k.Command] = k
	}

	// ⌘T stays the bare "new terminal" chord, with nothing sequenced under it.
	newTerm := byCmd["terminal.new"]
	assert.Equal(t, "cmd", newTerm.ModifierKeys)
	assert.Equal(t, "t", newTerm.KeyCode)
	assert.Equal(t, "", newTerm.Sequence)

	here := byCmd["terminal.here"]
	assert.Equal(t, "cmd,shift", here.ModifierKeys)
	assert.Equal(t, "t", here.KeyCode)
	assert.Equal(t, "", here.Sequence)
	assert.Equal(t, "cmd,shift", here.DefaultModifierKeys)
	assert.Equal(t, "", here.DefaultSequence)

	insert := byCmd["terminal.insert"]
	assert.Equal(t, "cmd", insert.ModifierKeys)
	assert.Equal(t, "i", insert.KeyCode)
	assert.Equal(t, "", insert.Sequence)
	assert.Equal(t, "i", insert.DefaultKeyCode)
	assert.Equal(t, "", insert.DefaultSequence)
}
