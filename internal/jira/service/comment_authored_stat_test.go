package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/jira"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// commentFakeJira embeds fakeJira and overrides ListComments to return a
// fixed set of comments, regardless of issue key.
type commentFakeJira struct {
	*fakeJira
	comments []jira.Comment
}

func (f *commentFakeJira) ListComments(_ context.Context, _ string) ([]jira.Comment, error) {
	return f.comments, nil
}

// setupCommentStatTest opens a fresh in-memory store, creates one context and
// one issue, and returns a JiraProvider wired to fj plus the issue slice
// syncComments expects.
func setupCommentStatTest(t *testing.T, fj JiraAPI) (*JiraProvider, string, []jirastore.Issue) {
	t.Helper()
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	repo := jirastore.NewRepo(s.DB())

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	upserted, err := repo.UpsertIssue(ctx, c.ID, "PROJ-1", "summary", "open", "", "task", "", "2024-01-01T00:00:00Z", "", "")
	require.NoError(t, err)
	issue := jirastore.Issue{ID: upserted.ID, JiraKey: "PROJ-1"}

	p := NewJiraProvider(repo, fj, logger, s)
	return p, c.ID, []jirastore.Issue{issue}
}

func TestSyncCommentsRecordsStatEventWhenAuthorMatchesIdentity(t *testing.T) {
	fj := &commentFakeJira{fakeJira: &fakeJira{}, comments: []jira.Comment{
		{ID: "c1", Author: "Jane Doe", Body: "looks good", Created: "2026-08-15T10:00:00Z"},
	}}
	p, contextID, issues := setupCommentStatTest(t, fj)

	var recorded []appstore.StatEvent
	p.WithRecordStatFunc(func(_ context.Context, e appstore.StatEvent) error {
		recorded = append(recorded, e)
		return nil
	})
	p.WithStatsIdentityFunc(func(context.Context) (string, error) { return "Jane Doe", nil })

	total, err := p.syncComments(context.Background(), contextID, issues)
	require.NoError(t, err)
	assert.Equal(t, 1, total, "comment is synced regardless of stat capture")

	require.Len(t, recorded, 1)
	assert.Equal(t, "comment_authored", recorded[0].Kind)
	assert.Equal(t, "c1", recorded[0].ExternalID)
	assert.Equal(t, "PROJ-1", recorded[0].JiraKey)
	assert.Equal(t, "2026-08-15T10:00:00Z", recorded[0].OccurredAt)
}

func TestSyncCommentsSkipsStatEventWhenIdentityEmpty(t *testing.T) {
	fj := &commentFakeJira{fakeJira: &fakeJira{}, comments: []jira.Comment{
		{ID: "c2", Author: "Jane Doe", Body: "lgtm", Created: "2026-08-15T10:00:00Z"},
	}}
	p, contextID, issues := setupCommentStatTest(t, fj)

	var recorded []appstore.StatEvent
	p.WithRecordStatFunc(func(_ context.Context, e appstore.StatEvent) error {
		recorded = append(recorded, e)
		return nil
	})
	// Empty identity: the user never configured statsAuthorName.
	p.WithStatsIdentityFunc(func(context.Context) (string, error) { return "", nil })

	total, err := p.syncComments(context.Background(), contextID, issues)
	require.NoError(t, err)
	assert.Equal(t, 1, total, "comment is still synced")
	assert.Empty(t, recorded, "capture is skipped silently when identity is empty")
}

func TestSyncCommentsSkipsStatEventWhenAuthorDoesNotMatchIdentity(t *testing.T) {
	fj := &commentFakeJira{fakeJira: &fakeJira{}, comments: []jira.Comment{
		{ID: "c3", Author: "Someone Else", Body: "not mine", Created: "2026-08-15T10:00:00Z"},
	}}
	p, contextID, issues := setupCommentStatTest(t, fj)

	var recorded []appstore.StatEvent
	p.WithRecordStatFunc(func(_ context.Context, e appstore.StatEvent) error {
		recorded = append(recorded, e)
		return nil
	})
	p.WithStatsIdentityFunc(func(context.Context) (string, error) { return "Jane Doe", nil })

	total, err := p.syncComments(context.Background(), contextID, issues)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Empty(t, recorded, "capture is skipped when the comment author isn't the configured identity")
}

func TestSyncCommentsNoOpWhenCallbacksNotWired(t *testing.T) {
	fj := &commentFakeJira{fakeJira: &fakeJira{}, comments: []jira.Comment{
		{ID: "c4", Author: "Jane Doe", Body: "hello", Created: "2026-08-15T10:00:00Z"},
	}}
	p, contextID, issues := setupCommentStatTest(t, fj)

	// Neither WithRecordStatFunc nor WithStatsIdentityFunc called: must not panic.
	total, err := p.syncComments(context.Background(), contextID, issues)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}
