package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestPA is a helper that creates the minimum required parent records
// (org + context + person_assignment) so version tests can satisfy FKs.
func createTestPA(t *testing.T, s *Store, paID string) {
	t.Helper()
	ctx := context.Background()

	// Create an org first (contexts.org_id is NOT NULL)
	orgID := "org-" + paID
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES (?, 'Test Org', 0, datetime('now'), datetime('now'))`,
		orgID,
	)
	require.NoError(t, err)

	// Create a minimal context
	ctxID := "ctx-" + paID
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES (?, ?, 'active', ?, datetime('now'), datetime('now'))`,
		ctxID, "key-"+paID, orgID,
	)
	require.NoError(t, err)

	// Create a minimal person_assignment
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO context_person_assignments (id, context_id, name, body, updated_at) VALUES (?, ?, 'Test Person', '', datetime('now'))`,
		paID, ctxID,
	)
	require.NoError(t, err)
}

func TestAssignmentVersion_CreateAndList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	paID := "pa-ver-1"
	createTestPA(t, s, paID)

	v1, err := s.CreateAssignmentVersion(ctx, paID, "body v1", "prompt v1", true)
	require.NoError(t, err)
	assert.True(t, v1.IsActive)
	assert.Equal(t, "body v1", v1.Body)
	assert.Equal(t, "prompt v1", v1.LLMPrompt)

	v2, err := s.CreateAssignmentVersion(ctx, paID, "body v2", "prompt v2", true)
	require.NoError(t, err)
	assert.True(t, v2.IsActive)

	versions, err := s.ListAssignmentVersions(ctx, paID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	activeCount := 0
	for _, v := range versions {
		if v.IsActive {
			activeCount++
		}
	}
	assert.Equal(t, 1, activeCount, "only one version should be active")
}

func TestAssignmentVersion_SetActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	paID := "pa-ver-2"
	createTestPA(t, s, paID)

	v1, err := s.CreateAssignmentVersion(ctx, paID, "body v1", "", true)
	require.NoError(t, err)
	v2, err := s.CreateAssignmentVersion(ctx, paID, "body v2", "", true)
	require.NoError(t, err)

	err = s.SetActiveAssignmentVersion(ctx, paID, v1.ID)
	require.NoError(t, err)

	versions, err := s.ListAssignmentVersions(ctx, paID)
	require.NoError(t, err)
	for _, v := range versions {
		if v.ID == v1.ID {
			assert.True(t, v.IsActive, "v1 should be active")
		}
		if v.ID == v2.ID {
			assert.False(t, v.IsActive, "v2 should be inactive")
		}
	}
}

func TestAssignmentVersion_GetActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	paID := "pa-ver-3"
	createTestPA(t, s, paID)

	// No versions yet
	ver, err := s.GetActiveAssignmentVersion(ctx, paID)
	require.NoError(t, err)
	assert.Nil(t, ver, "should return nil when no active version")

	created, err := s.CreateAssignmentVersion(ctx, paID, "body", "prompt", true)
	require.NoError(t, err)

	ver, err = s.GetActiveAssignmentVersion(ctx, paID)
	require.NoError(t, err)
	require.NotNil(t, ver)
	assert.Equal(t, created.ID, ver.ID)
	assert.True(t, ver.IsActive)
	assert.Equal(t, "body", ver.Body)
}
