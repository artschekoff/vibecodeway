package vcs

import (
	"os/exec"
	"strings"
)

type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Diff   string `json:"diff"`
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
		changes = append(changes, FileChange{Path: path, Status: status, Diff: diff})
	}
	return changes, nil
}
