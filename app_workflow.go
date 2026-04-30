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
	return a.vcsStore.Repos.UpdateRepoMeta(context.Background(), repoID, label, description)
}

func (a *App) ListVCSRepos(platform string) ([]vcsstore.VCSRepo, error) {
	return a.vcsStore.Repos.ListRepos(context.Background(), platform)
}
