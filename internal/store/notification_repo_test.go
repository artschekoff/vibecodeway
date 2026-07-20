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

	inserted, err := s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "To Do", "Done")
	require.NoError(t, err)
	assert.True(t, inserted, "first notification should insert")

	// Same entity + event, still unread → dedup-skip.
	inserted, err = s.CreateNotification(ctx, c.ID, "jira_issue", "j-1", "jira_status_changed", "Done", "In Review")
	require.NoError(t, err)
	assert.False(t, inserted, "duplicate unread notification should be skipped")
}
