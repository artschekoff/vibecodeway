package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSnapshotResponse_WithEstimation(t *testing.T) {
	input := `{
	  "current_step": {"description":"Implementing auth","confidence":"high","from_event_ids":[]},
	  "status": "in_progress",
	  "section_operational": "## Current status\nIn progress",
	  "section_narrative": "## History\n...",
	  "section_engineering": "## Technical entities\n...",
	  "ai_next_steps_short": "Write auth tests",
	  "ai_next_steps_mid": "Deploy to staging",
	  "ai_next_steps_long": "GA release",
	  "estimated_hours": 4.5,
	  "estimation_confidence": "medium",
	  "estimation_basis": "2 requirements (2h each), 0.5h deducted for completed auth scaffold"
	}`

	sn, err := parseSnapshotResponse(input)
	require.NoError(t, err)
	require.NotNil(t, sn.EstimatedHours)
	assert.InDelta(t, 4.5, *sn.EstimatedHours, 0.001)
	assert.Equal(t, "medium", sn.EstimationConfidence)
	assert.Equal(t, "2 requirements (2h each), 0.5h deducted for completed auth scaffold", sn.EstimationBasis)
}

func TestParseSnapshotResponse_NoEstimation_ReturnsNilHours(t *testing.T) {
	input := `{
	  "current_step": {"description":"","confidence":"low","from_event_ids":[]},
	  "status": "not_started",
	  "section_operational": "",
	  "section_narrative": "",
	  "section_engineering": "",
	  "ai_next_steps_short": "",
	  "ai_next_steps_mid": "",
	  "ai_next_steps_long": ""
	}`

	sn, err := parseSnapshotResponse(input)
	require.NoError(t, err)
	assert.Nil(t, sn.EstimatedHours)
	assert.Empty(t, sn.EstimationConfidence)
}

func TestComputeCompletionDate(t *testing.T) {
	base := "2026-05-12T10:00:00Z"
	result := computeCompletionDate(base, 3.0)
	assert.Equal(t, "2026-05-12T13:00:00Z", result)
}

func TestComputeCompletionDate_ZeroHours(t *testing.T) {
	base := "2026-05-12T10:00:00Z"
	result := computeCompletionDate(base, 0)
	assert.Equal(t, "2026-05-12T10:00:00Z", result)
}
