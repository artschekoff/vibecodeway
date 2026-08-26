package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClearStaleNeedsAttention is the cleanup mechanism for a flag that is otherwise
// one-way: FlagNeedsAttention is called on every sync notification and nothing
// ever took it back off.
func TestClearStaleNeedsAttention(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	newFlagged := func(key string) *Context {
		c, err := s.CreateContext(ctx, key, org.ID)
		require.NoError(t, err)
		require.NoError(t, s.SetSnapshotStatus(ctx, c.ID, "in_progress"))
		_, err = s.AppendStatusLog(ctx, c.ID, "in_progress", "working", "user", nil)
		require.NoError(t, err)
		require.NoError(t, s.FlagNeedsAttention(ctx, c.ID, "New jira_issue jira_status_changed (To Do → Done)"))
		return c
	}
	status := func(id string) string {
		got, err := s.GetContext(ctx, id)
		require.NoError(t, err)
		return got.SnapshotStatus
	}

	// Archived: the user closed this out, there is nothing left to react to.
	archived := newFlagged("ARCH-1")
	_, err = s.ToggleContextStatus(ctx, archived.ID)
	require.NoError(t, err)

	// Active with every notification read: the user has seen it all.
	read := newFlagged("READ-1")
	_, err = s.CreateNotification(ctx, read.ID, "jira_issue", "j-read", "jira_status_changed", "To Do", "Done")
	require.NoError(t, err)
	require.NoError(t, s.MarkNotificationsRead(ctx, read.ID))

	// Active with an unread notification: still the user's move, must survive.
	unread := newFlagged("LIVE-1")
	_, err = s.CreateNotification(ctx, unread.ID, "jira_issue", "j-live", "jira_status_changed", "To Do", "In Review")
	require.NoError(t, err)

	n, err := s.ClearStaleNeedsAttention(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "archived and fully-read contexts get cleared")

	// Cleared contexts fall back to whatever they were before the flag.
	assert.Equal(t, "in_progress", status(archived.ID))
	assert.Equal(t, "in_progress", status(read.ID))
	assert.Equal(t, "needs_attention", status(unread.ID))

	// Idempotent: a second sweep finds nothing left to clear.
	n2, err := s.ClearStaleNeedsAttention(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n2)
}

// A context flagged with no prior status recorded falls back to not_started
// rather than staying stuck on needs_attention forever.
func TestClearStaleNeedsAttentionWithoutPriorStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "BARE-1", org.ID)
	require.NoError(t, err)
	require.NoError(t, s.FlagNeedsAttention(ctx, c.ID, "New pull_request ci_failure"))
	_, err = s.ToggleContextStatus(ctx, c.ID)
	require.NoError(t, err)

	n, err := s.ClearStaleNeedsAttention(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got, err := s.GetContext(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "not_started", got.SnapshotStatus)
}

// The sweep only undoes flags the sync layer set itself. A needs_attention the
// user or the AI chose deliberately is a judgement, not notification noise.
func TestClearStaleNeedsAttentionKeepsDeliberateFlags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	deliberate := func(key, source string) string {
		c, err := s.CreateContext(ctx, key, org.ID)
		require.NoError(t, err)
		require.NoError(t, s.SetSnapshotStatus(ctx, c.ID, "needs_attention"))
		_, err = s.AppendStatusLog(ctx, c.ID, "needs_attention", "blocked on infra", source, nil)
		require.NoError(t, err)
		// Archived — the sweep would clear it if source were ignored.
		_, err = s.ToggleContextStatus(ctx, c.ID)
		require.NoError(t, err)
		return c.ID
	}
	byUser := deliberate("USER-1", "user")
	byAI := deliberate("AI-1", "ai")

	// A flag with no status-log entry at all is orphaned legacy state: it renders
	// as a blank tile with no reason, so it carries nothing worth keeping.
	orphan, err := s.CreateContext(ctx, "ORPHAN-1", org.ID)
	require.NoError(t, err)
	require.NoError(t, s.SetSnapshotStatus(ctx, orphan.ID, "needs_attention"))

	n, err := s.ClearStaleNeedsAttention(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the orphaned flag is cleared")
	orphanGot, err := s.GetContext(ctx, orphan.ID)
	require.NoError(t, err)
	assert.Equal(t, "not_started", orphanGot.SnapshotStatus)

	for _, id := range []string{byUser, byAI} {
		got, err := s.GetContext(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "needs_attention", got.SnapshotStatus)
	}
}
