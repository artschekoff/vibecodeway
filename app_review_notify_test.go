package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnReviewNotifyEmitsReviewRequestedEvent(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}

	a.onReviewNotify(ctx, "repo-x", "juna-api", 88, "Fix flaky test", "octocat")

	rc, err := s.GetContextByKey(ctx, "juna-api — reviews") // em-dash U+2014
	require.NoError(t, err)
	require.NotNil(t, rc)

	evs, err := s.ListEvents(ctx, rc.ID)
	require.NoError(t, err)
	require.Len(t, evs, 1)

	assert.Equal(t, appstore.EventTypeReviewRequested, evs[0].Type)
	assert.Equal(t, appstore.EventSourceGitHub, evs[0].SourceType)
	assert.Equal(t, "juna-api#88", evs[0].SourceID)
	assert.Equal(t, "octocat", evs[0].Author)
	assert.Contains(t, evs[0].Summary, "PR #88")
}
