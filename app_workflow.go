package main

import (
	"context"
	"fmt"
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

// InterpretContextUpdate parses userText and returns the AI-suggested action
// (add_event or set_status) to apply to the context. Used by the Quick Update UI.
func (a *App) InterpretContextUpdate(contextID, userText string) (*appstore.ContextUpdateAction, error) {
	return a.analysisSvc.InterpretContextUpdate(a.ctx, contextID, userText)
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

// SetContextWorkflowStatus manually overrides the workflow status for a context
// and records the change in the status log.
func (a *App) SetContextWorkflowStatus(contextID, slug, comment string) (*appstore.ContextSnapshot, error) {
	ctx := context.Background()
	ok, err := a.db.IsValidWorkflowStatus(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("validating status: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("unknown status %q", slug)
	}
	if err := a.db.SetSnapshotStatus(ctx, contextID, slug); err != nil {
		return nil, err
	}
	prev, err := a.db.GetLatestStatusLogEntry(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get latest status log entry: %w", err)
	}
	var prevID *string
	if prev != nil {
		prevID = &prev.ID
	}
	if _, err := a.db.AppendStatusLog(ctx, contextID, slug, comment, "user", prevID); err != nil {
		return nil, err
	}
	return a.db.GetSnapshot(ctx, contextID)
}

// GetStatusLog returns the full status log for a context in chronological order.
func (a *App) GetStatusLog(contextID string) ([]appstore.StatusLogEntry, error) {
	return a.db.ListStatusLog(context.Background(), contextID)
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

// DeleteUserEvent removes a user-authored event and marks the snapshot stale.
func (a *App) DeleteUserEvent(contextID, eventID string) error {
	if err := a.db.DeleteEvent(context.Background(), eventID); err != nil {
		return err
	}
	_ = a.db.MarkSnapshotStale(context.Background(), contextID)
	return nil
}

// UpdateUserEvent changes the type and summary of a user-authored event and marks the snapshot stale.
func (a *App) UpdateUserEvent(contextID, eventID, eventType, summary string) (*appstore.Event, error) {
	evt, err := a.db.UpdateEvent(context.Background(), eventID, eventType, summary)
	if err != nil {
		return nil, err
	}
	_ = a.db.MarkSnapshotStale(context.Background(), contextID)
	return evt, nil
}

// --- Person Assignments ---

// GetPersonAssignments returns all person assignment records for a context.
func (a *App) GetPersonAssignments(contextID string) ([]appstore.PersonAssignment, error) {
	return a.db.ListPersonAssignments(context.Background(), contextID)
}

// GeneratePersonAssignments runs the AI pipeline to produce per-person technical assignment records.
// Replaces all existing assignments for the context.
func (a *App) GeneratePersonAssignments(contextID string) ([]appstore.PersonAssignment, error) {
	return a.analysisSvc.GeneratePersonAssignments(a.ctx, contextID)
}

// RefinePersonAssignment refines a single person's assignment body using the provided instruction.
func (a *App) RefinePersonAssignment(contextID, personID, instruction string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.RefinePersonAssignment(a.ctx, contextID, personID, instruction)
}

// SavePersonAssignment persists a manually edited body for a person assignment.
func (a *App) SavePersonAssignment(personID, body string) error {
	return a.db.UpdatePersonAssignmentBody(context.Background(), personID, body)
}

// DeletePersonAssignment removes a person assignment record.
func (a *App) DeletePersonAssignment(personID string) error {
	return a.db.DeletePersonAssignment(context.Background(), personID)
}

// GetContextParticipants returns sorted unique Jira assignees and comment authors for a context.
func (a *App) GetContextParticipants(contextID string) ([]string, error) {
	return a.analysisSvc.GetContextParticipants(a.ctx, contextID)
}

// GenerateSinglePersonAssignment generates a focused, implementation-ready assignment for one person.
// customPrompt may be empty. The record is created or updated for (contextID, personName).
func (a *App) GenerateSinglePersonAssignment(contextID, personName, customPrompt string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.GenerateSinglePersonAssignment(a.ctx, contextID, personName, customPrompt)
}

// SearchMessages returns context messages whose title or body contains query.
// Used by the @mention search in the instruction input. Returns up to 20 matches.
func (a *App) SearchMessages(contextID, query string) ([]appstore.Message, error) {
	if query == "" {
		return nil, nil
	}
	return a.db.SearchMessages(context.Background(), contextID, query)
}

// GetPendingMessages returns messages that arrived after this assignment's analyzed_through watermark.
func (a *App) GetPendingMessages(personAssignmentID string) ([]appstore.Message, error) {
	return a.analysisSvc.GetPendingMessages(a.ctx, personAssignmentID)
}

// MarkAssignmentsOutdated checks all assignments for a context against new messages
// and sets is_outdated=true on those with pending content. No AI call.
func (a *App) MarkAssignmentsOutdated(contextID string) (map[string]int, error) {
	return a.analysisSvc.MarkAssignmentsOutdated(a.ctx, contextID)
}

// RegeneratePersonAssignment incorporates all pending messages into the assignment via AI.
// Creates a new step and promotes it to is_current. Clears is_outdated.
func (a *App) RegeneratePersonAssignment(personAssignmentID string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.RegeneratePersonAssignment(a.ctx, personAssignmentID)
}

// GetPersonAssignmentSteps returns the full reasoning chain for a PersonAssignment.
func (a *App) GetPersonAssignmentSteps(personAssignmentID string) ([]appstore.ReasoningChainStep, error) {
	return a.db.ListChainSteps(context.Background(), appstore.ChainEntityPersonAssignment, personAssignmentID)
}

// SetCurrentAssignmentStep promotes the given step to is_current and syncs PersonAssignment.body.
func (a *App) SetCurrentAssignmentStep(personAssignmentID, stepID string) (*appstore.PersonAssignment, error) {
	outputState, err := a.db.SetChainCurrentStep(context.Background(), appstore.ChainEntityPersonAssignment, personAssignmentID, stepID)
	if err != nil {
		return nil, err
	}
	if err := a.db.UpdatePersonAssignmentBody(context.Background(), personAssignmentID, outputState); err != nil {
		return nil, err
	}
	return a.db.GetPersonAssignment(context.Background(), personAssignmentID)
}

// RebuildAssignmentChainFrom re-runs the reasoning chain from fromStepIndex onward.
// overrideContent replaces that step's message_content when non-empty.
func (a *App) RebuildAssignmentChainFrom(personAssignmentID string, fromStepIndex int, overrideContent string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.RebuildAssignmentChainFrom(a.ctx, personAssignmentID, fromStepIndex, overrideContent)
}

// GenerateEventsFromMessages extracts events from the message layer since the last event.
func (a *App) GenerateEventsFromMessages(contextID string) ([]appstore.Event, error) {
	return a.analysisSvc.GenerateEventsFromMessages(a.ctx, contextID)
}

// PreviewAssignmentStep runs the AI for a step without saving. Used by the propagation wizard.
func (a *App) PreviewAssignmentStep(personAssignmentID string, stepIndex int, customInputState string) (string, error) {
	return a.analysisSvc.PreviewAssignmentStep(a.ctx, personAssignmentID, stepIndex, customInputState)
}

// CommitAssignmentStep saves a wizard-confirmed step output to the chain.
func (a *App) CommitAssignmentStep(personAssignmentID string, stepIndex int, inputState, outputState string) (*appstore.ReasoningChainStep, error) {
	return a.analysisSvc.CommitAssignmentStep(a.ctx, personAssignmentID, stepIndex, inputState, outputState)
}

// RunSingleAssignmentStep previews the AI output for one step without saving.
func (a *App) RunSingleAssignmentStep(personAssignmentID string, stepIndex int, customInputState string) (string, error) {
	return a.analysisSvc.RunSingleAssignmentStep(a.ctx, personAssignmentID, stepIndex, customInputState)
}

// CommitStepOutput saves a confirmed step output and truncates downstream stale steps.
func (a *App) CommitStepOutput(personAssignmentID string, stepIndex int, inputState, outputState string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.CommitStepOutput(a.ctx, personAssignmentID, stepIndex, inputState, outputState)
}

// SaveAssignmentOutput saves a manually edited output body as a new active version.
func (a *App) SaveAssignmentOutput(personAssignmentID, body string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.SaveAssignmentOutput(a.ctx, personAssignmentID, body)
}

// RunAssignmentStepWithCustomPrompt runs the AI with customPrompt verbatim for preview.
// Does not write to the DB.
func (a *App) RunAssignmentStepWithCustomPrompt(personAssignmentID string, stepIndex int, customPrompt string) (string, error) {
	return a.analysisSvc.RunAssignmentStepWithCustomPrompt(a.ctx, personAssignmentID, stepIndex, customPrompt)
}

// CommitAssignmentVersionFromCustomPrompt saves a new step with a custom prompt and output.
// When the step is the last in the chain (isCurrent), it also creates a new active version
// and syncs PersonAssignment.body. Returns the updated PersonAssignment.
func (a *App) CommitAssignmentVersionFromCustomPrompt(personAssignmentID string, stepIndex int, customPrompt, outputState string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.CommitAssignmentVersionFromCustomPrompt(a.ctx, personAssignmentID, stepIndex, customPrompt, outputState)
}

// ListAssignmentVersions returns all versions for a person assignment, newest first.
func (a *App) ListAssignmentVersions(personAssignmentID string) ([]appstore.PersonAssignmentVersion, error) {
	return a.analysisSvc.ListAssignmentVersions(a.ctx, personAssignmentID)
}

// SetActiveAssignmentVersion activates the given version and syncs PersonAssignment.body.
func (a *App) SetActiveAssignmentVersion(personAssignmentID, versionID string) (*appstore.PersonAssignment, error) {
	if err := a.db.SetActiveAssignmentVersion(a.ctx, personAssignmentID, versionID); err != nil {
		return nil, err
	}
	ver, err := a.db.GetActiveAssignmentVersion(a.ctx, personAssignmentID)
	if err != nil {
		return nil, err
	}
	if ver == nil {
		return nil, fmt.Errorf("active version not found after activating %s", versionID)
	}
	if err := a.db.UpdatePersonAssignmentBody(a.ctx, personAssignmentID, ver.Body); err != nil {
		return nil, err
	}
	return a.db.GetPersonAssignment(a.ctx, personAssignmentID)
}

// --- Workflow statuses (enum table — unchanged) ---

// ListWorkflowStatuses returns all available workflow status options.
func (a *App) ListWorkflowStatuses() ([]appstore.WorkflowStatus, error) {
	return a.db.ListWorkflowStatuses(context.Background())
}

// CreateWorkflowStatus adds a new custom workflow status.
func (a *App) CreateWorkflowStatus(slug, label, aiHint, color string) (*appstore.WorkflowStatus, error) {
	return a.db.CreateWorkflowStatus(context.Background(), slug, label, aiHint, color)
}

// UpdateWorkflowStatus updates the display label, ai_hint, and color of a status.
func (a *App) UpdateWorkflowStatus(id, label, aiHint, color string) (*appstore.WorkflowStatus, error) {
	return a.db.UpdateWorkflowStatus(context.Background(), id, label, aiHint, color)
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
