package main

import (
	"context"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// --- Workflow: user overrides ---

// SetUserWorkflowOverrides saves the user's overrides for status, comment, and next-step.
// Pass an empty string for any field to clear that override (revert to AI proposal).
func (a *App) SetUserWorkflowOverrides(contextID, status, comment, nextStep string) error {
	return a.db.SetUserOverride(context.Background(), contextID, status, comment, nextStep)
}

// GetWorkflowSummary returns the current AI proposal and user overrides for a context.
func (a *App) GetWorkflowSummary(contextID string) (*WorkflowSummary, error) {
	c, err := a.db.GetContext(context.Background(), contextID)
	if err != nil {
		return nil, err
	}
	return contextToWorkflowSummary(c), nil
}

// WorkflowSummary is the complete workflow state returned to the frontend.
type WorkflowSummary struct {
	// AI proposal (what the system computed last refresh)
	AIProposedStatus   string  `json:"aiProposedStatus"`
	AIProposedComment  *string `json:"aiProposedComment,omitempty"`
	AIProposedNextStep *string `json:"aiProposedNextStep,omitempty"`
	AIProposedAt       *string `json:"aiProposedAt,omitempty"`
	// User overrides (nil = user accepts AI proposal)
	UserStatusOverride    *string `json:"userStatusOverride,omitempty"`
	UserCommentOverride   *string `json:"userCommentOverride,omitempty"`
	UserNextStepOverride  *string `json:"userNextStepOverride,omitempty"`
	UserOverrideUpdatedAt *string `json:"userOverrideUpdatedAt,omitempty"`
	// Effective (resolved) values — what the UI should actually display
	EffectiveStatus   string `json:"effectiveStatus"`
	EffectiveComment  string `json:"effectiveComment"`
	EffectiveNextStep string `json:"effectiveNextStep"`
}

func contextToWorkflowSummary(c *appstore.Context) *WorkflowSummary {
	return &WorkflowSummary{
		AIProposedStatus:      c.AIProposedStatus,
		AIProposedComment:     c.AIProposedComment,
		AIProposedNextStep:    c.AIProposedNextStep,
		AIProposedAt:          c.AIProposedAt,
		UserStatusOverride:    c.UserStatusOverride,
		UserCommentOverride:   c.UserCommentOverride,
		UserNextStepOverride:  c.UserNextStepOverride,
		UserOverrideUpdatedAt: c.UserOverrideUpdatedAt,
		EffectiveStatus:       c.EffectiveStatus(),
		EffectiveComment:      c.EffectiveComment(),
		EffectiveNextStep:     c.EffectiveNextStep(),
	}
}

// GetWorkflowProposals returns the history of AI proposals for a context, newest first.
func (a *App) GetWorkflowProposals(contextID string) ([]appstore.WorkflowProposal, error) {
	return a.db.ListWorkflowProposals(context.Background(), contextID)
}

// ListWorkflowStatuses returns all available workflow status options.
func (a *App) ListWorkflowStatuses() ([]appstore.WorkflowStatus, error) {
	return a.db.ListWorkflowStatuses(context.Background())
}

// CreateWorkflowStatus adds a new custom workflow status.
// slug must match [a-z][a-z0-9_]* and be unique.
func (a *App) CreateWorkflowStatus(slug, label string) (*appstore.WorkflowStatus, error) {
	return a.db.CreateWorkflowStatus(context.Background(), slug, label)
}

// UpdateWorkflowStatus renames the display label of a status (slug is immutable).
func (a *App) UpdateWorkflowStatus(id, label string) (*appstore.WorkflowStatus, error) {
	return a.db.UpdateWorkflowStatus(context.Background(), id, label)
}

// DeleteWorkflowStatus removes a custom status and nulls out affected context overrides.
// Returns an error if the status is a built-in system status.
func (a *App) DeleteWorkflowStatus(id string) error {
	return a.db.DeleteWorkflowStatus(context.Background(), id)
}

// --- Open questions ---

func (a *App) GetOpenQuestions(contextID string) ([]appstore.OpenQuestion, error) {
	return a.db.ListOpenQuestions(context.Background(), contextID)
}

func (a *App) DismissOpenQuestion(questionID string) error {
	return a.db.DismissOpenQuestion(context.Background(), questionID)
}

func (a *App) AnswerOpenQuestion(questionID, answer string) error {
	return a.db.AnswerOpenQuestion(context.Background(), questionID, answer)
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
