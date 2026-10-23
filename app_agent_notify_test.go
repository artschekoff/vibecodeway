package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchClaudeSettings(t *testing.T) {
	// existing other keys stay, channel inserted
	out, err := patchClaudeSettings("{\n  \"permissions\": {\"allow\": [\"Read\"]}\n}\n")
	require.NoError(t, err)
	assert.True(t, claudeBellEnabled(out))
	assert.Contains(t, out, `"permissions"`)
	// existing channel replaced in place
	out, err = patchClaudeSettings(`{"preferredNotifChannel": "auto", "x": 1}`)
	require.NoError(t, err)
	assert.Equal(t, `{"preferredNotifChannel": "terminal_bell", "x": 1}`, out)
	// idempotent; empty file created
	again, _ := patchClaudeSettings(out)
	assert.Equal(t, out, again)
	empty, err := patchClaudeSettings("")
	require.NoError(t, err)
	assert.True(t, claudeBellEnabled(empty))
}

func TestPatchCodexConfig(t *testing.T) {
	src := "model = \"x\"\n\n[tui]\nscreen_reader_detection_done = true\n\n[tui.model_availability_nux]\n\"gpt\" = 4\n"
	out, err := patchCodexConfig(src)
	require.NoError(t, err)
	assert.True(t, codexNotificationsEnabled(out))
	assert.Contains(t, out, "[tui]\nnotifications = true\nscreen_reader_detection_done = true\n")
	assert.Contains(t, out, "[tui.model_availability_nux]\n\"gpt\" = 4\n", "other sections untouched")
	assert.False(t, codexNotificationsEnabled("[tui.model_availability_nux]\nnotifications = true\n"), "only the [tui] section counts")

	// no [tui] section → appended; false → true
	out, _ = patchCodexConfig("model = \"x\"\n")
	assert.Equal(t, "model = \"x\"\n\n[tui]\nnotifications = true\n", out)
	out, _ = patchCodexConfig("[tui]\nnotifications = false\n")
	assert.Equal(t, "[tui]\nnotifications = true\n", out)
	again, _ := patchCodexConfig(out)
	assert.Equal(t, out, again)
}
