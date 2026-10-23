package main

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewContextForRepoIsIdempotent(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	a := &App{db: s, ctx: ctx, logger: newTestLogger()}

	id1, err := a.reviewContextForRepo("repo-x", "acme-api")
	require.NoError(t, err)
	require.NotEmpty(t, id1)

	id2, err := a.reviewContextForRepo("repo-x", "acme-api")
	require.NoError(t, err)
	require.Equal(t, id1, id2) // second call returns the SAME context, not a duplicate

	c, err := s.GetContextByKey(ctx, "acme-api — reviews")
	require.NoError(t, err)
	assert.Equal(t, id1, c.ID)
}
