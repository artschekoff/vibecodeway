// Package symbols provides tags-based code navigation: a Universal Ctags
// symbol index per worktree plus git-grep references.
package symbols

import (
	"errors"
	"os/exec"
	"strings"
)

// ErrCtagsNotFound is the sentinel for "Universal Ctags is not installed".
// The Wails layer forwards its message verbatim so the frontend can render an
// install hint instead of a generic failure.
var ErrCtagsNotFound = errors.New("ctags-not-found")

// homebrewFallbacks are checked when $PATH has no acceptable ctags — GUI apps
// on macOS often run without the user's shell PATH.
var homebrewFallbacks = []string{"/opt/homebrew/bin/ctags", "/usr/local/bin/ctags"}

// FindCtags returns a path to a Universal Ctags binary. macOS ships a BSD
// ctags at /usr/bin/ctags with no JSON output, so a candidate is accepted
// only when `ctags --version` mentions "Universal Ctags".
func FindCtags(lookPath func(string) (string, error), runVersion func(bin string) (string, error)) (string, error) {
	var candidates []string
	if p, err := lookPath("ctags"); err == nil && p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, homebrewFallbacks...)
	for _, bin := range candidates {
		out, err := runVersion(bin)
		if err == nil && strings.Contains(out, "Universal Ctags") {
			return bin, nil
		}
	}
	return "", ErrCtagsNotFound
}

// DefaultFindCtags wires FindCtags to the real filesystem.
func DefaultFindCtags() (string, error) {
	return FindCtags(exec.LookPath, func(bin string) (string, error) {
		out, err := exec.Command(bin, "--version").CombinedOutput()
		return string(out), err
	})
}
