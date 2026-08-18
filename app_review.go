package main

import (
	"fmt"
)

// reviewsOrgName is the dedicated organization that holds auto-created PR-review
// contexts, keeping them out of the user's real work orgs.
const reviewsOrgName = "Code Reviews"

// ReviewPRResult is returned to the frontend after starting a PR review.
type ReviewPRResult struct {
	ContextID   string `json:"contextId"`
	ContextKey  string `json:"contextKey"`
	Instruction string `json:"instruction"`
}

// ensureReviewsOrg returns the id of the "Code Reviews" organization, creating it
// if it does not exist yet.
func (a *App) ensureReviewsOrg() (string, error) {
	orgs, err := a.db.ListOrganizations(a.ctx)
	if err != nil {
		return "", err
	}
	for _, o := range orgs {
		if o.Name == reviewsOrgName {
			return o.ID, nil
		}
	}
	org, err := a.db.CreateOrganization(a.ctx, reviewsOrgName)
	if err != nil {
		return "", err
	}
	return org.ID, nil
}

// ensurePRReviewContext find-or-creates a context for the given PR (keyed
// "<owner>/<name>#<prNumber>" in the Code Reviews org) and links the PR's head
// branch. Idempotent: re-runs reuse the context and do not duplicate the branch
// (AddVCSBranch upserts). No terminal is opened.
func (a *App) ensurePRReviewContext(repoID string, prNumber int, headBranch, prTitle string) (contextID, contextKey string, err error) {
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoID)
	if err != nil {
		return "", "", fmt.Errorf("repo not found: %w", err)
	}
	key := fmt.Sprintf("%s/%s#%d", repo.Owner, repo.Name, prNumber)

	ctxRow, gerr := a.db.GetContextByKey(a.ctx, key)
	if gerr != nil || ctxRow == nil {
		orgID, oerr := a.ensureReviewsOrg()
		if oerr != nil {
			return "", "", oerr
		}
		ctxRow, err = a.db.CreateContext(a.ctx, key, orgID)
		if err != nil {
			return "", "", fmt.Errorf("create review context: %w", err)
		}
	}

	if headBranch != "" {
		if _, aerr := a.AddVCSBranch(ctxRow.ID, repo.Platform, repo.Owner+"/"+repo.Name, headBranch, prTitle, true); aerr != nil {
			a.logger.Sugar().Warnf("ReviewPRWithAI: add branch failed: %s", aerr.Error())
		}
	}
	return ctxRow.ID, ctxRow.Key, nil
}

// ReviewPRWithAI ensures a review context for the PR, links + checks out its
// branch as a worktree, opens the agent/terminal, and returns the review
// instruction plus the context to open in the editor.
func (a *App) ReviewPRWithAI(repoID string, prNumber int, headBranch, prTitle string) (*ReviewPRResult, error) {
	contextID, contextKey, err := a.ensurePRReviewContext(repoID, prNumber, headBranch, prTitle)
	if err != nil {
		return nil, err
	}
	instruction, err := a.analysisSvc.BuildReviewTask(a.ctx, contextID, prNumber, prTitle)
	if err != nil {
		return nil, err
	}
	firstWorktree, err := a.ensureContextWorkspaces(contextID)
	if err != nil {
		return nil, err
	}
	if firstWorktree == "" {
		return nil, fmt.Errorf("no local path for this repo; set one in the Repositories tab")
	}
	if err := a.openWithCheckout("", firstWorktree); err != nil {
		return nil, err
	}
	return &ReviewPRResult{ContextID: contextID, ContextKey: contextKey, Instruction: instruction}, nil
}
