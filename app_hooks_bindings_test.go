package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextEventHookBindings(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx}

	h, err := a.CreateContextEventHook(c.ID, "code_review", "os_notification", "")
	require.NoError(t, err)
	assert.NotEmpty(t, h.ID)

	list, err := a.ListContextEventHooks(c.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, a.DeleteContextEventHook(h.ID))
	list, err = a.ListContextEventHooks(c.ID)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}
