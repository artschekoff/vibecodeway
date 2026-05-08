package vcs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// LocalBranch describes one local git branch.
type LocalBranch struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	IsGone    bool   `json:"isGone"` // remote tracking branch no longer exists
}

// GitListLocalBranches returns all local branches for the repo at dir.
func GitListLocalBranches(dir string) ([]LocalBranch, error) {
	cmd := exec.Command("git", "-C", dir, "branch", "-vv")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch -vv in %s: %w", dir, err)
	}
	return parseLocalBranches(out), nil
}

func parseLocalBranches(data []byte) []LocalBranch {
	var result []LocalBranch
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}
		isCurrent := line[0] == '*'
		rest := strings.TrimSpace(line[2:])
		// Skip detached HEAD entries
		if strings.HasPrefix(rest, "(") {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		// fields[1] is the hash; everything after is commit info and optional [remote: gone]
		trailing := strings.Join(fields[2:], " ")
		isGone := strings.Contains(trailing, ": gone]")
		result = append(result, LocalBranch{Name: name, IsCurrent: isCurrent, IsGone: isGone})
	}
	return result
}

// GitDeleteLocalBranch force-deletes a local branch in the repo at dir.
func GitDeleteLocalBranch(dir, branch string) error {
	cmd := exec.Command("git", "-C", dir, "branch", "-D", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch -D %s in %s: %w\n%s", branch, dir, err, out)
	}
	return nil
}
