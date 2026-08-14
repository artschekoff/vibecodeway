package vcsstore

import "context"

// RepoWithLocations is a tracked repo enriched with every local checkout path
// and git-worktree workspace registered for it. Paths and Workspaces are always
// non-nil so they serialize as [] rather than null.
type RepoWithLocations struct {
	VCSRepo
	Paths      []VCSRepoPath  `json:"paths"`
	Workspaces []VCSWorkspace `json:"workspaces"`
}

// ListReposWithLocations returns all tracked repositories, each with its local
// filesystem paths (vcs_repo_paths) and workspaces (vcs_workspaces git worktrees).
// Repos are ordered platform, owner, name; repos with no paths/workspaces are
// still returned with empty arrays.
func (s *Store) ListReposWithLocations(ctx context.Context) ([]RepoWithLocations, error) {
	repos, err := s.Repos.ListRepos(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make([]RepoWithLocations, 0, len(repos))
	for _, r := range repos {
		paths, err := s.RepoPaths.ListForRepo(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		if paths == nil {
			paths = []VCSRepoPath{}
		}
		workspaces, err := s.Workspaces.ListByRepo(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		if workspaces == nil {
			workspaces = []VCSWorkspace{}
		}
		out = append(out, RepoWithLocations{VCSRepo: r, Paths: paths, Workspaces: workspaces})
	}
	return out, nil
}
