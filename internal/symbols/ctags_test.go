package symbols

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindCtags_acceptsUniversalRejectsBSD(t *testing.T) {
	look := func(name string) (string, error) { return "/usr/bin/ctags", nil }
	// macOS BSD ctags: --version is not a flag; output lacks the marker.
	bsd := func(bin string) (string, error) { return "usage: ctags ...", nil }
	_, err := FindCtags(look, bsd)
	require.ErrorIs(t, err, ErrCtagsNotFound)

	universal := func(bin string) (string, error) {
		return "Universal Ctags 6.1.0, Copyright (C) 2015-2023", nil
	}
	bin, err := FindCtags(look, universal)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/ctags", bin)
}

func TestFindCtags_fallsBackToHomebrewPaths(t *testing.T) {
	look := func(name string) (string, error) { return "", errors.New("not in PATH") }
	seen := []string{}
	version := func(bin string) (string, error) {
		seen = append(seen, bin)
		if bin == "/opt/homebrew/bin/ctags" {
			return "Universal Ctags 6.1.0", nil
		}
		return "", errors.New("no such file")
	}
	bin, err := FindCtags(look, version)
	require.NoError(t, err)
	assert.Equal(t, "/opt/homebrew/bin/ctags", bin)
	assert.Contains(t, seen, "/opt/homebrew/bin/ctags")
}
