package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextPlanCRUD(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	org, err := store.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := store.CreateContext(ctx, "my-ctx", org.ID)
	require.NoError(t, err)
	cID := c.ID

	pa, err := store.UpsertPersonAssignment(ctx, cID, PersonAssignment{Name: "Alice"})
	require.NoError(t, err)
	paID := pa.ID

	p, err := store.CreateContextPlan(ctx, cID, paID, "# Plan\n- step")
	require.NoError(t, err)
	require.NotEmpty(t, p.ID)
	assert.Equal(t, "# Plan\n- step", p.Body)

	got, err := store.GetContextPlan(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)

	list, err := store.ListContextPlansByAssignment(ctx, paID)
	require.NoError(t, err)
	require.Len(t, list, 1)
}
