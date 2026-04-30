// Package workflow computes workflow status and generates next-step suggestions for contexts.
package workflow

import (
	"context"
	"fmt"
	"strings"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// Compute derives the AI-proposed workflow status for a context.
// It does NOT write to the DB — the caller persists via store.SetAIProposal.
// Returns (status, comment, error).
func Compute(ctx context.Context, contextID string, s *appstore.Store, vs *vcsstore.Store) (string, string, error) {
	branches, err := vs.Branches.ListBranches(ctx, contextID)
	if err != nil {
		return "", "", fmt.Errorf("workflow.Compute list branches: %w", err)
	}
	if len(branches) == 0 {
		return "not_started", "", nil
	}

	// needs_attention check first — always authoritative.
	attnStatus, attnComment, err := checkAttentionConditions(ctx, branches, vs)
	if err != nil {
		return "", "", fmt.Errorf("workflow.Compute attention check: %w", err)
	}
	if attnStatus != "" {
		return attnStatus, attnComment, nil
	}

	hasOpen, err := s.HasOpenQuestions(ctx, contextID)
	if err != nil {
		return "", "", fmt.Errorf("workflow.Compute has open questions: %w", err)
	}
	if hasOpen {
		return "scoping", "Open questions need to be answered before work can begin.", nil
	}

	return "in_progress", "", nil
}

func checkAttentionConditions(ctx context.Context, branches []vcsstore.ContextVCSBranch, vs *vcsstore.Store) (string, string, error) {
	var issues []string
	for _, b := range branches {
		prs, err := vs.PullRequests.ListPRsForBranch(ctx, b.ID)
		if err != nil {
			return "", "", fmt.Errorf("checkAttentionConditions list prs for branch %s: %w", b.ID, err)
		}
		for _, pr := range prs {
			switch {
			case pr.Mergeable == "CONFLICTING":
				issues = append(issues, fmt.Sprintf("PR #%d has a merge conflict", pr.PRNumber))
			case pr.ReviewStatus == "changes_requested":
				issues = append(issues, fmt.Sprintf("PR #%d has changes requested", pr.PRNumber))
			case pr.CIStatus == "FAILURE":
				issues = append(issues, fmt.Sprintf("PR #%d CI checks are failing", pr.PRNumber))
			}
		}
	}
	if len(issues) == 0 {
		return "", "", nil
	}
	return "needs_attention", strings.Join(issues, "; "), nil
}
