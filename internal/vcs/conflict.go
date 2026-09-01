package vcs

import (
	"fmt"
	"os/exec"
)

// isConflictCode reports whether a 2-char porcelain XY code means "git stopped
// here and wants a human". Most unmerged states carry a U on one side, but
// both-added (AA) and both-deleted (DD) do not — git lists them as unmerged all
// the same, so matching on U alone would let them through as ordinary changes.
func isConflictCode(code string) bool {
	if len(code) < 2 {
		return false
	}
	switch code {
	case "AA", "DD":
		return true
	}
	return code[0] == 'U' || code[1] == 'U'
}

// ResolveConflict settles a conflicted file by taking one side wholesale and
// staging the result — `git add` is what actually clears the unmerged entry from
// the index. `side` is "ours" (what was already on this branch) or "theirs" (what
// is being merged in). Editing the conflict markers by hand and staging the file
// is the other, finer-grained route; this is the whole-file shortcut.
func ResolveConflict(repoPath, path, side string) error {
	if side != "ours" && side != "theirs" {
		return fmt.Errorf("resolve %s: side must be \"ours\" or \"theirs\", got %q", path, side)
	}
	if out, err := exec.Command("git", "-C", repoPath, "checkout", "--"+side, "--", path).CombinedOutput(); err != nil {
		return fmt.Errorf("take %s for %s: %w: %s", side, path, err, out)
	}
	if out, err := exec.Command("git", "-C", repoPath, "add", "--", path).CombinedOutput(); err != nil {
		return fmt.Errorf("stage resolved %s: %w: %s", path, err, out)
	}
	return nil
}
