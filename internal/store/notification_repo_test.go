package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateNotificationInsertedFlag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	inserted, err := s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "To Do", "Done", "")
	require.NoError(t, err)
	assert.True(t, inserted, "first notification should insert")

	// Same entity + event + new_value, still unread → dedup-skip: this is the
	// same change re-detected on a later sync, which is what the de-dupe exists
	// to suppress. (A different new_value on the same entity/event must insert —
	// see TestCreateNotificationDedupeKeysOnNewValueNotJustEntity below.)
	inserted, err = s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "To Do", "Done", "")
	require.NoError(t, err)
	assert.False(t, inserted, "duplicate unread notification should be skipped")
}

// TestCreateNotificationDedupeKeysOnNewValueNotJustEntity is the regression test
// for the finding that the de-dupe swallowed a rule's note: PROJ-1 goes
// To Do -> In Review (no rule matches, unread row inserted), then later goes
// In Review -> Done (a rule matches "Done"). Before new_value joined the
// de-dupe predicate, the second CreateNotification found the still-unread
// first row and returned false, discarding the note. Both transitions must
// insert because they are genuinely different changes.
func TestCreateNotificationDedupeKeysOnNewValueNotJustEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	inserted, err := s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "To Do", "In Review", "")
	require.NoError(t, err)
	assert.True(t, inserted, "first transition should insert")

	// Different transition on the same entity, same event_type, while the first
	// is still unread. Must still insert — this is the bug being fixed.
	inserted, err = s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "In Review", "Done", "tell QA we are ready")
	require.NoError(t, err)
	assert.True(t, inserted, "a different transition on the same entity must not be swallowed by an older unread row")
}

// TestCreateNotificationDedupeStillBlocksSameTransitionReplayed proves the spam
// gate the de-dupe exists for still works: re-detecting the exact same
// transition on a later sync, while the first is still unread, must not
// produce a second notification.
func TestCreateNotificationDedupeStillBlocksSameTransitionReplayed(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	inserted, err := s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "In Review", "Done", "tell QA we are ready")
	require.NoError(t, err)
	assert.True(t, inserted)

	// Same transition re-detected on a later sync, still unread -> de-duped.
	inserted, err = s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "In Review", "Done", "tell QA we are ready")
	require.NoError(t, err)
	assert.False(t, inserted, "the same transition replayed while unread must still be de-duped")
}
