package analysis

import (
	"context"
	"fmt"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"go.uber.org/zap"
)

// ChainProcessor runs the AI for one reasoning step and returns the new output state.
// inputState = current state (previous step's output, or "" for step 0).
// step = the step definition (message_content, source_type, user_instruction).
// Implementations must also set step.PromptVersion, step.EventsSnapshot, and
// optionally step.CompressedSummary before returning.
type ChainProcessor interface {
	RunStep(ctx context.Context, inputState string, step *appstore.ReasoningChainStep) (string, error)
}

// QualityJudge scores a completed step output against domain-specific criteria.
// Returns score 0.0–1.0 and a short rationale string.
// Runs asynchronously (goroutine) so it never blocks the main step flow.
type QualityJudge interface {
	Score(ctx context.Context, step appstore.ReasoningChainStep) (float64, string, error)
}

// ReasoningChainService manages generic ordered reasoning chains.
// It is shared by PersonAssignmentService and ContextAnalysisService.
type ReasoningChainService struct {
	store  *appstore.Store
	logger *zap.Logger
}

func NewReasoningChainService(store *appstore.Store, logger *zap.Logger) *ReasoningChainService {
	return &ReasoningChainService{store: store, logger: logger}
}

// AppendStep runs the AI for a new step and saves it.
// processor.RunStep is expected to fill step.PromptVersion, step.EventsSnapshot,
// and step.CompressedSummary before returning.
// If judge is non-nil, quality scoring runs in a background goroutine — it never
// blocks the caller and updates the step record asynchronously.
func (s *ReasoningChainService) AppendStep(ctx context.Context, step appstore.ReasoningChainStep, processor ChainProcessor, judge QualityJudge) (*appstore.ReasoningChainStep, error) {
	outputState, err := processor.RunStep(ctx, step.InputState, &step)
	if err != nil {
		return nil, fmt.Errorf("reasoning chain run step: %w", err)
	}
	step.OutputState = outputState

	saved, err := s.store.AppendChainStep(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("save chain step: %w", err)
	}

	// Async quality scoring — never blocks the caller.
	if judge != nil {
		go func(savedStep appstore.ReasoningChainStep) {
			scoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			score, notes, judgeErr := judge.Score(scoreCtx, savedStep)
			if judgeErr != nil {
				s.logger.Warn("quality judge failed", zap.String("step_id", savedStep.ID), zap.Error(judgeErr))
				return
			}
			if updateErr := s.store.UpdateChainStepQuality(scoreCtx, savedStep.ID, score, notes); updateErr != nil {
				s.logger.Warn("save quality score failed", zap.String("step_id", savedStep.ID), zap.Error(updateErr))
			}
		}(*saved)
	}

	return saved, nil
}

// SetCurrentStep promotes stepID to is_current and returns the step's output_state.
func (s *ReasoningChainService) SetCurrentStep(ctx context.Context, entityType, entityID, stepID string) (string, error) {
	return s.store.SetChainCurrentStep(ctx, entityType, entityID, stepID)
}

// PreviewStep runs the AI for step stepIndex using customInputState as the input.
// Nothing is written to the DB — purely a preview for the UI propagation wizard.
func (s *ReasoningChainService) PreviewStep(ctx context.Context, entityType, entityID string, stepIndex int, customInputState string, processor ChainProcessor) (string, error) {
	steps, err := s.store.ListChainSteps(ctx, entityType, entityID)
	if err != nil {
		return "", fmt.Errorf("list chain steps: %w", err)
	}
	if stepIndex >= len(steps) {
		return "", fmt.Errorf("step %d out of range (chain has %d steps)", stepIndex, len(steps))
	}
	step := steps[stepIndex]
	step.InputState = customInputState // override with caller-provided input
	return processor.RunStep(ctx, customInputState, &step)
}

// CommitStep saves one confirmed step from the UI propagation wizard.
// Truncates from stepIndex onward, re-creates this step with the given inputState/outputState.
// If it was the last step, promotes it to is_current.
// judge may be nil to skip quality scoring for this step.
func (s *ReasoningChainService) CommitStep(ctx context.Context, entityType, entityID string, stepIndex int, inputState, outputState, llmPrompt string, judge QualityJudge) (*appstore.ReasoningChainStep, error) {
	steps, err := s.store.ListChainSteps(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	if stepIndex >= len(steps) {
		return nil, fmt.Errorf("step %d out of range", stepIndex)
	}

	original := steps[stepIndex]
	isLast := stepIndex == len(steps)-1

	if err := s.store.TruncateChainFrom(ctx, entityType, entityID, stepIndex); err != nil {
		return nil, fmt.Errorf("truncate chain: %w", err)
	}

	saved, err := s.store.AppendChainStep(ctx, appstore.ReasoningChainStep{
		EntityType:        entityType,
		EntityID:          entityID,
		SourceType:        original.SourceType,
		MessageID:         original.MessageID,
		MessageContent:    original.MessageContent,
		UserInstruction:   original.UserInstruction,
		EventsSnapshot:    original.EventsSnapshot,
		PromptVersion:     original.PromptVersion,
		CompressedSummary: original.CompressedSummary,
		InputState:        inputState,
		OutputState:       outputState,
		IsCurrent:         isLast,
		LLMPrompt:         llmPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("append committed step: %w", err)
	}

	// Async quality scoring for wizard-confirmed steps.
	if judge != nil {
		go func(ss appstore.ReasoningChainStep) {
			scoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			score, notes, scoreErr := judge.Score(scoreCtx, ss)
			if scoreErr != nil {
				s.logger.Warn("quality judge failed", zap.Error(scoreErr))
				return
			}
			_ = s.store.UpdateChainStepQuality(scoreCtx, ss.ID, score, notes)
		}(*saved)
	}

	return saved, nil
}

// RebuildFrom truncates the chain from stepIndex, re-runs each step via processor,
// cascading forward. The last step is promoted to is_current. overrideContent replaces
// stepIndex's message_content when non-empty. judge may be nil to skip quality scoring.
func (s *ReasoningChainService) RebuildFrom(ctx context.Context, entityType, entityID string, fromStepIndex int, overrideContent string, processor ChainProcessor, judge QualityJudge) error {
	steps, err := s.store.ListChainSteps(ctx, entityType, entityID)
	if err != nil {
		return fmt.Errorf("list steps: %w", err)
	}
	if fromStepIndex >= len(steps) {
		return fmt.Errorf("step %d out of range", fromStepIndex)
	}

	inputState := ""
	if fromStepIndex > 0 {
		inputState = steps[fromStepIndex-1].OutputState
	}
	toRerun := steps[fromStepIndex:]

	if err := s.store.TruncateChainFrom(ctx, entityType, entityID, fromStepIndex); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	var lastID string
	for i, step := range toRerun {
		if i == 0 && overrideContent != "" {
			step.MessageContent = overrideContent
		}
		step.InputState = inputState
		isLast := i == len(toRerun)-1

		output, runErr := processor.RunStep(ctx, inputState, &step)
		if runErr != nil {
			s.logger.Warn("rebuild chain: AI failed", zap.Int("step", fromStepIndex+i), zap.Error(runErr))
			output = inputState // carry forward unchanged on failure
		}

		rebuilt := appstore.ReasoningChainStep{
			EntityType:        entityType,
			EntityID:          entityID,
			SourceType:        step.SourceType,
			MessageID:         step.MessageID,
			MessageContent:    step.MessageContent,
			UserInstruction:   step.UserInstruction,
			InputState:        inputState,
			OutputState:       output,
			CompressedSummary: step.CompressedSummary,
			EventsSnapshot:    step.EventsSnapshot,
			PromptVersion:     step.PromptVersion,
			IsCurrent:         isLast,
		}
		saved, err := s.store.AppendChainStep(ctx, rebuilt)
		if err != nil {
			return fmt.Errorf("rebuild step %d: %w", fromStepIndex+i, err)
		}
		lastID = saved.ID
		if judge != nil {
			go func(ss appstore.ReasoningChainStep) {
				scoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				score, notes, scoreErr := judge.Score(scoreCtx, ss)
				if scoreErr != nil {
					s.logger.Warn("quality judge failed", zap.Error(scoreErr))
					return
				}
				_ = s.store.UpdateChainStepQuality(scoreCtx, ss.ID, score, notes)
			}(*saved)
		}
		inputState = output
	}

	if lastID != "" {
		_, _ = s.store.SetChainCurrentStep(ctx, entityType, entityID, lastID)
	}
	return nil
}
