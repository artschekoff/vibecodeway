package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraservice "github.com/artschekoff/vibecodeway/internal/jira/service"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	slackstore "github.com/artschekoff/vibecodeway/internal/slack/store"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
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

	key, summary, ticketKeys, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Equal(t, "CTX-A", key)
	assert.Contains(t, summary, "JIRA-1: Alpha summary")
	assert.Contains(t, summary, "JIRA-2: Beta summary")
	assert.ElementsMatch(t, []string{"JIRA-1", "JIRA-2"}, ticketKeys)
}

func TestContextMatchSignal_NoRelations_OnlyOwnIssues(t *testing.T) {
	a, jiraRepo := newAIMatchTestApp(t)
	org, err := a.db.CreateOrganization(a.ctx, "org")
	require.NoError(t, err)

	idA := seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-A", "JIRA-1", "Alpha summary")
	// Unrelated context — must NOT appear in the signal.
	seedContextWithIssue(t, a, jiraRepo, org.ID, "CTX-B", "JIRA-2", "Beta summary")

	_, summary, ticketKeys, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Contains(t, summary, "JIRA-1: Alpha summary")
	assert.NotContains(t, summary, "JIRA-2")
	assert.Equal(t, []string{"JIRA-1"}, ticketKeys)
}

func TestContextMatchSignal_IncludesParentTicketKeys(t *testing.T) {
	a, jiraRepo := newAIMatchTestApp(t)
	org, err := a.db.CreateOrganization(a.ctx, "org")
	require.NoError(t, err)

	c, err := a.db.CreateContext(a.ctx, "CTX-A", org.ID)
	require.NoError(t, err)
	// Subtask JIRA-10 whose parent is JIRA-1: branches named after the parent
	// (feat_JIRA-1_...) must match too.
	_, err = jiraRepo.UpsertIssue(a.ctx, c.ID, "JIRA-10", "Subtask", "", "", "", "", "", "JIRA-1", "")
	require.NoError(t, err)

	_, _, ticketKeys, err := a.contextMatchSignal(c.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"JIRA-10", "JIRA-1"}, ticketKeys)

	matchers := buildIdentifierMatchers(ticketKeys)
	hits := regexMatchBranches([]MatchedBranch{
		{Platform: "github", OwnerRepo: "o/r", Branch: "feat_JIRA-1_parent-work"},
		{Platform: "github", OwnerRepo: "o/r", Branch: "feat_JIRA-100_other"},
	}, matchers)
	require.Len(t, hits, 1)
	assert.Equal(t, "feat_JIRA-1_parent-work", hits[0].Branch)
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

	_, summary, ticketKeys, err := a.contextMatchSignal(idA)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(summary, "JIRA-1:"))
	assert.Equal(t, []string{"JIRA-1"}, ticketKeys)
}

func TestBuildIdentifierMatchers_wholeTokenCaseInsensitive(t *testing.T) {
	m := buildIdentifierMatchers([]string{"JH-1"})
	require.Len(t, m, 1)
	re := m[0]
	assert.True(t, re.MatchString("feat/JH-1-add-thumbs")) // delimited by / and -
	assert.True(t, re.MatchString("jh-1"))                 // case-insensitive, whole string
	assert.True(t, re.MatchString("bugfix/jh-1"))          // end of string boundary
	assert.False(t, re.MatchString("JH-100"))              // digit after -> not a token boundary
	assert.False(t, re.MatchString("myJH-1x"))             // letters around -> not a boundary
}

func TestBuildIdentifierMatchers_skipsEmptyAndDupes(t *testing.T) {
	m := buildIdentifierMatchers([]string{"JH-1", "", "  ", "jh-1", "JH-2"})
	assert.Len(t, m, 2) // JH-1 (dupe jh-1 dropped), JH-2
}

func TestRegexMatchBranches_selectsHits(t *testing.T) {
	cands := []MatchedBranch{
		{Platform: "github", OwnerRepo: "a/b", Branch: "feat/JH-1-x"},
		{Platform: "github", OwnerRepo: "a/b", Branch: "chore/cleanup"},
		{Platform: "github", OwnerRepo: "c/d", Branch: "JH-2"},
	}
	m := buildIdentifierMatchers([]string{"JH-1", "JH-2"})
	got := regexMatchBranches(cands, m)
	require.Len(t, got, 2)
	assert.Equal(t, "feat/JH-1-x", got[0].Branch)
	assert.Equal(t, "JH-2", got[1].Branch)
}

func TestRegexMatchBranches_noMatchersOrNoHits(t *testing.T) {
	cands := []MatchedBranch{{Branch: "chore/cleanup"}}
	assert.Nil(t, regexMatchBranches(cands, nil))
	assert.Nil(t, regexMatchBranches(cands, buildIdentifierMatchers([]string{"JH-9"})))
}
