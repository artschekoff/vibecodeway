package contextlog_test

import (
	"context"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAll_returnsLogsWithContextKey(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "sprint-42", org.ID)
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "First note")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "sprint-42", entries[0].ContextKey)
	assert.Equal(t, "First note", entries[0].SummaryMd)
	assert.Equal(t, "test-org", entries[0].OrgName)
}

func TestListAll_joinsOrgName(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "Acme Corp")
	require.NoError(t, err)

	c, err := s.CreateContext(ctx, "project-x", org.ID)
	require.NoError(t, err)

	err = s.SetContextOrg(ctx, c.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "Org note")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Acme Corp", entries[0].OrgName)
	assert.Equal(t, "project-x", entries[0].ContextKey)
}

func TestListAll_filterByTrigger(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx-a", org.ID)
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "Manual entry")
	require.NoError(t, err)
	_, err = svc.CreateAgentLog(ctx, c.ID, "Agent entry")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())

	manual, err := repo.ListAll(ctx, contextlog.TriggerManual, 50)
	require.NoError(t, err)
	require.Len(t, manual, 1)
	assert.Equal(t, contextlog.TriggerManual, manual[0].Trigger)

	agent, err := repo.ListAll(ctx, contextlog.TriggerAgent, 50)
	require.NoError(t, err)
	require.Len(t, agent, 1)
	assert.Equal(t, contextlog.TriggerAgent, agent[0].Trigger)

	all, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestListAll_respectsLimit(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx-b", org.ID)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = svc.CreateManualLog(ctx, c.ID, "note")
		require.NoError(t, err)
	}

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestListAll_orderedNewestFirst(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ctx-c", org.ID)
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "first")
	require.NoError(t, err)
	_, err = svc.CreateManualLog(ctx, c.ID, "second")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "second", entries[0].SummaryMd)
	assert.Equal(t, "first", entries[1].SummaryMd)
}
