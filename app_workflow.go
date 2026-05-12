package main

import (
	"context"
	"time"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// --- Analysis ---

// AnalyzeContext runs the full AI analysis pipeline for a context:
// extracts structured events from new data, rebuilds the three-section snapshot,
// and generates next-step recommendations by time horizon.
// This is triggered manually by the user; it does NOT run on refresh.
func (a *App) AnalyzeContext(contextID string) (*appstore.ContextSnapshot, error) {
	return a.analysisSvc.AnalyzeContext(a.ctx, contextID)
}

// GenerateHandoffPacket calls the AI to build a structured business requirements packet
// from the full event log. The packet is saved to the database and returned.
// Each call produces a new packet — use GetLatestHandoffPacket to retrieve without re-generating.
func (a *App) GenerateHandoffPacket(contextID string) (*appstore.InferredRequirementsPacket, error) {
	return a.analysisSvc.GenerateHandoffPacket(a.ctx, contextID)
}

// GetLatestHandoffPacket returns the most recently generated handoff packet for a context
// without calling the AI. Returns nil if no packet has been generated yet.
func (a *App) GetLatestHandoffPacket(contextID string) (*appstore.InferredRequirementsPacket, error) {
	return a.db.GetLatestPacket(context.Background(), contextID)
}

// --- Snapshot ---

// GetContextSnapshot returns the current analysis snapshot for a context, or nil if not yet analyzed.
func (a *App) GetContextSnapshot(contextID string) (*appstore.ContextSnapshot, error) {
	return a.db.GetSnapshot(context.Background(), contextID)
}

// UpdateUserNextSteps saves user-written next-step notes by time horizon.
// These are always visible without re-analysis and are passed as hints on next Analyze run.
func (a *App) UpdateUserNextSteps(contextID, short, mid, long string) error {
	return a.db.UpdateUserNextSteps(context.Background(), contextID, short, mid, long)
}

// OverrideCurrentStep lets the user correct the AI's identified current step.
// The correction is persisted immediately and marks the snapshot stale.
func (a *App) OverrideCurrentStep(contextID, description string) error {
	if err := a.db.OverrideCurrentStep(context.Background(), contextID, description); err != nil {
		return err
	}
	// Also emit a current_step_override event so analysis picks it up.
	_, err := a.db.AddEvent(context.Background(), appstore.Event{
		ContextID:  contextID,
		Type:       appstore.EventTypeCurrentStepOverride,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		SourceType: appstore.EventSourceUser,
		Summary:    description,
	})
	return err
}

// GetEstimationHistory returns all estimation log entries for a context, newest first.
// Returns an empty slice when no analyses have produced an estimate yet.
func (a *App) GetEstimationHistory(contextID string) ([]appstore.EstimationLog, error) {
	return a.db.GetEstimationHistory(context.Background(), contextID)
}

// AddUserEvent appends a user-authored event to the event log and marks the snapshot stale.
// eventType must be one of: user_note, next_step_recommendation, open_question_answered,
// requirement_confirmed, requirement_rejected, current_step_override.
func (a *App) AddUserEvent(contextID, eventType, summary string) (*appstore.Event, error) {
	evt, err := a.db.AddEvent(context.Background(), appstore.Event{
		ContextID:  contextID,
		Type:       eventType,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		SourceType: appstore.EventSourceUser,
		Summary:    summary,
	})
	if err != nil {
		return nil, err
	}
	_ = a.db.MarkSnapshotStale(context.Background(), contextID)
	return evt, nil
}

// ListEvents returns all events for a context in chronological order.
func (a *App) ListEvents(contextID string) ([]appstore.Event, error) {
	return a.db.ListEvents(context.Background(), contextID)
}

// --- Workflow statuses (enum table — unchanged) ---

// ListWorkflowStatuses returns all available workflow status options.
func (a *App) ListWorkflowStatuses() ([]appstore.WorkflowStatus, error) {
	return a.db.ListWorkflowStatuses(context.Background())
}

// CreateWorkflowStatus adds a new custom workflow status.
func (a *App) CreateWorkflowStatus(slug, label string) (*appstore.WorkflowStatus, error) {
	return a.db.CreateWorkflowStatus(context.Background(), slug, label)
}

// UpdateWorkflowStatus renames the display label of a status.
func (a *App) UpdateWorkflowStatus(id, label string) (*appstore.WorkflowStatus, error) {
	return a.db.UpdateWorkflowStatus(context.Background(), id, label)
}

// DeleteWorkflowStatus removes a custom status.
func (a *App) DeleteWorkflowStatus(id string) error {
	return a.db.DeleteWorkflowStatus(context.Background(), id)
}

// --- VCS Repo Meta ---

func (a *App) UpdateVCSRepoMeta(repoID, label, description string) error {
	repo, err := a.vcsStore.Repos.GetRepo(context.Background(), repoID)
	if err != nil {
		return err
	}
	return a.vcsStore.Repos.UpdateRepoMeta(context.Background(), repoID, label, description, repo.OnCheckoutCommand)
}

func (a *App) ListVCSRepos(platform string) ([]vcsstore.VCSRepo, error) {
	return a.vcsStore.Repos.ListRepos(context.Background(), platform)
}
