package main

import (
	"context"
	"fmt"

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
