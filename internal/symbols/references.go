package symbols

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Reference is one usage site of a symbol name.
type Reference struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Preview string `json:"preview"`
}

// References lists word-boundary usages of name via `git grep -nw`. It needs
// only git, so it works even when ctags is not installed. grep's exit code 1
// (no matches) is a normal empty result.
func References(worktree, name string, limit int) ([]Reference, error) {
	if limit <= 0 {
		limit = 200
	}
	cmd := exec.Command("git", "-C", worktree, "grep", "-nw", "--untracked", "--", name)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil // no matches
		}
		return nil, fmt.Errorf("git grep in %s: %w", worktree, err)
	}
	var refs []Reference
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		// Format: path:line:content — content may itself contain colons.
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		n, convErr := strconv.Atoi(parts[1])
		if convErr != nil {
			continue
		}
		refs = append(refs, Reference{Path: parts[0], Line: n, Preview: strings.TrimSpace(parts[2])})
		if len(refs) >= limit {
			break
		}
	}
	return refs, nil
}
