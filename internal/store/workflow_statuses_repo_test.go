package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkflowStatuses_ReturnsSeedDefaults(t *testing.T) {
	s := openTestStore(t)
	statuses, err := s.ListWorkflowStatuses(context.Background())
	require.NoError(t, err)
	assert.Len(t, statuses, 6)
	slugs := make([]string, len(statuses))
	for i, st := range statuses {
		slugs[i] = st.Slug
	}
	assert.ElementsMatch(t, []string{"not_started", "in_progress", "scoping", "needs_attention", "ready", "blocked"}, slugs)
}

func TestListWorkflowStatuses_SystemFirstThenCustom(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	_, err := s.CreateWorkflowStatus(ctx, "custom_one", "Custom One", "", "")
	require.NoError(t, err)

	statuses, err := s.ListWorkflowStatuses(ctx)
	require.NoError(t, err)
	// System statuses come first
	for i := 0; i < 6; i++ {
		assert.True(t, statuses[i].IsSystem, "expected system status at index %d", i)
	}
	assert.False(t, statuses[6].IsSystem)
	assert.Equal(t, "custom_one", statuses[6].Slug)
}

func TestCreateWorkflowStatus_RejectsInvalidSlug(t *testing.T) {
	s := openTestStore(t)
	_, err := s.CreateWorkflowStatus(context.Background(), "has space", "Has Space", "", "")
	assert.Error(t, err)

	_, err = s.CreateWorkflowStatus(context.Background(), "HasUpper", "Has Upper", "", "")
	assert.Error(t, err)

	_, err = s.CreateWorkflowStatus(context.Background(), "_leading", "Leading underscore", "", "")
	assert.Error(t, err)
}

func TestCreateWorkflowStatus_RejectsDuplicateSlug(t *testing.T) {
	s := openTestStore(t)
	_, err := s.CreateWorkflowStatus(context.Background(), "not_started", "Duplicate", "", "")
	assert.Error(t, err)
}

func TestCreateWorkflowStatus_AcceptsValidSlug(t *testing.T) {
	s := openTestStore(t)
	st, err := s.CreateWorkflowStatus(context.Background(), "custom_blocked", "Blocked", "", "")
	require.NoError(t, err)
	assert.Equal(t, "custom_blocked", st.Slug)
	assert.Equal(t, "Blocked", st.Label)
	assert.False(t, st.IsSystem)
	assert.NotEmpty(t, st.ID)
}

func TestUpdateWorkflowStatus_ChangesLabel(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	st, err := s.CreateWorkflowStatus(ctx, "paused", "Paused", "", "")
	require.NoError(t, err)

	updated, err := s.UpdateWorkflowStatus(ctx, st.ID, "On Hold", "", "")
	require.NoError(t, err)
	assert.Equal(t, "On Hold", updated.Label)
	assert.Equal(t, "paused", updated.Slug) // slug unchanged
}

func TestUpdateWorkflowStatus_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.UpdateWorkflowStatus(context.Background(), "non-existent-id", "Label", "", "")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDeleteWorkflowStatus_RejectsSystemStatus(t *testing.T) {
	s := openTestStore(t)
	statuses, err := s.ListWorkflowStatuses(context.Background())
	require.NoError(t, err)
	var systemID string
	for _, st := range statuses {
		if st.IsSystem {
			systemID = st.ID
			break
		}
	}
	require.NotEmpty(t, systemID)
	err = s.DeleteWorkflowStatus(context.Background(), systemID)
	assert.ErrorIs(t, err, ErrSystemStatus)
}

func TestDeleteWorkflowStatus_RemovesCustomStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	st, err := s.CreateWorkflowStatus(ctx, "custom_done", "Custom Done", "", "")
	require.NoError(t, err)

	require.NoError(t, s.DeleteWorkflowStatus(ctx, st.ID))

	statuses, err := s.ListWorkflowStatuses(ctx)
	require.NoError(t, err)
	for _, existing := range statuses {
		assert.NotEqual(t, st.ID, existing.ID)
	}
}

func TestDeleteWorkflowStatus_ResetsSnapshotStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	st, err := s.CreateWorkflowStatus(ctx, "custom_done", "Custom Done", "", "")
	require.NoError(t, err)

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-context", org.ID)
	require.NoError(t, err)

	// Give the context a snapshot with the custom status.
	require.NoError(t, s.UpsertSnapshot(ctx, &ContextSnapshot{
		ContextID: c.ID,
		Status:    "custom_done",
	}))

	require.NoError(t, s.DeleteWorkflowStatus(ctx, st.ID))

	sn, err := s.GetSnapshot(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, sn)
	assert.Equal(t, "not_started", sn.Status)
	assert.True(t, sn.IsStale)
}

func TestIsValidWorkflowStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	valid, err := s.IsValidWorkflowStatus(ctx, "in_progress")
	require.NoError(t, err)
	assert.True(t, valid)

	valid, err = s.IsValidWorkflowStatus(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, valid)
}
