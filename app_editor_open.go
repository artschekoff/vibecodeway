package main

import (
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// EditorRepoChoice is a repo the user can pick to checkout when a context has no
// ready workspace yet. Paths are the repo's configured local checkouts.
type EditorRepoChoice struct {
	RepoID   string                 `json:"repoId"`
	RepoName string                 `json:"repoName"` // owner/name
	Paths    []vcsstore.VCSRepoPath `json:"paths"`
}

// ContextEditorPrep tells the frontend how to open the in-app editor for a
// context: either a workspace is ready (open directly), or the user must pick a
// repo + branch to checkout first.
type ContextEditorPrep struct {
	Ready   bool               `json:"ready"`   // at least one worktree is ready
	Choices []EditorRepoChoice `json:"choices"` // repos with a local path, for the picker (when !Ready)
}

// EnsureContextEditorWorkspace prepares to open the in-app editor for a context.
// It auto-creates/finds worktrees for the context's named branches; if at least
// one is ready the editor can open straight into it. Otherwise it returns the
// linked repos that have a local path so the frontend can offer a
// repo → branch → checkout picker. No new git work beyond ensureContextWorkspaces.
func (a *App) EnsureContextEditorWorkspace(contextID string) (*ContextEditorPrep, error) {
	firstWorktree, err := a.ensureContextWorkspaces(contextID)
	if err != nil {
		return nil, err
	}
	if firstWorktree != "" {
		return &ContextEditorPrep{Ready: true}, nil
	}

	branches, err := a.ListVCSBranches(contextID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var choices []EditorRepoChoice
	for _, b := range branches {
		if seen[b.RepoID] {
			continue
		}
		paths, perr := a.vcsStore.RepoPaths.ListForRepo(a.ctx, b.RepoID)
		if perr != nil || len(paths) == 0 {
			continue
		}
		repo, rerr := a.vcsStore.Repos.GetRepo(a.ctx, b.RepoID)
		if rerr != nil {
			continue
		}
		seen[b.RepoID] = true
		choices = append(choices, EditorRepoChoice{
			RepoID:   b.RepoID,
			RepoName: repo.FullName(),
			Paths:    paths,
		})
	}
	return &ContextEditorPrep{Ready: false, Choices: choices}, nil
}
