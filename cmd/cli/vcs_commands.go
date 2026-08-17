package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	"github.com/spf13/cobra"
)

// newRepoListCmd lists every connected repository together with its local
// checkout paths and git-worktree workspaces.
func newRepoListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-list",
		Short: "List connected repositories with their local paths and workspaces",
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			repos, err := env.vcsStore.ListReposWithLocations(context.Background())
			if err != nil {
				return fmt.Errorf("listing repos: %w", err)
			}
			return writeJSON(repos)
		},
	}
	return setAgentDocs(cmd,
		"List every connected (tracked) repository with its local checkout paths "+
			"(vcs_repo_paths) and git-worktree workspaces (vcs_workspaces). Read-only. "+
			"Use to answer 'which repos are connected and where on disk are they / their workspaces'. "+
			"The top-level localPath field is deprecated; use the paths[] array.",
		"(no flags)",
		`[{"id":"uuid","platform":"github","owner":"acme","name":"api","localPath":"","label":"","description":"","onCheckoutCommand":"","createdAt":"RFC3339","paths":[{"id":"uuid","repoId":"uuid","path":"/abs/checkout","createdAt":"RFC3339"}],"workspaces":[{"id":"uuid","repoPathId":"uuid","repoId":"uuid","branch":"feat/x","worktreePath":"/abs/checkout/.worktrees/feat-x","status":"active","createdAt":"RFC3339","updatedAt":"RFC3339"}]}]`,
		`vibecodeway-cli repo-list`,
	)
}

// splitOwnerRepo splits "owner/repo" into its two halves; both must be non-empty.
func splitOwnerRepo(ownerRepo string) (string, string, error) {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("--owner-repo must be in 'owner/repo' format, got %q", ownerRepo)
	}
	return parts[0], parts[1], nil
}

// newBranchAddCmd links a repo (and optionally one branch) to a context.
// With --pr-number and no --branch it asks gh for the PR's head branch, because
// a PR URL carries the PR number but not the branch name.
func newBranchAddCmd() *cobra.Command {
	var contextID, contextKey, ownerRepo, branch, label, platform string
	var prNumber int

	cmd := &cobra.Command{
		Use:   "branch-add",
		Short: "Link a repository (and optionally a branch) to a context",
		RunE: func(_ *cobra.Command, _ []string) error {
			if ownerRepo == "" {
				return fmt.Errorf("--owner-repo is required")
			}
			if !vcs.IsPlatformEnabled(vcs.Platform(platform)) {
				return fmt.Errorf("platform %q is not supported yet", platform)
			}
			owner, name, err := splitOwnerRepo(ownerRepo)
			if err != nil {
				return err
			}

			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			id, err := resolveContextID(ctx, env, contextID, contextKey)
			if err != nil {
				return err
			}

			if branch == "" && prNumber > 0 {
				// env.runner is hard-wired to GitHub. Correct today (only github is
				// enabled per vcs.IsPlatformEnabled), but if another platform is ever
				// enabled this must route through a platform-appropriate runner
				// instead of assuming GitHub.
				pr, err := env.runner.GetPRByNumber(ctx, owner, name, prNumber)
				if err != nil {
					return fmt.Errorf("resolving head branch of PR #%d: %w", prNumber, err)
				}
				branch = pr.HeadBranch
			}

			repo, err := env.vcsStore.Repos.EnsureRepo(ctx, platform, owner, name)
			if err != nil {
				return fmt.Errorf("ensuring repo: %w", err)
			}
			b, err := env.vcsStore.Branches.AddBranch(ctx, id, repo.ID, branch, label)
			if err != nil {
				return fmt.Errorf("adding branch: %w", err)
			}
			b.Repo = repo
			return writeJSON(b)
		},
	}

	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key, e.g. 'sprint-42'")
	cmd.Flags().StringVar(&ownerRepo, "owner-repo", "", "Repository in 'owner/repo' format")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name; empty tracks every PR of the repo")
	cmd.Flags().IntVar(&prNumber, "pr-number", 0, "PR number; its head branch is used when --branch is omitted")
	cmd.Flags().StringVar(&label, "label", "", "Optional human-readable tag, e.g. 'api'")
	cmd.Flags().StringVar(&platform, "platform", "github", "VCS platform (only 'github' is supported)")
	_ = cmd.MarkFlagRequired("owner-repo")

	return setAgentDocs(cmd,
		"Link a repository (and optionally one branch) to a context. Pass --branch for a branch link, "+
			"--pr-number to resolve the branch from a pull request via gh, or neither to track every PR of the repo. "+
			"Idempotent: re-adding an existing repo+branch link returns it and updates the label. "+
			"--pr-number only resolves the head branch; unlike the app's 'add specific PR' action, it does not "+
			"store the PR itself or its comments — nothing PR-shaped appears until the next sync.",
		"--context-id UUID | --context-key STRING (one required)\n"+
			"--owner-repo STRING (required; 'owner/repo')\n"+
			"--branch STRING (optional)\n"+
			"--pr-number INT (optional; used only when --branch is empty)\n"+
			"--label STRING (optional)\n"+
			"--platform STRING (optional; default 'github')",
		`{"id":"uuid","contextId":"uuid","repoId":"uuid","branch":"string","label":"string","createdAt":"RFC3339","repo":{"id":"uuid","platform":"github","owner":"string","name":"string"}}`,
		`vibecodeway-cli branch-add --context-key sprint-42 --owner-repo juna-health/frontend --branch feat/JH-123-login`,
	)
}
