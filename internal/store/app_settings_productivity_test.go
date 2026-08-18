package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppSettingsProductivityDefaults(t *testing.T) {
	s := openTestStore(t)
	got, err := s.GetAppSettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 8.0, got.WorkdayHours)
	assert.Equal(t, 2.0, got.TaskHours)
}

func TestAppSettingsProductivityRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	current, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	current.WorkdayHours = 6.5
	current.TaskHours = 1.5
	require.NoError(t, s.SetAppSettings(ctx, *current))

	got, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6.5, got.WorkdayHours)
	assert.Equal(t, 1.5, got.TaskHours)
}

func TestTaskCompletionsTableExists(t *testing.T) {
	s := openTestStore(t)
	assertTableExists(t, s, "task_completions")
}
