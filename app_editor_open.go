package main

import (
	"fmt"

	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
)

// EditorRepoChoice is a repo the user can pick to checkout when a context has no
// ready workspace yet. Paths are the repo's configured local checkouts.
type EditorRepoChoice struct {
	RepoID   string                 `json:"repoId"`
	RepoName string                 `json:"repoName"` // owner/name
	Paths    []vcsstore.VCSRepoPath `json:"paths"`
}

// EditorWorktree is a branch target the editor can open into; WorktreePath is
// empty until a worktree exists for it.
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
	Ready     bool               `json:"ready"`     // at least one named branch to open into
	Worktrees []EditorWorktree   `json:"worktrees"` // branch targets; WorktreePath is "" until checked out
	Choices   []EditorRepoChoice `json:"choices"`   // repos with a local path, for the checkout picker (when !Ready)
}

// contextBranchTargets lists the context's named branch links with the repo
// name and, when a workspace already exists for one, its path. It never
// checks anything out: the user picks a target first, and only that one gets
// a worktree (and the on-checkout script).
func (a *App) contextBranchTargets(contextID string) ([]EditorWorktree, error) {
	branches, err := a.ListVCSBranches(contextID)
	if err != nil {
		return nil, fmt.Errorf("list vcs branches: %w", err)
	}
	var out []EditorWorktree
	for _, b := range branches {
		if b.Branch == "" {
			continue
		}
		// A repo with no local checkout cannot become a workspace.
		if paths, perr := a.vcsStore.RepoPaths.ListForRepo(a.ctx, b.RepoID); perr != nil || len(paths) == 0 {
			continue
		}
		name := b.RepoID
		if repo, rerr := a.vcsStore.Repos.GetRepo(a.ctx, b.RepoID); rerr == nil {
			name = repo.FullName()
		}
		t := EditorWorktree{RepoID: b.RepoID, RepoName: name, Branch: b.Branch}
		if ws, _ := a.FindWorkspaceForBranch(b.RepoID, b.Branch); ws != nil {
			t.WorktreePath = ws.WorktreePath
		}
		out = append(out, t)
	}
	return out, nil
}

// EnsureContextEditorWorkspace prepares to open the in-app editor for a context.
// With named branch links it returns them as targets (WorktreePath set when a
// workspace already exists); the frontend lets the user pick one and calls
// EnsureBranchWorktree for just that target. Otherwise it returns the linked
// repos that have a local path so the frontend can offer a repo → branch →
// checkout picker. Nothing is checked out here.
func (a *App) EnsureContextEditorWorkspace(contextID string) (*ContextEditorPrep, error) {
	targets, err := a.contextBranchTargets(contextID)
	if err != nil {
		return nil, err
	}
	if len(targets) > 0 {
		return &ContextEditorPrep{Ready: true, Worktrees: targets}, nil
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
