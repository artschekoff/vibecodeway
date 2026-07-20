package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextEventHooksRepo(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	t.Run("create, list, list-enabled, delete", func(t *testing.T) {
		h1, err := s.CreateContextEventHook(ctx, c.ID, "jira_update", "os_notification", "")
		require.NoError(t, err)
		assert.NotEmpty(t, h1.ID)
		assert.True(t, h1.Enabled)

		_, err = s.CreateContextEventHook(ctx, c.ID, "pr_update", "run_script", "echo hi")
		require.NoError(t, err)

		all, err := s.ListContextEventHooks(ctx, c.ID)
		require.NoError(t, err)
		assert.Len(t, all, 2)

		enabled, err := s.ListEnabledHooks(ctx, c.ID, "jira_update")
		require.NoError(t, err)
		require.Len(t, enabled, 1)
		assert.Equal(t, "os_notification", enabled[0].ActionType)

		require.NoError(t, s.DeleteContextEventHook(ctx, h1.ID))
		all, err = s.ListContextEventHooks(ctx, c.ID)
		require.NoError(t, err)
		assert.Len(t, all, 1)
	})

	t.Run("empty lists are non-nil", func(t *testing.T) {
		empty, err := s.ListContextEventHooks(ctx, "no-such-context")
		require.NoError(t, err)
		assert.NotNil(t, empty)
		assert.Len(t, empty, 0)
	})

	t.Run("ContextIDsForRepo resolves via context_vcs_branches", func(t *testing.T) {
		// Seed a repo and a branch link so the resolver has data.
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO vcs_repos (id, platform, owner, name, created_at) VALUES (?, ?, ?, ?, ?)`,
			"repo-1", "github", "acme", "api", "2026-07-20T00:00:00Z")
		require.NoError(t, err)
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, created_at) VALUES (?, ?, ?, ?, ?)`,
			"cvb-1", c.ID, "repo-1", "main", "2026-07-20T00:00:00Z")
		require.NoError(t, err)

		ids, err := s.ContextIDsForRepo(ctx, "repo-1")
		require.NoError(t, err)
		assert.Equal(t, []string{c.ID}, ids)

		none, err := s.ContextIDsForRepo(ctx, "repo-none")
		require.NoError(t, err)
		assert.NotNil(t, none)
		assert.Len(t, none, 0)
	})
}
