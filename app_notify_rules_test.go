package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newNotifyTestApp builds an App with a real store and no Wails runtime. Safe
// because no hooks are configured in a fresh DB, so dispatchContextHooks
// returns before it would emit a runtime event.
func newNotifyTestApp(t *testing.T) (*App, *appstore.Store, string) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	return &App{db: s, ctx: ctx, logger: zap.NewNop()}, s, c.ID
}

func TestOnJiraNotifyStoresTheMatchedRuleNote(t *testing.T) {
	a, s, contextID := newNotifyTestApp(t)
	ctx := context.Background()

	_, err := s.CreateNotificationRule(ctx, contextID, "jira_status_changed", "Done", "tell QA we are ready")
	require.NoError(t, err)

	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "In Review", "Done"))

	unread, err := s.ListUnreadNotifications(ctx, contextID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "tell QA we are ready", unread[0].Note)
	assert.Equal(t, "Done", unread[0].NewValue)
}

func TestOnJiraNotifyStoresNoNoteWhenNoRuleMatches(t *testing.T) {
	a, s, contextID := newNotifyTestApp(t)
	ctx := context.Background()

	// A rule exists, but for a different status.
	_, err := s.CreateNotificationRule(ctx, contextID, "jira_status_changed", "Blocked", "unblock me")
	require.NoError(t, err)

	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "In Review", "Done"))

	unread, err := s.ListUnreadNotifications(ctx, contextID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "", unread[0].Note)
}

func TestOnJiraNotifyWithNoRulesBehavesAsBefore(t *testing.T) {
	a, s, contextID := newNotifyTestApp(t)
	ctx := context.Background()

	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "To Do", "In Review"))

	unread, err := s.ListUnreadNotifications(ctx, contextID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "", unread[0].Note)

	// A non-terminal status still flags the context for attention. That flag
	// lives on the snapshot (FlagNeedsAttention -> SetSnapshotStatus), not on
	// the contexts row.
	snap, err := s.GetSnapshot(ctx, contextID)
	require.NoError(t, err)
	assert.Equal(t, "needs_attention", snap.Status)
}
