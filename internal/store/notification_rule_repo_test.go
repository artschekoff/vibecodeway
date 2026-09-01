package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndListNotificationRules(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	rule, err := s.CreateNotificationRule(ctx, c.ID, "jira_status_changed", "Done", "tell QA we are ready")
	require.NoError(t, err)
	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, "Done", rule.MatchValue)
	assert.Equal(t, "tell QA we are ready", rule.Note)
	assert.True(t, rule.Enabled, "a new rule is enabled")
	assert.NotEmpty(t, rule.CreatedAt)

	rules, err := s.ListNotificationRules(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, rule.ID, rules[0].ID)
	assert.Equal(t, c.ID, rules[0].ContextID)
	assert.Equal(t, "jira_status_changed", rules[0].EventType)
}

func TestListNotificationRulesIsScopedToOneContextAndOrdered(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "PROJ-2", org.ID)
	require.NoError(t, err)

	_, err = s.CreateNotificationRule(ctx, c1.ID, "jira_status_changed", "Done", "first")
	require.NoError(t, err)
	_, err = s.CreateNotificationRule(ctx, c1.ID, "jira_status_changed", "In Review", "second")
	require.NoError(t, err)
	_, err = s.CreateNotificationRule(ctx, c2.ID, "jira_status_changed", "Done", "other context")
	require.NoError(t, err)

	rules, err := s.ListNotificationRules(ctx, c1.ID)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	// Creation order, because "first match wins" downstream must be predictable.
	assert.Equal(t, "first", rules[0].Note)
	assert.Equal(t, "second", rules[1].Note)
}

func TestListNotificationRulesEmptyForUnknownContext(t *testing.T) {
	s := openTestStore(t)
	rules, err := s.ListNotificationRules(context.Background(), "no-such-context")
	require.NoError(t, err)
	assert.Empty(t, rules)
}

func TestNotificationNoteRoundTrips(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "PROJ-1", org.ID)
	require.NoError(t, err)

	inserted, err := s.CreateNotification(ctx, c.ID, "jira_issue", "j-1",
		"jira_status_changed", "To Do", "Done", "tell QA we are ready")
	require.NoError(t, err)
	require.True(t, inserted)

	unread, err := s.ListUnreadNotifications(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "tell QA we are ready", unread[0].Note)
}
