package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddContextRelation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "project-alpha", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "project-beta", org.ID)
	require.NoError(t, err)

	rel, err := s.AddContextRelation(ctx, c1.ID, c2.ID, "blocks")
	require.NoError(t, err)
	assert.NotEmpty(t, rel.ID)
	assert.Equal(t, c1.ID, rel.ContextID)
	assert.Equal(t, c2.ID, rel.TargetID)
	assert.Equal(t, "project-beta", rel.TargetKey)
	assert.Equal(t, "blocks", rel.RelationType)
	assert.NotEmpty(t, rel.CreatedAt)
}

func TestAddContextRelation_DuplicateErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, _ := s.CreateOrganization(ctx, "test-org")
	c1, _ := s.CreateContext(ctx, "alpha", org.ID)
	c2, _ := s.CreateContext(ctx, "beta", org.ID)
	_, err := s.AddContextRelation(ctx, c1.ID, c2.ID, "blocks")
	require.NoError(t, err)

	_, err = s.AddContextRelation(ctx, c1.ID, c2.ID, "blocks")
	assert.Error(t, err, "expected error on duplicate relation")
}

func TestListContextRelations(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, _ := s.CreateOrganization(ctx, "test-org")
	c1, _ := s.CreateContext(ctx, "alpha", org.ID)
	c2, _ := s.CreateContext(ctx, "beta", org.ID)
	c3, _ := s.CreateContext(ctx, "gamma", org.ID)
	_, _ = s.AddContextRelation(ctx, c1.ID, c2.ID, "blocks")
	_, _ = s.AddContextRelation(ctx, c1.ID, c3.ID, "relates-to")

	rels, err := s.ListContextRelations(ctx, c1.ID)
	require.NoError(t, err)
	assert.Len(t, rels, 2)

	// Relations are directional — c2 has none originating from it
	rels2, err := s.ListContextRelations(ctx, c2.ID)
	require.NoError(t, err)
	assert.Len(t, rels2, 0)
}

func TestRemoveContextRelation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, _ := s.CreateOrganization(ctx, "test-org")
	c1, _ := s.CreateContext(ctx, "alpha", org.ID)
	c2, _ := s.CreateContext(ctx, "beta", org.ID)
	rel, _ := s.AddContextRelation(ctx, c1.ID, c2.ID, "blocks")

	err := s.RemoveContextRelation(ctx, rel.ID)
	require.NoError(t, err)

	rels, _ := s.ListContextRelations(ctx, c1.ID)
	assert.Len(t, rels, 0)
}

func TestRemoveContextRelation_NotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.RemoveContextRelation(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListContextRelations_ReturnsAllForContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	src, err := s.CreateContext(ctx, "auth-service", org.ID)
	require.NoError(t, err)
	db, err := s.CreateContext(ctx, "db-migration", org.ID)
	require.NoError(t, err)
	other, err := s.CreateContext(ctx, "unrelated", org.ID)
	require.NoError(t, err)

	_, err = s.AddContextRelation(ctx, src.ID, db.ID, "blocked-by")
	require.NoError(t, err)
	_, err = s.AddContextRelation(ctx, src.ID, other.ID, "relates-to")
	require.NoError(t, err)

	rels, err := s.ListContextRelations(ctx, src.ID)
	require.NoError(t, err)
	require.Len(t, rels, 2)
	assert.Equal(t, "db-migration", rels[0].TargetKey)
	assert.Equal(t, "blocked-by", rels[0].RelationType)
	assert.Equal(t, "relates-to", rels[1].RelationType)

	// a context with no outgoing relations returns empty, not error
	none, err := s.ListContextRelations(ctx, db.ID)
	require.NoError(t, err)
	assert.Empty(t, none)
}
