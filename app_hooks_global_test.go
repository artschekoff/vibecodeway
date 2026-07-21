package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatch_GlobalFallbackAndOverride(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	dir := t.TempDir()
	globalMarker := filepath.Join(dir, "global.txt")
	ctxMarker := filepath.Join(dir, "ctx.txt")

	// Global run_script hooks for two categories.
	_, err = s.CreateGlobalEventHook(ctx, "jira_update", "run_script", "printf g > "+globalMarker)
	require.NoError(t, err)
	_, err = s.CreateGlobalEventHook(ctx, "pr_update", "run_script", "printf g > "+globalMarker)
	require.NoError(t, err)

	// Context overrides pr_update only.
	_, err = s.CreateContextEventHook(ctx, c.ID, "pr_update", "run_script", "printf c > "+ctxMarker)
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}

	// jira_update: no context hook → global fires.
	a.dispatchContextHooks(c.ID, "jira_update")
	_, err = os.Stat(globalMarker)
	assert.NoError(t, err, "global jira_update hook should have run")

	// pr_update: context hook exists → context fires, global does NOT.
	_ = os.Remove(globalMarker)
	a.dispatchContextHooks(c.ID, "pr_update")
	_, ctxErr := os.Stat(ctxMarker)
	assert.NoError(t, ctxErr, "context pr_update hook should have run")
	_, gErr := os.Stat(globalMarker)
	assert.Error(t, gErr, "global pr_update must NOT run when context overrides it")
}
