package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

func TestDedupeMatches(t *testing.T) {
	in := []MatchedBranch{
		{Platform: "github", OwnerRepo: "a/b", Branch: "feat/x"},
		{Platform: "github", OwnerRepo: "a/b", Branch: "feat/x"}, // dup
		{Platform: "github", OwnerRepo: "a/b", Branch: "main"},   // default
	}
	out := dedupeMatches(in)
	require.Len(t, out, 1)
	assert.Equal(t, "feat/x", out[0].Branch)
}

// newAIMatchTestApp builds an App backed by a temp SQLite store and a minimal
// ContextService (no live Jira client / providers) so contextMatchSignal can be
// exercised end-to-end. Returns the App and the jira Repo for seeding issues.
func newAIMatchTestApp(t *testing.T) (*App, *jirastore.Repo) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	jiraRepo := jirastore.NewRepo(s.DB())
	svc := jiraservice.NewContextService(s, nil, jiraRepo, slackstore.NewRepo(s.DB()), nil, newTestLogger())
	a := &App{db: s, svc: svc, ctx: context.Background(), logger: newTestLogger()}
	return a, jiraRepo
}

// seedContextWithIssue creates a context under org and attaches one Jira issue.
func seedContextWithIssue(t *testing.T, a *App, jiraRepo *jirastore.Repo, orgID, key, jiraKey, summary string) string {
	t.Helper()
	c, err := a.db.CreateContext(a.ctx, key, orgID)
	require.NoError(t, err)
	_, err = jiraRepo.UpsertIssue(a.ctx, c.ID, jiraKey, summary, "", "", "", "", "", "", "")
	require.NoError(t, err)
	return c.ID
}

func TestContextMatchSignal_AggregatesRelatedContexts(t *testing.T) {
	a, jiraRepo := newAIMatchTestApp(t)
	org, err := a.db.CreateOrganization(a.ctx, "org")
	require.NoError(t, err)

	idA := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-A", "JIRA-1", "Alpha summary")
	idB := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-B", "JIRA-2", "Beta summary")
	_, err = a.db.AddContextRelation(a.ctx, idA, idB, "relates-to")
	require.NoError(t, err)

	key, summary, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Equal(t, "CTX-A", key)
	assert.Contains(t, summary, "JIRA-1: Alpha summary")
	assert.Contains(t, summary, "JIRA-2: Beta summary")
}

func TestContextMatchSignal_NoRelations_OnlyOwnIssues(t *testing.T) {
	a, jiraRepo := newAIMatchTestApp(t)
	org, err := a.db.CreateOrganization(a.ctx, "org")
	require.NoError(t, err)

	idA := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-A", "JIRA-1", "Alpha summary")
	// Unrelated context — must NOT appear in the signal.
	seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-B", "JIRA-2", "Beta summary")

	_, summary, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Contains(t, summary, "JIRA-1: Alpha summary")
	assert.NotContains(t, summary, "JIRA-2")
}

func TestContextMatchSignal_DedupesSharedJiraKey(t *testing.T) {
	a, jiraRepo := newAIMatchTestApp(t)
	org, err := a.db.CreateOrganization(a.ctx, "org")
	require.NoError(t, err)

	// Both contexts reference the SAME Jira key — it must appear once.
	idA := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-A", "JIRA-1", "Shared summary")
	idB := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-B", "JIRA-1", "Shared summary")
	_, err = a.db.AddContextRelation(a.ctx, idA, idB, "relates-to")
	require.NoError(t, err)

	_, summary, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(summary, "JIRA-1:"))
}
