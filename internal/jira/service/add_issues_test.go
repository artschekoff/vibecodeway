package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/jira"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeJira is a test double for JiraAPI.
type fakeJira struct {
	issues        map[string]*jira.IssueDetail
	searchResults map[string][]jira.SearchResult
	searchErr     map[string]error
}

func (f *fakeJira) ViewIssue(_ context.Context, key string) (*jira.IssueDetail, error) {
	if issue, ok := f.issues[key]; ok {
		return issue, nil
	}
	return nil, fmt.Errorf("issue not found: %s", key)
}

func (f *fakeJira) Search(_ context.Context, jql string) ([]jira.SearchResult, error) {
	if f.searchErr != nil {
		if err, ok := f.searchErr[jql]; ok {
			return nil, err
		}
	}
	if results, ok := f.searchResults[jql]; ok {
		return results, nil
	}
	return nil, nil
}

func (f *fakeJira) ListComments(_ context.Context, _ string) ([]jira.Comment, error) {
	return nil, nil
}

func (f *fakeJira) AuthStatus(_ context.Context) (string, error) {
	return "", nil
}

func (f *fakeJira) GetInstanceURL(_ context.Context) string {
	return ""
}

func newServiceWithJira(t *testing.T, j JiraAPI) (*ContextService, *appstore.Store) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	svc := NewContextService(s, j, jirastore.NewRepo(s.DB()), slackstore.NewRepo(s.DB()), nil, logger)
	return svc, s
}

func issueKeys(t *testing.T, svc *ContextService, contextID string) []string {
	t.Helper()
	issues, err := svc.jiraRepo.GetIssuesByContext(context.Background(), contextID, false)
	require.NoError(t, err)
	keys := make([]string, len(issues))
	for i, iss := range issues {
		keys[i] = iss.JiraKey
	}
	return keys
}

func makeDetail(key string) *jira.IssueDetail {
	return &jira.IssueDetail{
		Key:     key,
		Summary: "Summary of " + key,
		Status:  "To Do",
	}
}

func TestAddIssuesToContext_AddsSubtasksFromAPIField(t *testing.T) {
	parent := makeDetail("JH-100")
	parent.Subtasks = []jira.SubtaskRef{
		{Key: "JH-101", Summary: "subtask one"},
		{Key: "JH-102", Summary: "subtask two"},
	}
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-100": parent,
			"JH-101": makeDetail("JH-101"),
			"JH-102": makeDetail("JH-102"),
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-100"})
	require.NoError(t, err)
	assert.Equal(t, 3, added)
	assert.ElementsMatch(t, []string{"JH-100", "JH-101", "JH-102"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_AddsLinkedIssues(t *testing.T) {
	parent := makeDetail("JH-200")
	parent.Links = []jira.IssueLink{
		{TypeName: "Blocks", Direction: "inward", LinkedKey: "JH-201"},
	}
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-200": parent,
			"JH-201": makeDetail("JH-201"),
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-200"})
	require.NoError(t, err)
	assert.Equal(t, 2, added)
	assert.ElementsMatch(t, []string{"JH-200", "JH-201"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_AddsChildIssuesViaJQL(t *testing.T) {
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-300": makeDetail("JH-300"),
			"JH-301": makeDetail("JH-301"),
			"JH-302": makeDetail("JH-302"),
		},
		searchResults: map[string][]jira.SearchResult{
			`parent = "JH-300"`: {
				{Key: "JH-301"},
				{Key: "JH-302"},
			},
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-300"})
	require.NoError(t, err)
	assert.Equal(t, 3, added)
	assert.ElementsMatch(t, []string{"JH-300", "JH-301", "JH-302"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_DeduplicatesRefsSameKey(t *testing.T) {
	parent := makeDetail("JH-400")
	parent.Subtasks = []jira.SubtaskRef{{Key: "JH-401"}}
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-400": parent,
			"JH-401": makeDetail("JH-401"),
		},
		searchResults: map[string][]jira.SearchResult{
			`parent = "JH-400"`: {{Key: "JH-401"}},
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-400"})
	require.NoError(t, err)
	assert.Equal(t, 2, added, "JH-401 counted once even if present in multiple ref sources")
	assert.ElementsMatch(t, []string{"JH-400", "JH-401"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_PrimaryKeyNotDuplicated(t *testing.T) {
	issueA := makeDetail("JH-500")
	issueA.Links = []jira.IssueLink{{TypeName: "Relates", Direction: "outward", LinkedKey: "JH-501"}}
	issueB := makeDetail("JH-501")
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-500": issueA,
			"JH-501": issueB,
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-500", "JH-501"})
	require.NoError(t, err)
	assert.Equal(t, 2, added)
	assert.ElementsMatch(t, []string{"JH-500", "JH-501"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_SearchFailureTolerated(t *testing.T) {
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-600": makeDetail("JH-600"),
		},
		searchErr: map[string]error{
			`parent = "JH-600"`: fmt.Errorf("acli failed"),
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-600"})
	require.NoError(t, err)
	assert.Equal(t, 1, added, "primary issue added even when child search fails")
	assert.ElementsMatch(t, []string{"JH-600"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_RefViewIssueFailureTolerated(t *testing.T) {
	parent := makeDetail("JH-700")
	parent.Links = []jira.IssueLink{
		{TypeName: "Blocks", Direction: "inward", LinkedKey: "JH-701"},
	}
	j := &fakeJira{
		issues: map[string]*jira.IssueDetail{
			"JH-700": parent,
			// JH-701 intentionally missing — simulates acli failure
		},
	}
	svc, s := newServiceWithJira(t, j)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, []string{"JH-700"})
	require.NoError(t, err)
	assert.Equal(t, 1, added, "primary added even when linked issue fetch fails")
	assert.ElementsMatch(t, []string{"JH-700"}, issueKeys(t, svc, c.ID))
}

func TestAddIssuesToContext_EmptyKeysReturnsZero(t *testing.T) {
	svc, s := newServiceWithJira(t, &fakeJira{issues: map[string]*jira.IssueDetail{}})
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test", org.ID)
	require.NoError(t, err)

	added, err := svc.AddIssuesToContext(ctx, c.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, added)
}
