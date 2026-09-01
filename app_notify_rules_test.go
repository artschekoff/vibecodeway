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

// TestOnJiraNotifyDispatchesHooksOnlyOnInsert pins dispatchContextHooks to the
// de-dupe gate: a regression that moves the hookDetail construction and the
// dispatch call outside `if inserted` would fire the hook a second time for a
// re-sync of the same unread change, and this test is the one that would catch
// it. run_script needs no Wails runtime, so its side effect (an appended marker
// line) is directly observable in a plain test.
func TestOnJiraNotifyDispatchesHooksOnlyOnInsert(t *testing.T) {
	a, s, contextID := newNotifyTestApp(t)
	ctx := context.Background()

	_, err := s.CreateNotificationRule(ctx, contextID, "jira_status_changed", "Done", "tell QA we are ready")
	require.NoError(t, err)

	dir := t.TempDir()
	marker := filepath.Join(dir, "fired.txt")
	_, err = s.CreateContextEventHook(ctx, contextID, "jira_update", "run_script", "printf 'x' >> "+marker)
	require.NoError(t, err)

	// Same entityID/eventType/newValue both times: the second call is de-duped
	// by CreateNotification because the first notification is still unread.
	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "In Review", "Done"))
	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "In Review", "Done"))

	unread, err := s.ListUnreadNotifications(ctx, contextID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "tell QA we are ready", unread[0].Note)

	got, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "x", string(got), "hook must fire exactly once, only on the insert that de-dupe allows")
}

// TestOnJiraNotifyDoesNotSwallowANoteCarryingTransition is the end-to-end
// regression test for the finding that the de-dupe swallowed the note in the
// feature's headline scenario: PROJ-1 goes To Do -> In Review with no rule
// matching (an unread notification is inserted for the entity), then a rule
// is created matching "Done", then PROJ-1 goes In Review -> Done. Before
// new_value joined CreateNotification's de-dupe predicate, the second call
// found the still-unread first row and returned false, discarding the note,
// skipping the hook dispatch, and leaving the Activity Feed showing the stale
// "To Do -> In Review" line. Both notifications must exist, and the Done one
// must carry the note.
func TestOnJiraNotifyDoesNotSwallowANoteCarryingTransition(t *testing.T) {
	a, s, contextID := newNotifyTestApp(t)
	ctx := context.Background()

	// Step 1: PROJ-1 goes To Do -> In Review. No rule exists yet.
	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "To Do", "In Review"))

	// Step 2: the user creates a rule: status becomes Done -> note.
	_, err := s.CreateNotificationRule(ctx, contextID, "jira_status_changed", "Done", "tell QA we are ready")
	require.NoError(t, err)

	// Step 3: PROJ-1 goes In Review -> Done.
	require.NoError(t, a.onJiraNotify(ctx, contextID, "jira_issue", "j-1",
		"jira_status_changed", "In Review", "Done"))

	unread, err := s.ListUnreadNotifications(ctx, contextID)
	require.NoError(t, err)
	require.Len(t, unread, 2, "both the To Do->In Review and In Review->Done transitions must have inserted a row")

	var doneNote, doneNewValue string
	for _, n := range unread {
		if n.NewValue == "Done" {
			doneNote = n.Note
			doneNewValue = n.NewValue
		}
	}
	assert.Equal(t, "Done", doneNewValue, "expected an unread notification for the Done transition")
	assert.Equal(t, "tell QA we are ready", doneNote, "the Done notification must carry the matched rule's note")
}
