package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// If no DB record exists for GitHub, falls back to querying the gh CLI so that existing gh auth
// survives a db-reinit without requiring the user to go through the OAuth device flow again.
func (a *App) GetVCSAuth(platform string) (*vcsstore.VCSAuth, error) {
	auth, err := a.vcsStore.Auth.GetAuth(a.ctx, platform)
	if err == nil {
		return auth, nil
	}
	// Bootstrap from gh CLI when the DB record is missing.
	if vcs.Platform(platform) == vcs.PlatformGitHub {
		if username, ghErr := a.vcsSvc.GetAuthUser(a.ctx, vcs.PlatformGitHub); ghErr == nil && username != "" {
			if _, saveErr := a.vcsStore.Auth.UpsertAuth(a.ctx, platform, username, ""); saveErr != nil {
				a.logger.Sugar().Warnf("GetVCSAuth: bootstrap save failed: %v", saveErr)
			}
			return &vcsstore.VCSAuth{Platform: platform, Username: username}, nil
		}
	}
	return &vcsstore.VCSAuth{Platform: platform}, nil // not authenticated, not an error
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

// ListRepoLocalPaths returns all local filesystem paths registered for a tracked repo.
func (a *App) ListRepoLocalPaths(repoID string) ([]vcsstore.VCSRepoPath, error) {
	return a.vcsStore.RepoPaths.ListForRepo(a.ctx, repoID)
}

// AddRepoLocalPath opens a directory picker and adds the selected path to the tracked repo.
// Returns nil without error if the user cancels the picker.
func (a *App) AddRepoLocalPath(repoID string) (*vcsstore.VCSRepoPath, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select repository directory",
	})
	if err != nil {
		return nil, fmt.Errorf("open directory dialog: %w", err)
	}
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	return a.vcsStore.RepoPaths.Add(a.ctx, repoID, path)
}

// RemoveRepoLocalPath deletes a local filesystem path entry by ID.
func (a *App) RemoveRepoLocalPath(pathID string) error {
	return a.vcsStore.RepoPaths.Delete(a.ctx, pathID)
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

// CheckoutBranch switches to branch in localPath, then opens the configured terminal there.
// Deprecated: use CheckoutInWorkspace for worktree-based isolation.
func (a *App) CheckoutBranch(localPath, branch string) error {
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("local path not accessible: %w", err)
	}
	if err := vcs.GitCheckout(localPath, branch); err != nil {
		return err
	}
	return a.openTerminalAt(localPath)
}

// openTerminalAt opens the configured terminal app at path.
// A missing or unconfigured terminal is silently ignored.
func (a *App) openTerminalAt(path string) error {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		a.logger.Sugar().Warnf("openTerminalAt: failed to load settings: %v", err)
		return nil
	}
	if settings.TerminalAppPath == "" {
		return nil
	}
	cmd := exec.Command("open", "-a", settings.TerminalAppPath, path)
	if err := cmd.Start(); err != nil {
		a.logger.Sugar().Warnf("open terminal failed: app=%s err=%v", settings.TerminalAppPath, err)
	}
	return nil
}

// ensureWorktreesIgnored appends ".worktrees" to baseDir/.gitignore if not already present.
func ensureWorktreesIgnored(baseDir string) error {
	gitignorePath := filepath.Join(baseDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(content), ".worktrees") {
		return nil
	}
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	prefix := ""
	if len(content) > 0 && content[len(content)-1] != '\n' {
		prefix = "\n"
	}
	_, err = fmt.Fprintf(f, "%s.worktrees\n", prefix)
	return err
}

// --- Workspace (git worktree) management ---

// FindWorkspaceForBranch returns an existing worktree workspace for the given repo and branch.
// Returns nil, nil if no workspace exists.
func (a *App) FindWorkspaceForBranch(repoID, branch string) (*vcsstore.VCSWorkspace, error) {
	return a.vcsStore.Workspaces.FindByBranch(a.ctx, repoID, branch)
}

// CheckoutInWorkspace creates a git worktree for branch at the given repo path and opens terminal.
// The worktree is created at <repoPath>/.worktrees/<branch-safe-name>.
func (a *App) CheckoutInWorkspace(repoPathID, branch string) error {
	repoPath, err := a.vcsStore.RepoPaths.Get(a.ctx, repoPathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	baseDir := repoPath.Path
	if _, err := os.Stat(baseDir); err != nil {
		return fmt.Errorf("local path not accessible: %w", err)
	}
	if err := ensureWorktreesIgnored(baseDir); err != nil {
		a.logger.Sugar().Warnf("CheckoutInWorkspace: .gitignore update failed: %v", err)
	}
	worktreeName := vcs.BranchToWorktreeName(branch)
	worktreePath := filepath.Join(baseDir, ".worktrees", worktreeName)
	if err := vcs.GitWorktreeAdd(baseDir, worktreePath, branch); err != nil {
		return err
	}
	w := &vcsstore.VCSWorkspace{
		RepoPathID:   repoPathID,
		RepoID:       repoPath.RepoID,
		Branch:       branch,
		WorktreePath: worktreePath,
		Status:       "active",
	}
	if err := a.vcsStore.Workspaces.Create(a.ctx, w); err != nil {
		return fmt.Errorf("storing workspace record: %w", err)
	}
	return a.openTerminalAt(worktreePath)
}

// OpenWorkspaceTerminal opens the configured terminal at the workspace's worktree path.
func (a *App) OpenWorkspaceTerminal(workspaceID string) error {
	w, err := a.vcsStore.Workspaces.Get(a.ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	return a.openTerminalAt(w.WorktreePath)
}

// ListWorkspaces returns all workspaces for a specific repo path.
func (a *App) ListWorkspaces(repoPathID string) ([]vcsstore.VCSWorkspace, error) {
	return a.vcsStore.Workspaces.ListByPath(a.ctx, repoPathID)
}

// ListAllWorkspaces returns all known workspaces across all repos and paths.
func (a *App) ListAllWorkspaces() ([]vcsstore.VCSWorkspace, error) {
	return a.vcsStore.Workspaces.ListAll(a.ctx)
}

// RefreshWorkspacesForRepo reconciles DB workspace records with what git reports on disk
// for all local paths of the repo. Removes stale DB entries; adds newly discovered ones.
func (a *App) RefreshWorkspacesForRepo(repoID string) error {
	paths, err := a.vcsStore.RepoPaths.ListForRepo(a.ctx, repoID)
	if err != nil {
		return err
	}
	for _, rp := range paths {
		worktrees, err := vcs.GitWorktreeList(rp.Path)
		if err != nil {
			a.logger.Sugar().Warnf("RefreshWorkspacesForRepo: git worktree list failed for %s: %v", rp.Path, err)
			continue
		}
		existing, err := a.vcsStore.Workspaces.ListByPath(a.ctx, rp.ID)
		if err != nil {
			return err
		}
		onDisk := make(map[string]string) // worktreePath -> branch
		for _, wt := range worktrees {
			if wt.Branch != "" {
				onDisk[wt.Path] = wt.Branch
			}
		}
		for _, w := range existing {
			if _, ok := onDisk[w.WorktreePath]; !ok {
				_ = a.vcsStore.Workspaces.Delete(a.ctx, w.ID)
			}
		}
		existingPaths := make(map[string]bool)
		for _, w := range existing {
			existingPaths[w.WorktreePath] = true
		}
		for wPath, branch := range onDisk {
			if wPath == rp.Path || existingPaths[wPath] {
				continue
			}
			w := &vcsstore.VCSWorkspace{
				RepoPathID:   rp.ID,
				RepoID:       repoID,
				Branch:       branch,
				WorktreePath: wPath,
				Status:       "active",
			}
			_ = a.vcsStore.Workspaces.Create(a.ctx, w)
		}
	}
	return nil
}

// DeleteWorkspace removes the git worktree from disk and deletes its DB record.
// If git worktree remove fails (e.g. already deleted), the DB record is still removed.
func (a *App) DeleteWorkspace(workspaceID string) error {
	w, err := a.vcsStore.Workspaces.Get(a.ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if rmErr := vcs.GitWorktreeRemove(w.WorktreePath); rmErr != nil {
		a.logger.Sugar().Warnf("DeleteWorkspace: git worktree remove failed (continuing): %v", rmErr)
	}
	return a.vcsStore.Workspaces.Delete(a.ctx, workspaceID)
}

// MarkWorkspaceShouldDelete sets a workspace's status to "should_delete".
func (a *App) MarkWorkspaceShouldDelete(workspaceID string) error {
	return a.vcsStore.Workspaces.UpdateStatus(a.ctx, workspaceID, "should_delete")
}
