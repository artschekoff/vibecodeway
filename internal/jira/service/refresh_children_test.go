package service

import (
	"context"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/jira"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestJiraProviderRefresh_DiscoversNewChildren verifies that Refresh picks up child
// issues (both API subtasks and next-gen "parent = KEY" children) that appeared in
// Jira after the parent was first added — mirroring AddIssuesToContext's discovery.
func TestJiraProviderRefresh_DiscoversNewChildren(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	jRepo := jirastore.NewRepo(s.DB())
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	// Parent already stored (added earlier); children JH-501/JH-502 are brand new.
	_, err = jRepo.UpsertIssue(ctx, c.ID, "JH-500", "parent", "To Do", "", "", "", "", "", "")
	require.NoError(t, err)

	parent := makeDetail("JH-500")
	parent.Subtasks = []jira.SubtaskRef{{Key: "JH-501", Summary: "api subtask"}}
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-500": parent,
			"JH-501": makeDetail("JH-501"),
			"JH-502": makeDetail("JH-502"),
		},
		searchResults: map[string][]jira.SearchResult{
			`parent = "JH-500"`: {{Key: "JH-502"}},
		},
	}

	p := NewJiraProvider(jRepo, j, logger, nil)
	_, err = p.Refresh(ctx, c.ID)
	require.NoError(t, err)

	keys := make([]string, 0)
	issues, err := jRepo.GetIssuesByContext(ctx, c.ID, false)
	require.NoError(t, err)
	for _, iss := range issues {
		keys = append(keys, iss.JiraKey)
	}
	assert.ElementsMatch(t, []string{"JH-500", "JH-501", "JH-502"}, keys)
}
