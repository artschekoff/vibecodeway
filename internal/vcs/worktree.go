package vcs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// WorktreeInfo describes one git worktree entry from `git worktree list --porcelain`.
type WorktreeInfo struct {
	Path   string
	Branch string // empty when HEAD is detached
	HEAD   string
}

// BranchToWorktreeName converts a branch name to a filesystem-safe directory name.
// "feat/foo-bar" → "feat-foo-bar"
func BranchToWorktreeName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// ParseWorktreeListForTest exposes parseWorktreeList for package-external tests.
func ParseWorktreeListForTest(data []byte) []WorktreeInfo {
	return parseWorktreeList(data)
}

// GitWorktreeAdd creates a new worktree at worktreePath checked out to branch.
func GitWorktreeAdd(baseDir, worktreePath, branch string) error {
	cmd := exec.Command("git", "-C", baseDir, "worktree", "add", worktreePath, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add %s: %w\n%s", branch, err, out)
	}
	return nil
}

// GitWorktreeRemove removes a worktree from the repo rooted at baseDir, forcing
// removal even if it has uncommitted changes.
func GitWorktreeRemove(baseDir, worktreePath string) error {
	cmd := exec.Command("git", "-C", baseDir, "worktree", "remove", "--force", worktreePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove %s: %w\n%s", worktreePath, err, out)
	}
	return nil
}

// GitWorktreeList returns all worktrees registered for the repo at baseDir.
func GitWorktreeList(baseDir string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "-C", baseDir, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list in %s: %w", baseDir, err)
	}
	return parseWorktreeList(out), nil
}

func parseWorktreeList(data []byte) []WorktreeInfo {
	var result []WorktreeInfo
	var current WorktreeInfo
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				result = append(result, current)
			}
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if current.Path != "" {
		result = append(result, current)
	}
	return result
}
