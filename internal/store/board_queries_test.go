package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListBoardBlockers_empty(t *testing.T) {
	db := openTestStore(t)
	blockers, err := db.ListBoardBlockers(context.Background())
	require.NoError(t, err)
	require.Empty(t, blockers)
}

func TestListBoardAssignments_empty(t *testing.T) {
	db := openTestStore(t)
	assignments, err := db.ListBoardAssignments(context.Background(), "alice")
	require.NoError(t, err)
	require.Empty(t, assignments)
}

func TestListBoardPRsByAuthor_empty(t *testing.T) {
	db := openTestStore(t)
	prs, err := db.ListBoardPRsByAuthor(context.Background(), "alice")
	require.NoError(t, err)
	require.Empty(t, prs)
}
