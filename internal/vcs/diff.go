package vcs

import (
	"os/exec"
	"strings"
)

type FileChange struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	Diff        string `json:"diff"`
	IndexStatus string `json:"indexStatus"` // staged-side status word; "" when not staged
	WorkStatus  string `json:"workStatus"`  // worktree-side status word; "" when clean in worktree
}

func statusWord(code string) string {
	switch {
	case strings.HasPrefix(code, "??"):
		return "untracked"
	case strings.ContainsAny(code, "A"):
		return "added"
	case strings.ContainsAny(code, "D"):
		return "deleted"
	case strings.ContainsAny(code, "R"):
		return "renamed"
	default:
		return "modified"
	}
}

// statusLetterWord maps a single porcelain status letter (one side of the XY
// code) to a word. A space (clean on that side) maps to "".
func statusLetterWord(c byte) string {
	switch c {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'U':
		return "unmerged"
	case '?':
		return "untracked"
	default:
		return ""
	}
}

// splitStatus turns a 2-char porcelain code (XY) into (indexStatus, workStatus).
// Untracked ("??") is treated as an unstaged-only untracked change.
func splitStatus(code string) (indexStatus, workStatus string) {
	if code == "??" {
		return "", "untracked"
	}
	return statusLetterWord(code[0]), statusLetterWord(code[1])
}

// RepoDiff lists working-tree changes vs HEAD (plus untracked files) for repoPath.
func RepoDiff(repoPath string) ([]FileChange, error) {
	// git status --porcelain gives the file list + codes.
	out, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		// Renames are "old -> new"; take the new path.
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		status := statusWord(code)
		indexStatus, workStatus := splitStatus(code)
		var diff string
		if status == "untracked" {
			// Show the new file's content as an add-diff (no-index; non-zero exit is expected on diff).
			d, _ := exec.Command("git", "-C", repoPath, "diff", "--no-index", "--", "/dev/null", path).Output()
			diff = string(d)
		} else {
			d, derr := exec.Command("git", "-C", repoPath, "diff", "HEAD", "--", path).Output()
			if derr == nil {
				diff = string(d)
			}
		}
		changes = append(changes, FileChange{Path: path, Status: status, Diff: diff, IndexStatus: indexStatus, WorkStatus: workStatus})
	}
	return changes, nil
}
