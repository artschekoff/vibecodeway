package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/artschekoff/vibecodeway/internal/repofiles"
	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
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
			if _, saveErr := a.vcsStore.Auth.UpsertAuth(a.ctx, platform, username, "", ""); saveErr != nil {
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
// autodetectPR=false links the branch without ever resolving its pull requests —
// what you want for a long-lived branch like main.
func (a *App) AddVCSBranch(contextID, platform, ownerRepo, branch, label string, autodetectPR bool) (*vcsstore.ContextVCSBranch, error) {
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

	b, err := a.vcsStore.Branches.AddBranch(a.ctx, contextID, repo.ID, branch, label, autodetectPR)
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

	branch, err := a.vcsStore.Branches.AddBranch(a.ctx, contextID, repo.ID, pr.HeadBranch, "", true)
	if err != nil {
		return fmt.Errorf("adding branch link for PR #%d: %w", prNumber, err)
	}

	res, err := a.vcsStore.PullRequests.UpsertPR(
		a.ctx,
		repo.ID, branch.ID, pr.Number,
		pr.Title, pr.Author, pr.State, pr.ReviewStatus, pr.URL, pr.HeadBranch,
		pr.Mergeable, pr.MergeStateStatus, pr.CIStatus, pr.IsDraft, pr.CreatedAt, pr.UpdatedAt,
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
		if _, _, err := a.vcsStore.PullRequests.UpsertComment(
			a.ctx,
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
// RefreshContextPRs re-syncs pull requests for the context's registered VCS
// branches from the remote (gh pr list --head <branch>) and returns the fresh
// list. Unlike RefreshContext (which also re-hits Jira/Slack and swallows VCS
// warnings), this is PR-only and surfaces the sync error to the caller so the
// editor can show it instead of silently rendering nothing.
func (a *App) RefreshContextPRs(contextID string) ([]vcsstore.PullRequest, error) {
	if _, err := a.vcsSvc.Refresh(a.ctx, contextID); err != nil {
		return nil, fmt.Errorf("refresh PRs for context: %w", err)
	}
	return a.GetVCSPRsForContext(contextID)
}

// ReconcileMyOpenPRs re-checks every PR stored as state='open' against its remote
// and updates any that have since merged or closed. The board's "My Open PRs" reads
// state='open', and per-context sync only runs when a context is opened/refreshed —
// so a PR merged in a quiet context lingers as "open" until this reconciles it.
// One remote list call per distinct repo; best-effort (a failing repo is logged, skipped).
func (a *App) ReconcileMyOpenPRs() error {
	open, err := a.vcsStore.PullRequests.ListOpenPRsWithRepo(a.ctx)
	if err != nil {
		return fmt.Errorf("list open prs: %w", err)
	}
	if len(open) == 0 {
		return nil
	}

	// Group open PR numbers by repo so each remote is queried once.
	type repoInfo struct {
		platform, owner, name string
		numbers               map[int]bool
	}
	repos := map[string]*repoInfo{}
	for _, o := range open {
		ri := repos[o.RepoID]
		if ri == nil {
			ri = &repoInfo{platform: o.Platform, owner: o.Owner, name: o.Name, numbers: map[int]bool{}}
			repos[o.RepoID] = ri
		}
		ri.numbers[o.PRNumber] = true
	}

	for repoID, ri := range repos {
		runner, err := a.vcsSvc.RunnerFor(vcs.Platform(ri.platform))
		if err != nil {
			a.logger.Sugar().Warnf("reconcile prs: runner for %s: %v", ri.platform, err)
			continue
		}
		// branch="" → all PRs (any state) for the repo. A merged PR is still returned
		// by its number even after its head branch is deleted.
		prs, err := runner.ListPRsForBranch(a.ctx, ri.owner, ri.name, "")
		if err != nil {
			a.logger.Sugar().Warnf("reconcile prs: list %s/%s: %v", ri.owner, ri.name, err)
			continue
		}
		for _, pr := range prs {
			if ri.numbers[pr.Number] && pr.State != "open" {
				if err := a.vcsStore.PullRequests.SetPRState(a.ctx, repoID, pr.Number, pr.State); err != nil {
					a.logger.Sugar().Warnf("reconcile prs: set state %s#%d: %v", ri.name, pr.Number, err)
				}
			}
		}
	}
	return nil
}

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
	owner, name := parts[0], parts[1]

	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, platform, owner, name)
	if err != nil {
		return nil, err
	}

	// Best-effort: populate description from GitHub. Never blocks or errors the caller.
	if repo.Description == "" {
		if runner, err := a.vcsSvc.RunnerFor(p); err == nil {
			if desc, err := runner.GetRepoDescription(a.ctx, owner, name); err == nil && desc != "" {
				if updateErr := a.vcsStore.Repos.UpdateRepoMeta(a.ctx, repo.ID, repo.Label, desc, repo.OnCheckoutCommand); updateErr == nil {
					repo.Description = desc
				}
			}
		}
	}
	return repo, nil
}

// RemoveTrackedRepo removes a tracked repo and all its review requests.
func (a *App) RemoveTrackedRepo(repoID string) error {
	return a.vcsStore.Repos.DeleteRepo(a.ctx, repoID)
}

// ListTrackedRepos returns all tracked repos, optionally filtered by platform.
func (a *App) ListTrackedRepos(platform string) ([]vcsstore.VCSRepo, error) {
	return a.vcsStore.Repos.ListRepos(a.ctx, platform)
}

// UpdateTrackedRepoMeta updates the label, description, and on-checkout command for a tracked repo.
func (a *App) UpdateTrackedRepoMeta(repoID, label, description, onCheckoutCommand string) error {
	return a.vcsStore.Repos.UpdateRepoMeta(a.ctx, repoID, label, description, onCheckoutCommand)
}

// GetRepoFileTree returns repo-root-relative file paths for a linked repo's local
// checkout, for the AI to choose which files are relevant. Ignored/build dirs are skipped.
func (a *App) GetRepoFileTree(repoPath string) ([]string, error) {
	return repofiles.FileTree(repoPath)
}

// ReadRepoFiles returns contents for the given repo-relative paths, path-confined to
// repoPath and size-bounded. Used to inject code context into analysis prompts.
func (a *App) ReadRepoFiles(repoPath string, paths []string) (map[string]string, error) {
	return repofiles.ReadFiles(repoPath, paths)
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
	if err := a.vcsSvc.SyncReviewRequests(a.ctx, vcs.PlatformGitHub); err != nil {
		return err
	}
	// Authored PRs are additive; a failure here should not blank the requested list.
	if err := a.vcsSvc.SyncAuthoredPRs(a.ctx, vcs.PlatformGitHub); err != nil {
		a.logger.Warn("refresh: sync authored PRs failed", zap.Error(err))
	}
	return nil
}

// DismissReviewRequest hides a "waiting for author" item from the Code Reviews page.
func (a *App) DismissReviewRequest(id string) error {
	return a.vcsStore.ReviewRequests.DismissReviewRequest(a.ctx, id)
}

// MarkReviewRequestViewed marks a single Code-Reviews item as seen (clears its "new" dot).
func (a *App) MarkReviewRequestViewed(id string) error {
	return a.vcsStore.ReviewRequests.MarkReviewRequestViewed(a.ctx, id)
}

// MarkAllReviewRequestsViewed marks every pending Code-Reviews item as seen.
func (a *App) MarkAllReviewRequestsViewed() error {
	return a.vcsStore.ReviewRequests.MarkAllReviewRequestsViewed(a.ctx)
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
	if err := a.vcsStore.Repos.UpdateLocalPath(a.ctx, repoID, localPath); err != nil {
		return err
	}
	if localPath != "" {
		if _, err := a.vcsStore.RepoPaths.Upsert(a.ctx, repoID, localPath); err != nil {
			return err
		}
	}
	return nil
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
	return a.openWithCheckout("", localPath)
}

// knownAgentDirs is the minimal set of directories checked when the app's
// PATH doesn't include Homebrew or user-local installs.
var knownAgentDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
}

// resolveAgentPath resolves a coding agent executable name to an absolute path,
// checking known extra dirs when exec.LookPath fails (Wails apps have a stripped PATH).
func resolveAgentPath(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range knownAgentDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

// openWithCheckout runs onCheckoutCommand (if set) via the user's shell, then
// opens the configured terminal at path and/or the coding agent directly.
func (a *App) openWithCheckout(onCheckoutCommand, path string) error {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		a.logger.Sugar().Warnf("openWithCheckout: failed to load settings: %v", err)
		return nil
	}

	terminalApp := settings.TerminalAppPath
	agentApp := settings.CodingAgentApp

	if terminalApp == "" && agentApp == "" {
		return nil
	}

	// Run the checkout command to completion before opening the agent so the
	// environment is ready (e.g. deps installed) when the editor opens.

	// Emits checkout:start/log/done events. These are only surfaced in the UI on
	// tabs that mount the useCheckout hook (Contexts, Context Detail, Pending Code
	// Reviews today). Callers adding checkout triggers from other tabs need to
	// mount the hook there too.
	if onCheckoutCommand != "" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		cmd := exec.Command(shell, "-c", onCheckoutCommand)
		cmd.Dir = path

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		runtime.EventsEmit(a.ctx, "checkout:start", map[string]interface{}{
			"path": path,
			"cmd":  onCheckoutCommand,
		})

		if err := cmd.Start(); err != nil {
			a.logger.Sugar().Warnf("on_checkout_command start failed: cmd=%q workDir=%q err=%v", onCheckoutCommand, path, err)
			runtime.EventsEmit(a.ctx, "checkout:done", map[string]interface{}{
				"exitCode": -1,
				"error":    err.Error(),
			})
		} else {
			// Besides streaming to the UI, keep a bounded tail of the output so
			// the run is diagnosable later from the Logs page — the checkout:log
			// events are visible only on tabs that mount useCheckout.
			var outMu sync.Mutex
			var outTail []string
			const maxTailLines = 200
			streamPipe := func(r io.Reader, stream string) {
				sc := bufio.NewScanner(r)
				sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // ponytail: 1MB cap per line, enough for npm/yarn output
				for sc.Scan() {
					runtime.EventsEmit(a.ctx, "checkout:log", map[string]interface{}{
						"line":   sc.Text(),
						"stream": stream,
					})
					outMu.Lock()
					outTail = append(outTail, "["+stream+"] "+sc.Text())
					if len(outTail) > maxTailLines {
						outTail = outTail[len(outTail)-maxTailLines:]
					}
					outMu.Unlock()
				}
			}
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); streamPipe(stdout, "stdout") }()
			go func() { defer wg.Done(); streamPipe(stderr, "stderr") }()
			wg.Wait()
			runErr := cmd.Wait()
			exitCode := 0
			errStr := ""
			output := strings.Join(outTail, "\n")
			if runErr != nil {
				exitCode = -1
				if ee, ok := runErr.(*exec.ExitError); ok {
					exitCode = ee.ExitCode()
				}
				errStr = runErr.Error()
				a.logger.Warn("on_checkout_command failed",
					zap.String("cmd", onCheckoutCommand), zap.String("workDir", path),
					zap.Int("exitCode", exitCode), zap.String("output", output), zap.Error(runErr))
			} else {
				a.logger.Info("on_checkout_command ok",
					zap.String("cmd", onCheckoutCommand), zap.String("workDir", path),
					zap.String("output", output))
			}
			runtime.EventsEmit(a.ctx, "checkout:done", map[string]interface{}{
				"exitCode": exitCode,
				"error":    errStr,
			})
		}
	}

	// No terminal configured: launch coding agent directly.
	if terminalApp == "" {
		agentBin := resolveAgentPath(agentApp)
		cmd := exec.Command(agentBin, path)
		if err := cmd.Start(); err != nil {
			a.logger.Sugar().Warnf("open coding agent failed: app=%s err=%v", agentApp, err)
		}
		return nil
	}

	// Open terminal at the directory. Passing a directory (not a script file)
	// works across terminal apps — each opens a new window at that path.
	termCmd := exec.Command("open", "-a", terminalApp, path)
	if err := termCmd.Start(); err != nil {
		a.logger.Sugar().Warnf("open terminal failed: app=%s err=%v", terminalApp, err)
	}

	// Open coding agent separately if configured.
	if agentApp != "" {
		agentBin := resolveAgentPath(agentApp)
		agentCmd := exec.Command(agentBin, path)
		if err := agentCmd.Start(); err != nil {
			a.logger.Sugar().Warnf("open coding agent failed: app=%s err=%v", agentApp, err)
		}
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
// Falls back to scanning registered local paths via git worktree list when no DB record exists,
// and auto-persists any discovered worktree so future lookups are instant.
// samePath reports whether two paths point at the same location, resolving
// symlinks first. git worktree list returns canonical paths (e.g. /private/var
// on macOS) while a registered repo path may be a symlink (e.g. /var), so a
// plain string compare would wrongly treat the main checkout as a worktree.
func samePath(p, q string) bool {
	if p == q {
		return true
	}
	rp, errP := filepath.EvalSymlinks(p)
	rq, errQ := filepath.EvalSymlinks(q)
	if errP == nil && errQ == nil {
		return rp == rq
	}
	return filepath.Clean(p) == filepath.Clean(q)
}

func (a *App) FindWorkspaceForBranch(repoID, branch string) (*vcsstore.VCSWorkspace, error) {
	// Fast path: DB record exists.
	if ws, err := a.vcsStore.Workspaces.FindByBranch(a.ctx, repoID, branch); err != nil || ws != nil {
		return ws, err
	}
	// Fallback: scan all registered local paths for an existing git worktree.
	paths, err := a.vcsStore.RepoPaths.ListForRepo(a.ctx, repoID)
	if err != nil || len(paths) == 0 {
		return nil, nil
	}
	for _, rp := range paths {
		worktrees, wErr := vcs.GitWorktreeList(rp.Path)
		if wErr != nil {
			continue
		}
		for _, wt := range worktrees {
			if wt.Branch != branch || samePath(wt.Path, rp.Path) {
				continue
			}
			ws := &vcsstore.VCSWorkspace{
				RepoPathID:   rp.ID,
				RepoID:       repoID,
				Branch:       branch,
				WorktreePath: wt.Path,
				Status:       "active",
			}
			_ = a.vcsStore.Workspaces.Create(a.ctx, ws)
			return ws, nil
		}
	}
	return nil, nil
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

	// If the branch is already checked out in a git worktree, reuse it.
	if existing, err := vcs.GitWorktreeList(baseDir); err == nil {
		for _, wt := range existing {
			if wt.Branch == branch {
				ws := &vcsstore.VCSWorkspace{
					RepoPathID:   repoPathID,
					RepoID:       repoPath.RepoID,
					Branch:       branch,
					WorktreePath: wt.Path,
					Status:       "active",
				}
				// Best-effort: record the workspace if not already tracked.
				_ = a.vcsStore.Workspaces.Create(a.ctx, ws)
				repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoPath.RepoID)
				onCheckout := ""
				if err == nil {
					onCheckout = repo.OnCheckoutCommand
				}
				return a.openWithCheckout(onCheckout, wt.Path)
			}
		}
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
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoPath.RepoID)
	onCheckout := ""
	if err == nil {
		onCheckout = repo.OnCheckoutCommand
	}
	return a.openWithCheckout(onCheckout, worktreePath)
}

// OpenWorkspaceTerminal runs on_checkout_command (if configured) and opens terminal at the workspace path.
func (a *App) OpenWorkspaceTerminal(workspaceID string) error {
	w, err := a.vcsStore.Workspaces.Get(a.ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, w.RepoID)
	onCheckout := ""
	if err == nil {
		onCheckout = repo.OnCheckoutCommand
	}
	return a.openWithCheckout(onCheckout, w.WorktreePath)
}

// ensureWorktreeForBranch finds or creates a git worktree for (repoID, branch)
// and returns its on-disk path. Unlike CheckoutInWorkspace it does NOT open a
// terminal or run onCheckoutCommand — it only guarantees the worktree exists.
// Returns "" (no error) when the repo has no accessible local path to host a
// worktree, so callers can skip that repo.
func (a *App) ensureWorktreeForBranch(repoID, branch string) (string, error) {
	if ws, err := a.FindWorkspaceForBranch(repoID, branch); err == nil && ws != nil {
		return ws.WorktreePath, nil
	}
	paths, err := a.vcsStore.RepoPaths.ListForRepo(a.ctx, repoID)
	if err != nil || len(paths) == 0 {
		return "", nil
	}
	rp := paths[0]
	baseDir := rp.Path
	if _, err := os.Stat(baseDir); err != nil {
		return "", nil
	}
	if err := ensureWorktreesIgnored(baseDir); err != nil {
		a.logger.Sugar().Warnf("ensureWorktreeForBranch: .gitignore update failed: %v", err)
	}
	// Reuse an existing on-disk worktree for the branch if one is already present.
	if existing, err := vcs.GitWorktreeList(baseDir); err == nil {
		for _, wt := range existing {
			if wt.Branch == branch && !samePath(wt.Path, baseDir) {
				_ = a.vcsStore.Workspaces.Create(a.ctx, &vcsstore.VCSWorkspace{
					RepoPathID: rp.ID, RepoID: repoID, Branch: branch, WorktreePath: wt.Path, Status: "active",
				})
				return wt.Path, nil
			}
		}
	}
	worktreePath := filepath.Join(baseDir, ".worktrees", vcs.BranchToWorktreeName(branch))
	if err := vcs.GitWorktreeAdd(baseDir, worktreePath, branch); err != nil {
		return "", err
	}
	_ = a.vcsStore.Workspaces.Create(a.ctx, &vcsstore.VCSWorkspace{
		RepoPathID: rp.ID, RepoID: repoID, Branch: branch, WorktreePath: worktreePath, Status: "active",
	})
	return worktreePath, nil
}

// ensureContextWorkspaces creates/finds a git worktree for every repo+branch
// attached to contextID and returns the first worktree path (used to open a
// single terminal). Per-repo failures are logged and skipped.
func (a *App) ensureContextWorkspaces(contextID string) (string, error) {
	wts, err := a.contextWorktrees(contextID)
	if err != nil {
		return "", err
	}
	if len(wts) == 0 {
		return "", nil
	}
	return wts[0].WorktreePath, nil
}

// OpenRepoPathTerminal opens the configured terminal at the given repo local path.
func (a *App) OpenRepoPathTerminal(pathID string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	return a.openWithCheckout("", rp.Path)
}

// FetchPruneRepoPath runs `git fetch --prune` for the given repo path.
func (a *App) FetchPruneRepoPath(pathID string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	cmd := exec.Command("git", "-C", rp.Path, "fetch", "--prune")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch --prune in %s: %w\n%s", rp.Path, err, out)
	}
	return nil
}

// ListLocalBranches returns all local git branches for the given repo path.
func (a *App) ListLocalBranches(pathID string) ([]vcs.LocalBranch, error) {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return nil, fmt.Errorf("repo path not found: %w", err)
	}
	return vcs.GitListLocalBranches(rp.Path)
}

// DeleteLocalBranch force-deletes a local branch in the given repo path.
func (a *App) DeleteLocalBranch(pathID, branch string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	return vcs.GitDeleteLocalBranch(rp.Path, branch)
}

// RemoveGoneLocalBranches deletes every local branch at pathID whose upstream is
// gone (skipping the current branch). Best-effort: a per-branch failure is
// collected, not fatal. Returns the number of branches actually removed.
func (a *App) RemoveGoneLocalBranches(pathID string) (int, error) {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return 0, fmt.Errorf("repo path not found: %w", err)
	}
	branches, err := vcs.GitListLocalBranches(rp.Path)
	if err != nil {
		return 0, fmt.Errorf("list local branches: %w", err)
	}
	removed := 0
	var failed []string
	for _, name := range vcs.GoneDeletableBranches(branches) {
		if delErr := vcs.GitDeleteLocalBranch(rp.Path, name); delErr != nil {
			a.logger.Sugar().Warnf("RemoveGoneLocalBranches: delete %s failed: %v", name, delErr)
			failed = append(failed, name)
			continue
		}
		removed++
	}
	if len(failed) > 0 {
		return removed, fmt.Errorf("removed %d, failed to remove: %s", removed, strings.Join(failed, ", "))
	}
	return removed, nil
}

// UpdatePRBranch merges the base branch into the given PR's head branch on the remote
// (GitHub's "Update branch"). Resolves the repo from repoID and dispatches to its runner.
func (a *App) UpdatePRBranch(repoID string, prNumber int) error {
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoID)
	if err != nil {
		return fmt.Errorf("resolving repo %q: %w", repoID, err)
	}
	runner, err := a.vcsSvc.RunnerFor(vcs.Platform(repo.Platform))
	if err != nil {
		return fmt.Errorf("getting runner for %q: %w", repo.Platform, err)
	}
	if err := runner.UpdatePRBranch(a.ctx, repo.Owner, repo.Name, prNumber); err != nil {
		return fmt.Errorf("updating PR #%d branch: %w", prNumber, err)
	}
	return nil
}

// SwitchMainWorkingTree switches the primary checkout of the repo at pathID to branch.
func (a *App) SwitchMainWorkingTree(repoPathID, branch string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, repoPathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	cmd := exec.Command("git", "-C", rp.Path, "switch", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git switch %s in %s: %w\n%s", branch, rp.Path, err, out)
	}
	return nil
}

// CheckoutLocalBranch checks out a local branch as a worktree (or opens the
// existing workspace terminal if already checked out) and opens terminal.
func (a *App) CheckoutLocalBranch(repoPathID, branch string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, repoPathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	existing, err := a.vcsStore.Workspaces.FindByBranch(a.ctx, rp.RepoID, branch)
	if err == nil && existing != nil {
		repo, rErr := a.vcsStore.Repos.GetRepo(a.ctx, rp.RepoID)
		onCheckout := ""
		if rErr == nil {
			onCheckout = repo.OnCheckoutCommand
		}
		return a.openWithCheckout(onCheckout, existing.WorktreePath)
	}
	return a.CheckoutInWorkspace(repoPathID, branch)
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
	repoPath, err := a.vcsStore.RepoPaths.Get(a.ctx, w.RepoPathID)
	if err != nil {
		a.logger.Sugar().Warnf("DeleteWorkspace: repo path not found (continuing): %v", err)
	} else {
		if rmErr := vcs.GitWorktreeRemove(repoPath.Path, w.WorktreePath); rmErr != nil {
			a.logger.Sugar().Warnf("DeleteWorkspace: git worktree remove failed (continuing): %v", rmErr)
		}
	}
	return a.vcsStore.Workspaces.Delete(a.ctx, workspaceID)
}

// MarkWorkspaceShouldDelete sets a workspace's status to "should_delete".
func (a *App) MarkWorkspaceShouldDelete(workspaceID string) error {
	return a.vcsStore.Workspaces.UpdateStatus(a.ctx, workspaceID, "should_delete")
}

// --- AI Branch Matching ---

const (
	// maxMatchRepos caps how many tracked repos per platform are scanned for
	// candidate branches, to keep AIMatchBranches bounded on large repo lists.
	maxMatchRepos = 40
	// maxMatchCandidates caps the total number of candidate branches sent to
	// the AI matcher across all platforms/repos.
	maxMatchCandidates = 400
)

// defaultBranchNames lists common default/trunk branch names that are never
// useful match candidates and are dropped by dedupeMatches.
var defaultBranchNames = map[string]bool{"main": true, "master": true, "develop": true, "dev": true}

// MatchedBranch is a candidate (or matched) branch surfaced to the frontend.
// JSON tags MUST match the frontend VCSBranchInput shape.
type MatchedBranch struct {
	Platform  string `json:"platform"`
	OwnerRepo string `json:"ownerRepo"`
	Branch    string `json:"branch"`
	Label     string `json:"label"`
}

// dedupeMatches drops candidates whose branch is a common default/trunk
// branch name or empty, and collapses duplicate platform/ownerRepo/branch
// triples, preserving the first occurrence's order.
func dedupeMatches(candidates []MatchedBranch) []MatchedBranch {
	seen := map[string]bool{}
	out := make([]MatchedBranch, 0, len(candidates))
	for _, c := range candidates {
		if defaultBranchNames[c.Branch] || c.Branch == "" {
			continue
		}
		key := c.Platform + "|" + c.OwnerRepo + "|" + c.Branch
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// buildIdentifierMatchers compiles one whole-token, case-insensitive regexp per
// non-empty, de-duplicated identifier (the context key + linked ticket keys).
// Token boundaries are string edges or any non-alphanumeric character, so
// "JH-1" matches "feat/JH-1-x" but not "JH-100" or "myJH-1x".
func buildIdentifierMatchers(identifiers []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	seen := map[string]bool{}
	for _, id := range identifiers {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		low := strings.ToLower(id)
		if seen[low] {
			continue
		}
		seen[low] = true
		re, err := regexp.Compile(`(?i)(?:^|[^a-zA-Z0-9])` + regexp.QuoteMeta(id) + `(?:[^a-zA-Z0-9]|$)`)
		if err != nil {
			continue
		}
		out = append(out, re)
	}
	return out
}

// regexMatchBranches returns the candidates whose Branch matches any matcher,
// preserving input order and including each candidate at most once. Returns nil
// when there are no matchers or no hits.
func regexMatchBranches(candidates []MatchedBranch, matchers []*regexp.Regexp) []MatchedBranch {
	if len(matchers) == 0 {
		return nil
	}
	var out []MatchedBranch
	for _, c := range candidates {
		for _, re := range matchers {
			if re.MatchString(c.Branch) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// contextMatchSignal builds the titles-only signal passed to the AI branch
// matcher: the edited context's key plus a short summary of the Jira ticket
// keys/summaries of the edited context AND every directly-related context
// (relations of any type). No ticket bodies/comments/code. Duplicate Jira keys
// across contexts are collapsed. Fail-soft: a relations-lookup error falls back
// to the edited context alone, and per-context issue errors are skipped.
func (a *App) contextMatchSignal(contextID string) (key, summary string, ticketKeys []string, err error) {
	ctxRow, err := a.db.GetContext(a.ctx, contextID)
	if err != nil {
		return "", "", nil, err
	}

	// The signal spans the edited context plus its directly-related contexts.
	ids := []string{contextID}
	seenID := map[string]bool{contextID: true}
	rels, rerr := a.db.ListContextRelations(a.ctx, contextID)
	if rerr != nil {
		a.logger.Sugar().Warnf("AIMatchBranches: list context relations failed: %s", rerr.Error())
	} else {
		for _, r := range rels {
			if r.TargetID == "" || seenID[r.TargetID] {
				continue
			}
			seenID[r.TargetID] = true
			ids = append(ids, r.TargetID)
		}
	}

	var lines []string
	seenKey := map[string]bool{}
	if a.svc != nil {
		for _, id := range ids {
			issues, ierr := a.svc.GetIssuesWithComments(a.ctx, id)
			if ierr != nil {
				a.logger.Sugar().Warnf("AIMatchBranches: list jira issues failed: %s", ierr.Error())
				continue
			}
			for _, issue := range issues {
				if seenKey[issue.JiraKey] {
					continue
				}
				seenKey[issue.JiraKey] = true
				ticketKeys = append(ticketKeys, issue.JiraKey)
				lines = append(lines, issue.JiraKey+": "+issue.Summary)
			}
		}
	}
	return ctxRow.Key, strings.Join(lines, "\n"), ticketKeys, nil
}

// AIMatchBranches enumerates branches across tracked repos (capped by
// maxMatchRepos/maxMatchCandidates) and asks the AI matcher which ones
// likely belong to contextID, based on branch names/titles only. Fail-soft:
// per-repo enumeration errors are skipped rather than aborting the whole
// scan. Returns an empty slice (not nil) when there are no candidates.
func (a *App) AIMatchBranches(contextID string) ([]MatchedBranch, error) {
	key, summary, ticketKeys, err := a.contextMatchSignal(contextID)
	if err != nil {
		return nil, err
	}

	var candidates []MatchedBranch
	for _, p := range vcs.KnownPlatforms() {
		if !vcs.IsPlatformEnabled(p) {
			continue
		}
		platform := string(p)
		repos, rerr := a.ListTrackedRepos(platform)
		if rerr != nil {
			a.logger.Sugar().Warnf("AIMatchBranches: list tracked repos failed: %s", rerr.Error())
			continue
		}
		for ri, repo := range repos {
			if ri >= maxMatchRepos || len(candidates) >= maxMatchCandidates {
				break
			}
			ownerRepo := repo.FullName()
			names, berr := a.ListRepoBranches(platform, ownerRepo)
			if berr != nil {
				a.logger.Sugar().Warnf("AIMatchBranches: list repo branches failed: %s", berr.Error())
				continue
			}
			for _, name := range names {
				candidates = append(candidates, MatchedBranch{Platform: platform, OwnerRepo: ownerRepo, Branch: name})
				if len(candidates) >= maxMatchCandidates {
					break
				}
			}
		}
	}

	candidates = dedupeMatches(candidates)
	if len(candidates) == 0 {
		return []MatchedBranch{}, nil
	}

	// Cheap deterministic pass first: match the context/ticket keys against
	// branch names. Only fall back to the (slower, paid) AI matcher when this
	// finds nothing.
	matchers := buildIdentifierMatchers(append([]string{key}, ticketKeys...))
	if hits := regexMatchBranches(candidates, matchers); len(hits) > 0 {
		return hits, nil
	}

	titles := make([]string, len(candidates))
	for i, c := range candidates {
		titles[i] = c.OwnerRepo + ": " + c.Branch
	}
	idx, merr := a.analysisSvc.MatchBranchesToContext(a.ctx, key, summary, titles)
	if merr != nil {
		a.logger.Sugar().Warnf("AIMatchBranches: ai match failed: %s", merr.Error())
		return nil, merr
	}
	out := make([]MatchedBranch, 0, len(idx))
	for _, i := range idx {
		if i < 0 || i >= len(candidates) {
			continue
		}
		out = append(out, candidates[i])
	}
	return out, nil
}

// AutoLinkRepos runs AI branch matching for contextID in the background and
// persists the matched branches — but only if the AutoLinkReposOnContextCreate
// setting is enabled. Fire-and-forget: the frontend calls this after a new
// context is populated; an "autolink:done" event is emitted when finished.
func (a *App) AutoLinkRepos(contextID string) {
	go func() {
		if _, err := a.autoLinkReposOnce(contextID); err != nil {
			a.logger.Sugar().Warnf("AutoLinkRepos: %s", err.Error())
		}
	}()
}

// autoLinkReposOnce is the synchronous, testable core of AutoLinkRepos. It
// returns the number of branches linked. When the setting is disabled it is a
// no-op returning (0, nil) without invoking the matcher.
func (a *App) autoLinkReposOnce(contextID string) (int, error) {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return 0, err
	}
	if settings == nil || !settings.AutoLinkReposOnContextCreate {
		return 0, nil
	}
	matches, err := a.AIMatchBranches(contextID)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, m := range matches {
		if _, err := a.AddVCSBranch(contextID, m.Platform, m.OwnerRepo, m.Branch, m.Label, true); err != nil {
			a.logger.Sugar().Warnf("AutoLinkRepos: add branch failed: %s", err.Error())
			continue
		}
		added++
	}
	runtime.EventsEmit(a.ctx, "autolink:done", map[string]interface{}{
		"contextId": contextID,
		"added":     added,
	})
	return added, nil
}

// --- VCS Diff ---

// RepoChange is the working-tree diff for one repo attached to a context.
type RepoChange struct {
	RepoID   string           `json:"repoId"`
	RepoName string           `json:"repoName"` // owner/name
	RepoPath string           `json:"repoPath"`
	Branch   string           `json:"branch"` // context branch link this path serves ("" = main checkout)
	Files    []vcs.FileChange `json:"files"`
}

// dedupeRepoPaths removes entries with an empty RepoPath and collapses
// duplicate RepoPath entries, keeping the first occurrence.
func dedupeRepoPaths(in []RepoChange) []RepoChange {
	seen := map[string]bool{}
	out := make([]RepoChange, 0, len(in))
	for _, r := range in {
		if r.RepoPath == "" || seen[r.RepoPath] {
			continue
		}
		seen[r.RepoPath] = true
		out = append(out, r)
	}
	return out
}

// StageFile stages repo-relative path in the repo at repoPath.
func (a *App) StageFile(repoPath, path string) error {
	if _, err := safeJoin(repoPath, path); err != nil {
		return err
	}
	return vcs.GitStageFile(repoPath, path)
}

// UnstageFile removes repo-relative path from the index, keeping the working tree.
func (a *App) UnstageFile(repoPath, path string) error {
	if _, err := safeJoin(repoPath, path); err != nil {
		return err
	}
	return vcs.GitUnstageFile(repoPath, path)
}

// ResolveConflict settles a merge conflict on repo-relative path by taking one
// whole side: "ours" (this branch) or "theirs" (the branch being merged in).
// Editing the conflict markers and staging the file is the finer-grained route;
// this is the shortcut for files nobody wants to merge by hand (lockfiles,
// generated output).
func (a *App) ResolveConflict(repoPath, path, side string) error {
	if _, err := safeJoin(repoPath, path); err != nil {
		return err
	}
	return vcs.ResolveConflict(repoPath, path, side)
}

// DiscardFile discards working-tree (and staged) changes for repo-relative
// path: tracked files are restored to HEAD; untracked files are deleted. The
// path is containment-checked before any filesystem deletion.
func (a *App) DiscardFile(repoPath, path string) error {
	full, err := safeJoin(repoPath, path)
	if err != nil {
		return err
	}
	if restoreErr := vcs.GitRestoreFile(repoPath, path); restoreErr == nil {
		return nil
	}
	// No HEAD entry (untracked): remove the file. `full` is guaranteed inside repoPath.
	if rmErr := os.Remove(full); rmErr != nil {
		return fmt.Errorf("discard %s: %w", path, rmErr)
	}
	return nil
}

// ConfirmAction shows a native Yes/No dialog and reports whether the user
// confirmed. Used instead of window.confirm(), which the Wails WKWebView does
// not reliably support (it returns false without ever prompting), silently
// no-opping confirm-gated actions like file revert/discard.
func (a *App) ConfirmAction(title, message string) (bool, error) {
	sel, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{"Yes", "No"},
		DefaultButton: "Yes",
		CancelButton:  "No",
	})
	if err != nil {
		return false, err
	}
	return sel == "Yes", nil
}

// GetContextChanges returns the git working-tree changes for every repo
// attached to contextID (deduped, only repos whose LocalPath exists on
// disk). Diffing is fail-soft: a repo that errors during diff is left with
// nil Files rather than failing the whole call.
func (a *App) GetContextChanges(contextID string) ([]RepoChange, error) {
	branches, err := a.ListVCSBranches(contextID)
	if err != nil {
		return nil, fmt.Errorf("list vcs branches: %w", err)
	}
	var repos []RepoChange
	for _, b := range branches {
		repo, rerr := a.vcsStore.Repos.GetRepo(a.ctx, b.RepoID)
		if rerr != nil {
			continue
		}
		// Prefer the branch's worktree (e.g. the PR checkout from "Review with
		// AI") so the editor's terminal, diff, and changed-files panel operate on
		// it rather than the repo's main checkout.
		var candidates []string
		if ws, werr := a.vcsStore.Workspaces.FindByBranch(a.ctx, b.RepoID, b.Branch); werr == nil && ws != nil && ws.WorktreePath != "" {
			candidates = append(candidates, ws.WorktreePath)
		}
		// Paths live in vcs_repo_paths; local_path is a legacy fallback.
		if paths, perr := a.vcsStore.RepoPaths.ListForRepo(a.ctx, b.RepoID); perr == nil {
			for _, p := range paths {
				candidates = append(candidates, p.Path)
			}
		}
		if repo.LocalPath != "" {
			candidates = append(candidates, repo.LocalPath)
		}
		for _, path := range candidates {
			if info, serr := os.Stat(path); serr != nil || !info.IsDir() {
				continue
			}
			repos = append(repos, RepoChange{RepoID: b.RepoID, RepoName: repo.Owner + "/" + repo.Name, RepoPath: path, Branch: b.Branch})
			break // one (first existing) path per repo
		}
	}
	repos = dedupeRepoPaths(repos)
	for i := range repos {
		files, derr := vcs.RepoDiff(repos[i].RepoPath)
		if derr != nil {
			continue // fail-soft: skip un-diffable repo, leave Files nil
		}
		repos[i].Files = files
	}
	return repos, nil
}
