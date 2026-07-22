package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// InterpretContextUpdate parses userText and returns the AI-proposed list of
// actions (set_status, add_event, add_ticket, add_repo, add_relation) for the
// user to review and confirm. Used by the Quick Update UI.
func (a *App) InterpretContextUpdate(contextID, userText string) ([]appstore.ContextUpdateAction, error) {
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

// --- Quick Update (confirmed apply) ---

// ApplyActionResult reports the outcome of applying one confirmed Quick Update action.
type ApplyActionResult struct {
	Action  string `json:"action"`
	OK      bool   `json:"ok"`
	Message string `json:"message"` // error text or short success note
}

// validEventTypes is the canonical set of EventType* constants an AI-proposed
// add_event action may use. Anything outside this set is rejected rather than
// persisted raw, matching the strictness set_status already has via
// IsValidWorkflowStatus.
var validEventTypes = map[string]bool{
	appstore.EventTypeRequirementStated:      true,
	appstore.EventTypeRequirementConfirmed:   true,
	appstore.EventTypeRequirementRejected:    true,
	appstore.EventTypeOpenQuestionRaised:     true,
	appstore.EventTypeOpenQuestionAnswered:   true,
	appstore.EventTypeDecisionMade:           true,
	appstore.EventTypeBlockerIdentified:      true,
	appstore.EventTypeBlockerResolved:        true,
	appstore.EventTypeWorkCompleted:          true,
	appstore.EventTypeStatusUpdate:           true,
	appstore.EventTypeCurrentStepOverride:    true,
	appstore.EventTypeNextStepRecommendation: true,
	appstore.EventTypeUserNote:               true,
}

// ApplyContextUpdateActions applies user-confirmed Quick Update actions, one by one,
// reporting per-action success/failure. A failed action does not abort the rest.
func (a *App) ApplyContextUpdateActions(contextID string, actions []appstore.ContextUpdateAction) ([]ApplyActionResult, error) {
	out := make([]ApplyActionResult, 0, len(actions))
	for _, act := range actions {
		res := ApplyActionResult{Action: act.Action, OK: true}
		var err error
		switch act.Action {
		case "set_status":
			_, err = a.SetContextWorkflowStatus(contextID, act.StatusSlug, act.StatusComment)
		case "add_event":
			et := act.EventType
			if et == "" {
				et = appstore.EventTypeUserNote
			} else if !validEventTypes[et] {
				err = fmt.Errorf("unknown event type %q", et)
			}
			if err == nil {
				_, err = a.AddUserEvent(contextID, et, act.Summary)
			}
		case "add_ticket":
			if len(act.JiraKeys) == 0 {
				err = fmt.Errorf("no jira keys")
			} else {
				_, err = a.AddIssuesToContext(contextID, act.JiraKeys)
			}
		case "add_repo":
			if act.Platform == "" || act.OwnerRepo == "" || act.Branch == "" {
				err = fmt.Errorf("platform, ownerRepo, and branch are required")
			} else {
				_, err = a.AddVCSBranch(contextID, act.Platform, act.OwnerRepo, act.Branch, act.RepoLabel)
			}
		case "add_relation":
			err = a.addRelationByKey(contextID, act.TargetContextKey, act.RelationType)
		default:
			res.OK = false
			res.Message = "unsupported action: " + act.Action
			out = append(out, res)
			continue
		}
		if err != nil {
			res.OK = false
			res.Message = err.Error()
		}
		out = append(out, res)
		if a.logger != nil {
			a.logger.Sugar().Infow("applied quick update action",
				"contextId", contextID, "action", act.Action, "ok", res.OK)
		}
	}
	return out, nil
}

// validRelationTypes is the canonical set of context relation types accepted
// by the frontend (see ContextRelationType / RELATION_TYPE_LABELS in
// frontend/src/types/index.ts). AddContextRelation stores whatever string it
// is given with no validation, so an off-vocabulary value would "succeed" but
// render as a blank badge in the UI and never round-trip — reject it here
// before it reaches the store.
var validRelationTypes = map[string]bool{
	"blocks":           true,
	"is-blocked-by":    true,
	"relates-to":       true,
	"duplicates":       true,
	"is-duplicated-by": true,
}

// addRelationByKey resolves targetKey to a context id and stores a relation
// from contextID to that context. Returns an error if the key doesn't resolve
// or if relationType is not one of validRelationTypes.
func (a *App) addRelationByKey(contextID, targetKey, relationType string) error {
	if targetKey == "" {
		return fmt.Errorf("target context key is required")
	}
	if !validRelationTypes[relationType] {
		return fmt.Errorf("unknown relation type %q", relationType)
	}
	target, err := a.db.GetContextByKey(context.Background(), targetKey)
	if err != nil {
		return fmt.Errorf("resolving target context %q: %w", targetKey, err)
	}
	_, err = a.db.AddContextRelation(context.Background(), contextID, target.ID, relationType)
	return err
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

// --- Plans ---

// GeneratePlan generates and persists a new implementation plan for the given
// person assignment.
func (a *App) GeneratePlan(contextID, personAssignmentID string) (*appstore.ContextPlan, error) {
	return a.analysisSvc.GeneratePlan(a.ctx, contextID, personAssignmentID)
}

// ListContextPlans returns all previously generated plans for a person
// assignment, most recent first.
func (a *App) ListContextPlans(personAssignmentID string) ([]appstore.ContextPlan, error) {
	return a.db.ListContextPlansByAssignment(a.ctx, personAssignmentID)
}

// writePlanTempFile writes a plan's markdown body to a temp file named
// cc-plan-<planID>.md and returns the resulting path. Separated from
// OpenPlanInTerminal so it's testable without spawning a terminal.
func writePlanTempFile(planID, body string) (string, error) {
	path := filepath.Join(os.TempDir(), "cc-plan-"+planID+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("writing plan temp file: %w", err)
	}
	return path, nil
}

// OpenPlanInTerminal writes the given plan to a temp markdown file and opens
// it in the configured terminal/agent at the plan's context's repo path.
// It returns the path to the temp file so the UI can surface it to the user.
func (a *App) OpenPlanInTerminal(contextID, planID string) (string, error) {
	plan, err := a.db.GetContextPlan(a.ctx, planID)
	if err != nil {
		return "", fmt.Errorf("get context plan: %w", err)
	}

	path, err := writePlanTempFile(plan.ID, plan.Body)
	if err != nil {
		return "", err
	}

	branches, err := a.ListVCSBranches(contextID)
	if err != nil {
		return "", fmt.Errorf("list vcs branches: %w", err)
	}

	var repoPath string
	for _, b := range branches {
		if b.Repo != nil && b.Repo.LocalPath != "" {
			repoPath = b.Repo.LocalPath
			break
		}
	}
	if repoPath == "" {
		return "", fmt.Errorf("no linked repo with a local path found for context %q", contextID)
	}

	if err := a.openWithCheckout("", repoPath); err != nil {
		return "", err
	}

	return path, nil
}
