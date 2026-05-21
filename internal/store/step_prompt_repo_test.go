package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepPrompt_RoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	step, err := s.AppendChainStep(ctx, ReasoningChainStep{
		EntityType:  ChainEntityPersonAssignment,
		EntityID:    "pa-test-1",
		SourceType:  "initial",
		InputState:  "",
		OutputState: "output body",
		IsCurrent:   true,
		LLMPrompt:   "You are an analyst. Generate a task assignment for Alice.",
	})
	require.NoError(t, err)
	assert.Equal(t, "You are an analyst. Generate a task assignment for Alice.", step.LLMPrompt)

	steps, err := s.ListChainSteps(ctx, ChainEntityPersonAssignment, "pa-test-1")
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "You are an analyst. Generate a task assignment for Alice.", steps[0].LLMPrompt)
}

func TestStepPrompt_EmptyWhenNotSet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.AppendChainStep(ctx, ReasoningChainStep{
		EntityType:  ChainEntityPersonAssignment,
		EntityID:    "pa-test-2",
		SourceType:  "initial",
		InputState:  "",
		OutputState: "output",
		IsCurrent:   true,
	})
	require.NoError(t, err)

	steps, err := s.ListChainSteps(ctx, ChainEntityPersonAssignment, "pa-test-2")
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "", steps[0].LLMPrompt, "empty LLMPrompt must round-trip as empty string")
}
