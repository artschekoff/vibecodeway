package store

import (
	"context"
	"testing"

	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedIssueWithPriority(t *testing.T, s *Store, contextID, jiraKey, priority string) {
	t.Helper()
	jiraRepo := jirastore.NewRepo(s.DB())
	_, err := jiraRepo.UpsertIssue(context.Background(), contextID, jiraKey, "summary", "todo", "", "task", "", "", "", priority)
	require.NoError(t, err, "seed issue with priority")
}

func findByID(list []Context, id string) *Context {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func TestListContextsImportanceAverage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "importance-ctx", org.ID)
	require.NoError(t, err)

	// Two issues: High(4) and Medium(3) -> avg 3.5
	seedIssueWithPriority(t, s, c.ID, "ABC-1", "High")
	seedIssueWithPriority(t, s, c.ID, "ABC-2", "Medium")

	list, err := s.ListContexts(ctx, true)
	require.NoError(t, err)
	got := findByID(list, c.ID)
	require.NotNil(t, got)
	require.NotNil(t, got.Importance)
	assert.InDelta(t, 3.5, *got.Importance, 0.001)
}

func TestListContextsImportanceNilWhenNoPricedIssues(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "no-priority-ctx", org.ID)
	require.NoError(t, err)

	// Issue with unknown/empty priority -> excluded from AVG -> nil Importance.
	seedIssueWithPriority(t, s, c.ID, "ABC-3", "")

	list, err := s.ListContexts(ctx, true)
	require.NoError(t, err)
	got := findByID(list, c.ID)
	require.NotNil(t, got)
	assert.Nil(t, got.Importance)
}

// ListContextsByOrg shares scanContextRows with ListContexts, so its SELECT
// must carry the same trailing importance column to keep scan positions aligned.
func TestListContextsByOrgImportanceAverage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "by-org-ctx", org.ID)
	require.NoError(t, err)

	seedIssueWithPriority(t, s, c.ID, "ABC-4", "High")
	seedIssueWithPriority(t, s, c.ID, "ABC-5", "Medium")

	list, err := s.ListContextsByOrg(ctx, org.ID)
	require.NoError(t, err)
	got := findByID(list, c.ID)
	require.NotNil(t, got)
	require.NotNil(t, got.Importance)
	assert.InDelta(t, 3.5, *got.Importance, 0.001)
}
