package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalEventHooksRepo(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	h, err := s.CreateGlobalEventHook(ctx, "jira_update", "os_notification", "")
	require.NoError(t, err)
	assert.NotEmpty(t, h.ID)
	assert.True(t, h.Enabled)

	_, err = s.CreateGlobalEventHook(ctx, "pr_update", "run_script", "echo hi")
	require.NoError(t, err)

	all, err := s.ListGlobalEventHooks(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	enabled, err := s.ListEnabledGlobalHooks(ctx, "jira_update")
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, "os_notification", enabled[0].ActionType)

	none, err := s.ListEnabledGlobalHooks(ctx, "code_review")
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Len(t, none, 0)

	require.NoError(t, s.DeleteGlobalEventHook(ctx, h.ID))
	all, err = s.ListGlobalEventHooks(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}
