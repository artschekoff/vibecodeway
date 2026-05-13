package hotkey

import (
	"fmt"
	"strings"

	gohk "golang.design/x/hotkey"
)

var modifierMap = map[string]gohk.Modifier{
	"cmd":   gohk.ModCmd,
	"shift": gohk.ModShift,
	"ctrl":  gohk.ModCtrl,
	"alt":   gohk.ModOption,
}

var keyMap = map[string]gohk.Key{
	"a": gohk.KeyA, "b": gohk.KeyB, "c": gohk.KeyC, "d": gohk.KeyD,
	"e": gohk.KeyE, "f": gohk.KeyF, "g": gohk.KeyG, "h": gohk.KeyH,
	"i": gohk.KeyI, "j": gohk.KeyJ, "k": gohk.KeyK, "l": gohk.KeyL,
	"m": gohk.KeyM, "n": gohk.KeyN, "o": gohk.KeyO, "p": gohk.KeyP,
	"q": gohk.KeyQ, "r": gohk.KeyR, "s": gohk.KeyS, "t": gohk.KeyT,
	"u": gohk.KeyU, "v": gohk.KeyV, "w": gohk.KeyW, "x": gohk.KeyX,
	"y": gohk.KeyY, "z": gohk.KeyZ,
	"0": gohk.Key0, "1": gohk.Key1, "2": gohk.Key2, "3": gohk.Key3,
	"4": gohk.Key4, "5": gohk.Key5, "6": gohk.Key6, "7": gohk.Key7,
	"8": gohk.Key8, "9": gohk.Key9,
	"enter": gohk.KeyReturn, "space": gohk.KeySpace,
	"f1": gohk.KeyF1, "f2": gohk.KeyF2, "f3": gohk.KeyF3, "f4": gohk.KeyF4,
	"f5": gohk.KeyF5, "f6": gohk.KeyF6, "f7": gohk.KeyF7, "f8": gohk.KeyF8,
	"f9": gohk.KeyF9, "f10": gohk.KeyF10, "f11": gohk.KeyF11, "f12": gohk.KeyF12,
}

var modifierSymbols = map[string]string{
	"cmd":   "⌘",
	"shift": "⇧",
	"ctrl":  "⌃",
	"alt":   "⌥",
}

var keySymbols = map[string]string{
	"enter": "↵",
	"space": "␣",
}

// ParseModifiers converts a slice of modifier name strings into gohk.Modifier values.
func ParseModifiers(names []string) ([]gohk.Modifier, error) {
	mods := make([]gohk.Modifier, 0, len(names))
	for _, n := range names {
		m, ok := modifierMap[strings.ToLower(strings.TrimSpace(n))]
		if !ok {
			return nil, fmt.Errorf("unknown modifier %q", n)
		}
		mods = append(mods, m)
	}
	return mods, nil
}

// ParseKey converts a key name string into a gohk.Key value.
func ParseKey(name string) (gohk.Key, error) {
	k, ok := keyMap[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return 0, fmt.Errorf("unknown key %q", name)
	}
	return k, nil
}

// FormatCombo builds a display string like "⌘⇧C" from stored values.
// modifierKeys is a comma-separated string (e.g. "cmd,shift"), keyCode is e.g. "c".
func FormatCombo(modifierKeys, keyCode string) string {
	var b strings.Builder
	for _, m := range strings.Split(modifierKeys, ",") {
		m = strings.TrimSpace(m)
		if sym, ok := modifierSymbols[m]; ok {
			b.WriteString(sym)
		}
	}
	if sym, ok := keySymbols[keyCode]; ok {
		b.WriteString(sym)
	} else {
		b.WriteString(strings.ToUpper(keyCode))
	}
	return b.String()
}
