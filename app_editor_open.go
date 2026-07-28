package main

import (
	"fmt"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// EditorRepoChoice is a repo the user can pick to checkout when a context has no
// ready workspace yet. Paths are the repo's configured local checkouts.
type EditorRepoChoice struct {
	RepoID   string                 `json:"repoId"`
	RepoName string                 `json:"repoName"` // owner/name
	Paths    []vcsstore.VCSRepoPath `json:"paths"`
}

// EditorWorktree is a ready git worktree the editor can open into.
type EditorWorktree struct {
	RepoID       string `json:"repoId"`
	RepoName     string `json:"repoName"` // owner/name
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktreePath"`
}

// ContextEditorPrep tells the frontend how to open the in-app editor for a
// context: either a workspace is ready (open directly), or the user must pick a
// repo + branch to checkout first.
type ContextEditorPrep struct {
	Ready     bool               `json:"ready"`     // at least one worktree is ready
	Worktrees []EditorWorktree   `json:"worktrees"` // the ready worktrees (for choosing which to open)
	Choices   []EditorRepoChoice `json:"choices"`   // repos with a local path, for the checkout picker (when !Ready)
}

// contextWorktrees ensures (creates or finds) a git worktree for every named
// branch attached to the context and returns them all. Per-repo failures are
// logged and skipped (empty-branch links produce no worktree).
func (a *App) contextWorktrees(contextID string) ([]EditorWorktree, error) {
	branches, err := a.ListVCSBranches(contextID)
	if err != nil {
		return nil, fmt.Errorf("list vcs branches: %w", err)
	}
	var wts []EditorWorktree
	for _, b := range branches {
		wt, werr := a.ensureWorktreeForBranch(b.RepoID, b.Branch)
		if werr != nil {
			a.logger.Sugar().Warnf("contextWorktrees: worktree for repo %s branch %s failed: %s", b.RepoID, b.Branch, werr.Error())
			continue
		}
		if wt == "" {
			continue
		}
		name := b.RepoID
		if repo, rerr := a.vcsStore.Repos.GetRepo(a.ctx, b.RepoID); rerr == nil {
			name = repo.FullName()
		}
		wts = append(wts, EditorWorktree{RepoID: b.RepoID, RepoName: name, Branch: b.Branch, WorktreePath: wt})
	}
	return wts, nil
}

// EnsureContextEditorWorkspace prepares to open the in-app editor for a context.
// It auto-creates/finds worktrees for the context's named branches; if at least
// one is ready the editor can open straight into it. Otherwise it returns the
// linked repos that have a local path so the frontend can offer a
// repo → branch → checkout picker. No new git work beyond contextWorktrees.
func (a *App) EnsureContextEditorWorkspace(contextID string) (*ContextEditorPrep, error) {
	worktrees, err := a.contextWorktrees(contextID)
	if err != nil {
		return nil, err
	}
	if len(worktrees) > 0 {
		return &ContextEditorPrep{Ready: true, Worktrees: worktrees}, nil
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
