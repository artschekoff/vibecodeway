package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/artschekoff/context-consolidator/internal/vcs"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- VCS Auth ---

// VCSLoginStart initiates the device-code OAuth flow for the given platform.
// Returns the user code and verification URL for the frontend to display.
func (a *App) VCSLoginStart(platform string) (*vcs.DeviceCodeInfo, error) {
	p := vcs.Platform(platform)
	if !vcs.IsPlatformEnabled(p) {
		return nil, fmt.Errorf("platform %q is not supported yet", platform)
	}
	return a.vcsSvc.LoginStart(a.ctx, p)
}

// VCSLoginComplete polls until the user finishes the device-code flow.
// Returns the username on success. This call blocks until auth is complete or the code expires.
func (a *App) VCSLoginComplete(platform, deviceCode string, interval int) (string, error) {
	p := vcs.Platform(platform)
	if !vcs.IsPlatformEnabled(p) {
		return "", fmt.Errorf("platform %q is not supported yet", platform)
	}
	result, err := a.vcsSvc.LoginComplete(a.ctx, p, deviceCode, interval)
	if err != nil {
		return "", err
	}
	return result.Username, nil
}

// GetVCSAuth returns the stored auth info for a platform. Returns empty strings if not authenticated.
func (a *App) GetVCSAuth(platform string) (*vcsstore.VCSAuth, error) {
	auth, err := a.vcsStore.Auth.GetAuth(a.ctx, platform)
	if err != nil {
		return &vcsstore.VCSAuth{Platform: platform}, nil // not authenticated, not an error
	}
	return auth, nil
}

// GetSupportedPlatforms returns all known VCS platforms with their enabled status and labels.
func (a *App) GetSupportedPlatforms() []vcsstore.VCSPlatformInfo {
	platforms := vcs.KnownPlatforms()
	result := make([]vcsstore.VCSPlatformInfo, len(platforms))
	for i, p := range platforms {
		result[i] = vcsstore.VCSPlatformInfo{
			Platform: string(p),
			Label:    vcs.PlatformLabel(p),
			Enabled:  vcs.IsPlatformEnabled(p),
		}
	}
	return result
}

// --- VCS Branches ---

// AddVCSBranch links a repository (and optionally a branch) to a context.
// ownerRepo must be in "owner/repo" format. branch may be empty to track all PRs for the repo.
// label is an optional human-readable tag (e.g. "api", "data") stored on the branch link.
func (a *App) AddVCSBranch(contextID, platform, ownerRepo, branch, label string) (*vcsstore.ContextVCSBranch, error) {
	p := vcs.Platform(platform)
	if !vcs.IsPlatformEnabled(p) {
		return nil, fmt.Errorf("platform %q is not supported yet", platform)
	}

	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("ownerRepo must be in 'owner/repo' format, got %q", ownerRepo)
	}
	owner, name := parts[0], parts[1]

	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, platform, owner, name)
	if err != nil {
		return nil, fmt.Errorf("ensuring repo: %w", err)
	}

	b, err := a.vcsStore.Branches.AddBranch(a.ctx, contextID, repo.ID, branch, label)
	if err != nil {
		return nil, fmt.Errorf("adding branch: %w", err)
	}
	b.Repo = repo
	return b, nil
}

// RemoveVCSBranch removes a branch link (and cascades to its PRs/comments).
func (a *App) RemoveVCSBranch(branchID string) error {
	return a.vcsStore.Branches.DeleteBranch(a.ctx, branchID)
}

// UpdateVCSBranchLabel sets or clears the label for an existing branch link.
func (a *App) UpdateVCSBranchLabel(branchID, label string) error {
	return a.vcsStore.Branches.UpdateBranchLabel(a.ctx, branchID, label)
}

// AddSpecificPR fetches a single PR from the VCS platform by number, resolves its head
// branch, creates a branch link for that branch, and stores the PR with its comments.
// Used when the user attaches a PR URL via the quick-attach hotkey.
func (a *App) AddSpecificPR(contextID, platform, ownerRepo string, prNumber int) error {
	p := vcs.Platform(platform)
	if !vcs.IsPlatformEnabled(p) {
		return fmt.Errorf("platform %q is not supported yet", platform)
	}

	runner, err := a.vcsSvc.RunnerFor(p)
	if err != nil {
		return fmt.Errorf("getting runner for %q: %w", platform, err)
	}

	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("ownerRepo must be in 'owner/repo' format, got %q", ownerRepo)
	}
	owner, name := parts[0], parts[1]

	pr, err := runner.GetPRByNumber(a.ctx, owner, name, prNumber)
	if err != nil {
		return fmt.Errorf("fetching PR #%d from %s/%s: %w", prNumber, owner, name, err)
	}

	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, platform, owner, name)
	if err != nil {
		return fmt.Errorf("ensuring repo: %w", err)
	}

	branch, err := a.vcsStore.Branches.AddBranch(a.ctx, contextID, repo.ID, pr.HeadBranch, "")
	if err != nil {
		return fmt.Errorf("adding branch link for PR #%d: %w", prNumber, err)
	}

	res, err := a.vcsStore.PullRequests.UpsertPR(a.ctx,
		repo.ID, branch.ID, pr.Number,
		pr.Title, pr.Author, pr.State, pr.ReviewStatus, pr.URL, pr.HeadBranch,
		pr.Mergeable, pr.CIStatus, pr.CreatedAt, pr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("storing PR #%d: %w", prNumber, err)
	}

	// Fetch and store comments (best-effort — don't fail if comments can't be fetched).
	comments, err := runner.GetPRComments(a.ctx, owner, name, prNumber)
	if err != nil {
		a.logger.Sugar().Warnf("fetching comments for PR #%d in %s/%s: %v", prNumber, owner, name, err)
		return nil
	}
	for _, c := range comments {
		if err := a.vcsStore.PullRequests.UpsertComment(a.ctx,
			res.ID, c.ExternalID, nil, c.CommentType, c.Author, c.Body,
			c.Path, c.Line, c.IsResolved, c.InReplyToID,
			c.CreatedAt, c.UpdatedAt,
		); err != nil {
			a.logger.Sugar().Warnf("storing comment for PR #%d: %v", prNumber, err)
		}
	}
	return nil
}

// ListVCSBranches returns all branches for a context with repo info joined.
func (a *App) ListVCSBranches(contextID string) ([]vcsstore.ContextVCSBranch, error) {
	return a.vcsStore.Branches.ListBranches(a.ctx, contextID)
}

// ListRepoBranches returns branch names for the given repository via the VCS platform API.
// ownerRepo must be in "owner/repo" format.
func (a *App) ListRepoBranches(platform, ownerRepo string) ([]string, error) {
	p := vcs.Platform(platform)
	runner, err := a.vcsSvc.RunnerFor(p)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("ownerRepo must be owner/repo")
	}
	return runner.ListRepoBranches(a.ctx, parts[0], parts[1])
}

// GetVCSPRsForContext returns stored PRs (with comments) for a context.
// PRs are refreshed via the existing RefreshContext flow.
func (a *App) GetVCSPRsForContext(contextID string) ([]vcsstore.PullRequest, error) {
	prs, err := a.vcsStore.PullRequests.ListPRsForContext(a.ctx, contextID)
	if err != nil {
		return nil, err
	}
	for i, pr := range prs {
		comments, err := a.vcsStore.PullRequests.ListCommentsForPR(a.ctx, pr.ID)
		if err != nil {
			a.logger.Sugar().Warnf("loading comments for PR %d: %v", pr.PRNumber, err)
			continue
		}
		prs[i].Comments = vcsstore.BuildCommentTree(comments)
	}
	return prs, nil
}

// --- Tracked Repos (for Pending Code Reviews) ---

// AddTrackedRepo adds a global tracked repo (used for Pending Code Reviews).
func (a *App) AddTrackedRepo(platform, ownerRepo string) (*vcsstore.VCSRepo, error) {
	p := vcs.Platform(platform)
	if !vcs.IsPlatformEnabled(p) {
		return nil, fmt.Errorf("platform %q is not supported yet", platform)
	}
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("ownerRepo must be in 'owner/repo' format, got %q", ownerRepo)
	}
	return a.vcsStore.Repos.EnsureRepo(a.ctx, platform, parts[0], parts[1])
}

// RemoveTrackedRepo removes a tracked repo and all its review requests.
func (a *App) RemoveTrackedRepo(repoID string) error {
	return a.vcsStore.Repos.DeleteRepo(a.ctx, repoID)
}

// ListTrackedRepos returns all tracked repos, optionally filtered by platform.
func (a *App) ListTrackedRepos(platform string) ([]vcsstore.VCSRepo, error) {
	return a.vcsStore.Repos.ListRepos(a.ctx, platform)
}

// UpdateTrackedRepoMeta updates the human-readable label and description for a tracked repo.
func (a *App) UpdateTrackedRepoMeta(repoID, label, description string) error {
	return a.vcsStore.Repos.UpdateRepoMeta(a.ctx, repoID, label, description)
}

// --- Pending Code Reviews ---

// GetPendingCodeReviews returns stored review requests across all tracked repos.
// Call RefreshPendingReviews first to fetch fresh data from GitHub.
func (a *App) GetPendingCodeReviews() ([]vcsstore.ReviewRequest, error) {
	return a.vcsStore.ReviewRequests.ListReviewRequests(a.ctx, "")
}

// RefreshPendingReviews fetches review requests for the authenticated user
// across all tracked repos for all enabled and authenticated platforms.
func (a *App) RefreshPendingReviews() error {
	return a.vcsSvc.SyncReviewRequests(a.ctx, vcs.PlatformGitHub)
}

// DismissReviewRequest hides a "waiting for author" item from the Code Reviews page.
func (a *App) DismissReviewRequest(id string) error {
	return a.vcsStore.ReviewRequests.DismissReviewRequest(a.ctx, id)
}

// --- Repo Local Path ---

// PickRepoDirectory opens a native directory picker and returns the selected path.
// Returns an empty string if the user cancels.
func (a *App) PickRepoDirectory() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select repository directory",
	})
	if err != nil {
		return "", fmt.Errorf("open directory dialog: %w", err)
	}
	return path, nil
}

// SetTrackedRepoPath associates a local filesystem directory with a tracked repo.
// Pass an empty string to clear the path.
func (a *App) SetTrackedRepoPath(repoID, localPath string) error {
	if localPath != "" {
		if _, err := os.Stat(localPath); err != nil {
			return fmt.Errorf("path does not exist: %w", err)
		}
	}
	return a.vcsStore.Repos.UpdateLocalPath(a.ctx, repoID, localPath)
}
