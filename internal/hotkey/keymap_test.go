package hotkey

import (
	"testing"

	gohk "golang.design/x/hotkey"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModifiers_CmdShift(t *testing.T) {
	mods, err := ParseModifiers([]string{"cmd", "shift"})
	require.NoError(t, err)
	assert.Equal(t, []gohk.Modifier{gohk.ModCmd, gohk.ModShift}, mods)
}

func TestParseModifiers_UnknownReturnsError(t *testing.T) {
	_, err := ParseModifiers([]string{"cmd", "superkey"})
	assert.Error(t, err)
}

func TestParseKey_Letter(t *testing.T) {
	key, err := ParseKey("c")
	require.NoError(t, err)
	assert.Equal(t, gohk.KeyC, key)
}

func TestParseKey_Unknown(t *testing.T) {
	_, err := ParseKey("tilde")
	assert.Error(t, err)
}

func TestFormatCombo_CmdShiftC(t *testing.T) {
	assert.Equal(t, "⌘⇧C", FormatCombo("cmd,shift", "c"))
}

func TestFormatCombo_CmdEnter(t *testing.T) {
	assert.Equal(t, "⌘↵", FormatCombo("cmd", "enter"))
}
