package vcs

import (
	"fmt"
	"os/exec"
	"strings"
)

// runGit runs `git -C dir <args...>` and wraps failures with combined output.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s in %s: %w\n%s", strings.Join(args, " "), dir, err, out)
	}
	return nil
}

// GitStageFile stages path (adds it to the index). Works for tracked and untracked files.
func GitStageFile(dir, path string) error {
	return runGit(dir, "add", "--", path)
}

// GitUnstageFile removes path's staged changes from the index, leaving the working tree intact.
func GitUnstageFile(dir, path string) error {
	return runGit(dir, "restore", "--staged", "--", path)
}

// GitRestoreFile resets both the index and the working tree for path back to
// HEAD. It returns an error when path has no HEAD entry (e.g. an untracked
// file); callers discarding an untracked file should delete it instead.
func GitRestoreFile(dir, path string) error {
	return runGit(dir, "restore", "--source=HEAD", "--staged", "--worktree", "--", path)
}
