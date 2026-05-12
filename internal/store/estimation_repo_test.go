package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendAndGetEstimationLog(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org1")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "est-ctx", org.ID)
	require.NoError(t, err)

	hours1 := 1.0
	entry1 := EstimationLog{
		ContextID:           c.ID,
		TriggerType:         "initial",
		TriggerDescription:  "first analysis",
		EstimatedHoursAfter: &hours1,
		EstimationBasis:     "single requirement",
	}
	require.NoError(t, s.AppendEstimationLog(ctx, entry1))

	hours2 := 3.0
	delta := 2.0
	entry2 := EstimationLog{
		ContextID:            c.ID,
		TriggerType:          "re-analysis",
		TriggerDescription:   "new requirement added",
		DeltaHours:           &delta,
		EstimatedHoursBefore: &hours1,
		EstimatedHoursAfter:  &hours2,
		EstimationBasis:      "auth requirement added (+2h)",
	}
	require.NoError(t, s.AppendEstimationLog(ctx, entry2))

	log, err := s.GetEstimationHistory(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, log, 2)
	// Newest first
	assert.Equal(t, "re-analysis", log[0].TriggerType)
	assert.Equal(t, "initial", log[1].TriggerType)
	require.NotNil(t, log[0].DeltaHours)
	assert.InDelta(t, 2.0, *log[0].DeltaHours, 0.001)
}

func TestGetEstimationHistory_EmptyForNewContext(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "org1")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "new-ctx", org.ID)
	require.NoError(t, err)

	log, err := s.GetEstimationHistory(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, log)
}
