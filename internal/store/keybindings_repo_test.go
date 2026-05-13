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
	assert.Len(t, bindings, 2)
	assert.Equal(t, "cmd,shift", bindings[0].ModifierKeys)
	assert.Equal(t, "c", bindings[0].KeyCode)
}

func TestUpdateKeybinding_PersistsNewCombo(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.UpdateKeybinding(ctx, "quick_attach", "cmd,shift", "x")
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

	err := s.UpdateKeybinding(ctx, "nonexistent_command", "cmd", "z")
	assert.Error(t, err)
}
