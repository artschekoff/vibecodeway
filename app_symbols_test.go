package main

import (
	"testing"

	"github.com/artschekoff/vibecodeway/internal/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// The bindings must pass through the symbols service and keep the sentinel
// error message verbatim so the frontend can detect the missing binary.
func TestSymbolLookup_forwardsCtagsNotFound(t *testing.T) {
	app := &App{logger: zap.NewNop(), symbolsSvc: symbols.NewService(zap.NewNop())}
	// Point the service at a directory that is not a git repo AND stub ctags
	// away is unnecessary: on a machine without Universal Ctags the real error
	// path already returns ErrCtagsNotFound. To make the test deterministic,
	// use the exported test hook:
	app.symbolsSvc.SetFindCtagsForTest(func() (string, error) { return "", symbols.ErrCtagsNotFound })

	_, err := app.SymbolLookup(t.TempDir(), "anything")
	require.Error(t, err)
	assert.Equal(t, "ctags-not-found", err.Error())
}
