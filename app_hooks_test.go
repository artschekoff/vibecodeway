package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestLogger() *zap.Logger { return zap.NewNop() }

func TestRunScriptHook_WritesMarkerWithEnv(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "out.txt")
	// Script writes the injected env var to a file so we can assert it ran with context.
	cmd := "printf '%s' \"$CONTEXT_KEY\" > " + marker
	err := runScriptHook(cmd, "PROJ-1", "ctx-id-1", "pr_update")
	require.NoError(t, err)

	got, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "PROJ-1", string(got))
}

func TestDispatchContextHooks_RunsEnabledScript(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	dir := t.TempDir()
	marker := filepath.Join(dir, "fired.txt")
	_, err = s.CreateContextEventHook(ctx, c.ID, "pr_update", "run_script",
		"printf ran > "+marker)
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}
	a.dispatchContextHooks(c.ID, "pr_update")

	// dispatchContextHooks runs scripts synchronously for run_script hooks.
	_, statErr := os.Stat(marker)
	assert.NoError(t, statErr, "enabled pr_update script hook should have run")

	// A category with no hook is a no-op (must not panic).
	a.dispatchContextHooks(c.ID, "jira_update")
}
