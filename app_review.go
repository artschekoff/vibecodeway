package main

import (
	"fmt"
	"sort"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
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
// branch. On first creation it also attaches the PR itself (AddSpecificPR) so
// the context's PR panel works immediately. Idempotent: re-runs reuse the
// context and do not duplicate the branch (AddVCSBranch upserts). No terminal
// is opened.
func (a *App) ensurePRReviewContext(repoID string, prNumber int, headBranch, prTitle string) (contextID, contextKey string, err error) {
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoID)
	if err != nil {
		return "", "", fmt.Errorf("repo not found: %w", err)
	}
	key := fmt.Sprintf("%s/%s#%d", repo.Owner, repo.Name, prNumber)

	// A branch that is already being worked on (a worktree exists for it and a
	// context links it) is reviewed inside that context — no per-PR review
	// context gets created next to the work.
	var ctxRow *appstore.Context
	if cid := a.workContextForBranch(repoID, headBranch); cid != "" {
		ctxRow, _ = a.db.GetContext(a.ctx, cid)
	}
	if ctxRow == nil {
		ctxRow, _ = a.db.GetContextByKey(a.ctx, key)
	}
	if ctxRow == nil {
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
			a.logger.Sugar().Warnf("review context: add branch failed: %s", aerr.Error())
		}
	}
	// Fetch + link the PR when the context does not have it yet (also heals
	// contexts created before PRs were attached) — a network call per PR, so
	// skipped once linked; the background VCS sync keeps it fresh afterwards.
	// vcsSvc is nil in tests.
	if a.vcsSvc != nil && !a.contextHasPR(ctxRow.ID, prNumber) {
		if perr := a.AddSpecificPR(ctxRow.ID, repo.Platform, repo.Owner+"/"+repo.Name, prNumber); perr != nil {
			a.logger.Sugar().Warnf("review context: attach PR #%d failed: %s", prNumber, perr.Error())
		}
	}
	return ctxRow.ID, ctxRow.Key, nil
}

// workContextForBranch finds an active context that already works on branch:
// every tracked repo is checked (the PR's repo first) for a worktree of that
// branch, and the first such repo's branch link names the context. "" when
// nothing is checked out for the branch anywhere.
func (a *App) workContextForBranch(repoID, branch string) string {
	if branch == "" {
		return ""
	}
	repos, err := a.vcsStore.Repos.ListRepos(a.ctx, "")
	if err != nil {
		return ""
	}
	sort.SliceStable(repos, func(i, j int) bool { return repos[i].ID == repoID && repos[j].ID != repoID })
	for _, r := range repos {
		if ws, _ := a.FindWorkspaceForBranch(r.ID, branch); ws == nil {
			continue
		}
		links, lerr := a.vcsStore.Branches.ListAllBranchesForRepo(a.ctx, r.ID)
		if lerr != nil {
			continue
		}
		for _, l := range links {
			if l.Branch != branch {
				continue
			}
			if c, cerr := a.db.GetContext(a.ctx, l.ContextID); cerr == nil && c != nil && c.Status == "active" {
				return c.ID
			}
		}
	}
	return ""
}

// contextHasPR reports whether a PR with prNumber is already linked to the
// context through any of its branch links. Local DB reads only.
func (a *App) contextHasPR(contextID string, prNumber int) bool {
	branches, err := a.vcsStore.Branches.ListBranches(a.ctx, contextID)
	if err != nil {
		return false
	}
	for _, b := range branches {
		prs, perr := a.vcsStore.PullRequests.ListPRsForBranch(a.ctx, b.ID)
		if perr != nil {
			continue
		}
		for _, pr := range prs {
			if pr.PRNumber == prNumber {
				return true
			}
		}
	}
	return false
}

// ensureReviewContexts creates a per-PR review context for every pending
// review assigned to the user that does not have one yet, so artifacts can be
// attached to it before (or while) reviewing. Dismissed and already-reviewed
// requests are skipped; failures are logged per request, never fatal.
func (a *App) ensureReviewContexts() {
	reqs, err := a.vcsStore.ReviewRequests.ListReviewRequests(a.ctx, "")
	if err != nil {
		a.logger.Sugar().Warnf("ensureReviewContexts: list review requests failed: %s", err.Error())
		return
	}
	for _, r := range reqs {
		if r.Kind != "requested" || r.DismissedAt != nil || r.ReviewedAt != nil {
			continue
		}
		if _, _, cerr := a.ensurePRReviewContext(r.RepoID, r.PRNumber, r.HeadBranch, r.PRTitle); cerr != nil {
			a.logger.Sugar().Warnf("ensureReviewContexts: %s#%d: %s", r.RepoID, r.PRNumber, cerr.Error())
		}
	}
}

// ReviewPRWithAI resolves the context for the PR (an existing work context
// for its branch, else a per-PR review context), checks out just the PR's
// branch as a worktree (the on-checkout script runs when it is created),
// opens the agent/terminal there, and returns the review instruction plus
// the context to open in the editor. Other branches linked to the context
// are left alone.
func (a *App) ReviewPRWithAI(repoID string, prNumber int, headBranch, prTitle string) (*ReviewPRResult, error) {
	contextID, contextKey, err := a.ensurePRReviewContext(repoID, prNumber, headBranch, prTitle)
	if err != nil {
		return nil, err
	}
	instruction, err := a.analysisSvc.BuildReviewTask(a.ctx, contextID, prNumber, prTitle)
	if err != nil {
		return nil, err
	}
	worktree, err := a.ensureWorktreeForBranch(repoID, headBranch)
	if err != nil {
		return nil, err
	}
	if worktree == "" {
		return nil, fmt.Errorf("no local path for this repo; set one in the Repositories tab")
	}
	if err := a.openTerminalAt(worktree); err != nil {
		return nil, err
	}
	return &ReviewPRResult{ContextID: contextID, ContextKey: contextKey, Instruction: instruction}, nil
}
