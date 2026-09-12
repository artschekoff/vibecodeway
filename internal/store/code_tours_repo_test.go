package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleTour(contextID, repoPath string) *CodeTour {
	return &CodeTour{
		ContextID: contextID,
		RepoPath:  repoPath,
		Source:    "agent",
		Title:     "Add analytics dashboard",
		Steps: []TourStep{
			{
				File:   "src/x.ts",
				Anchor: "func computeDashboard(",
				What:   "Added the dashboard compute entry point",
				Why:    "Keeps aggregation out of the HTTP layer so it can be unit-tested",
			},
			{
				File:   "src/y.ts",
				Anchor: "export const Dashboard",
				What:   "Added the dashboard component",
				Why:    "Renders the computed stats",
			},
		},
		StepIndex: 0,
		Decisions: map[int]string{},
		CreatedAt: "2026-09-11T10:00:00Z",
	}
}

func TestUpsertAndGetCodeTourRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tour := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, tour))
	require.NotEmpty(t, tour.ID)

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tour.ID, got.ID)
	assert.Equal(t, "ctx-1", got.ContextID)
	assert.Equal(t, "/repo/a", got.RepoPath)
	assert.Equal(t, "agent", got.Source)
	assert.Equal(t, "Add analytics dashboard", got.Title)
	assert.Equal(t, 0, got.StepIndex)
	assert.Equal(t, "2026-09-11T10:00:00Z", got.CreatedAt)
	require.Len(t, got.Steps, 2)
	assert.Equal(t, "src/x.ts", got.Steps[0].File)
	assert.Equal(t, "func computeDashboard(", got.Steps[0].Anchor)
	assert.Equal(t, "Added the dashboard compute entry point", got.Steps[0].What)
	assert.Equal(t, "Keeps aggregation out of the HTTP layer so it can be unit-tested", got.Steps[0].Why)
	assert.Equal(t, "src/y.ts", got.Steps[1].File)
	assert.Empty(t, got.Decisions)
}

func TestUpsertCodeTourWithDecisionsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tour := sampleTour("ctx-1", "/repo/a")
	tour.Decisions = map[int]string{0: "staged", 1: "discarded"}
	tour.StepIndex = 1
	require.NoError(t, s.UpsertCodeTour(ctx, tour))

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.StepIndex)
	assert.Equal(t, map[int]string{0: "staged", 1: "discarded"}, got.Decisions)
}

func TestGetCodeTourAbsentReturnsNilNil(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	got, err := s.GetCodeTour(ctx, "no-such-context", "/repo/nope")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUpsertCodeTourReplacesPreviousTourForSamePair(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, first))
	firstID := first.ID

	second := sampleTour("ctx-1", "/repo/a")
	second.Title = "Second tour"
	second.Source = "generated"
	require.NoError(t, s.UpsertCodeTour(ctx, second))
	require.NotEqual(t, firstID, second.ID)

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, second.ID, got.ID)
	assert.Equal(t, "Second tour", got.Title)
	assert.Equal(t, "generated", got.Source)

	var count int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM code_tours WHERE context_id = ? AND repo_path = ?`,
		"ctx-1", "/repo/a",
	).Scan(&count))
	assert.Equal(t, 1, count, "only one row should remain for the pair")
}

func TestUpsertCodeTourDoesNotAffectOtherPair(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tourA := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, tourA))

	tourB := sampleTour("ctx-1", "/repo/b")
	tourB.Title = "Other repo tour"
	require.NoError(t, s.UpsertCodeTour(ctx, tourB))

	tourC := sampleTour("ctx-2", "/repo/a")
	tourC.Title = "Other context tour"
	require.NoError(t, s.UpsertCodeTour(ctx, tourC))

	gotA, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, gotA)
	assert.Equal(t, tourA.ID, gotA.ID)

	gotB, err := s.GetCodeTour(ctx, "ctx-1", "/repo/b")
	require.NoError(t, err)
	require.NotNil(t, gotB)
	assert.Equal(t, "Other repo tour", gotB.Title)

	gotC, err := s.GetCodeTour(ctx, "ctx-2", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, gotC)
	assert.Equal(t, "Other context tour", gotC.Title)
}

func TestSetCodeTourStep(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tour := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, tour))

	require.NoError(t, s.SetCodeTourStep(ctx, tour.ID, 1))

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.StepIndex)
}

func TestSetCodeTourDecision(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tour := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, tour))

	require.NoError(t, s.SetCodeTourDecision(ctx, tour.ID, 0, "staged"))
	require.NoError(t, s.SetCodeTourDecision(ctx, tour.ID, 1, "edited"))

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, map[int]string{0: "staged", 1: "edited"}, got.Decisions)
}

func TestSetCodeTourDecisionOverwritesExisting(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	tour := sampleTour("ctx-1", "/repo/a")
	require.NoError(t, s.UpsertCodeTour(ctx, tour))

	require.NoError(t, s.SetCodeTourDecision(ctx, tour.ID, 0, "staged"))
	require.NoError(t, s.SetCodeTourDecision(ctx, tour.ID, 0, "discarded"))

	got, err := s.GetCodeTour(ctx, "ctx-1", "/repo/a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, map[int]string{0: "discarded"}, got.Decisions)
}
