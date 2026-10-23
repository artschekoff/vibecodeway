package vcs

import (
	"os/exec"
	"strings"
)

type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	// Diff is always empty: diffs are computed lazily via FileDiffText when a
	// file is opened (VS Code-style — status lists names only). The field stays
	// so the generated frontend model shape doesn't churn.
	Diff         string `json:"diff"`
	IndexStatus  string `json:"indexStatus"`  // staged-side status word; "" when not staged
	WorkStatus   string `json:"workStatus"`   // worktree-side status word; "" when clean in worktree
	CommitStatus string `json:"commitStatus"` // status vs the base branch (branch commits); "" when only a working-tree change
	// Conflicted marks a file git left unmerged. It gets its own flag rather than
	// riding on IndexStatus/WorkStatus because an unmerged code (e.g. "UU") sets
	// BOTH sides, which listed the file under staged AND local at once.
	Conflicted bool `json:"conflicted"`
}

func statusWord(code string) string {
	switch {
	case isConflictCode(code):
		return "conflict"
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

// DefaultStatusLimit mirrors VS Code's `git.statusLimit` default: list at most
// this many changed files per repo. 0 means no limit.
const DefaultStatusLimit = 10000

// RepoDiff is RepoDiffWithLimit at the default limit, discarding the
// hit-limit flag — the signature the existing callers and tests use.
func RepoDiff(repoPath string) ([]FileChange, error) {
	files, _, err := RepoDiffWithLimit(repoPath, DefaultStatusLimit)
	return files, err
}

// RepoDiffWithLimit lists changes for repoPath. It combines two views, always queried
// live from git (no caching):
//  1. working-tree changes vs HEAD (staged/unstaged/untracked) — via git status,
//  2. the branch's committed changes vs its base branch — via git diff
//     <merge-base> HEAD. This is what surfaces a PR/branch's files in a freshly
//     checked-out worktree, where the working tree is clean and git status is
//     empty.
//
// The two views are independent (no cross-group dedup): a file that is part of
// the branch AND has a local uncommitted edit appears in both — once with
// CommitStatus (the PR stays complete, GitHub-style) and once with
// index/work status (the local change).
//
// limit caps the total number of listed files (VS Code's git.statusLimit:
// default 10000, 0 = unlimited). When the cap is hit the list is truncated
// and didHitLimit is true so the UI can warn instead of silently dropping.
func RepoDiffWithLimit(repoPath string, limit int) (files []FileChange, didHitLimit bool, err error) {
	// git status --porcelain gives the file list + codes. --untracked-files=all
	// expands wholly-untracked directories into their individual files instead of
	// collapsing them to a single "dir/" entry (which the editor can't open/diff).
	out, err := exec.Command("git", "-C", repoPath, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		return nil, false, err
	}
	var changes []FileChange
	atLimit := func() bool { return limit > 0 && len(changes) >= limit }
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		if atLimit() {
			didHitLimit = true
			break
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		// Renames are "old -> new"; take the new path.
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		status := statusWord(code)
		conflicted := isConflictCode(code)
		indexStatus, workStatus := splitStatus(code)
		// A conflict belongs to the conflicts group alone; leaving the unmerged
		// word on both sides double-listed it under staged and local.
		if conflicted {
			indexStatus, workStatus = "", ""
		}
		changes = append(changes, FileChange{Path: path, Status: status, IndexStatus: indexStatus, WorkStatus: workStatus, Conflicted: conflicted})
	}

	// Append the branch's committed changes vs its base branch.
	if mergeBase := MergeBaseAgainstDefault(repoPath); mergeBase != "" {
		nsOut, nsErr := exec.Command("git", "-C", repoPath, "diff", "--name-status", mergeBase, "HEAD").Output()
		if nsErr == nil {
			for _, line := range strings.Split(strings.TrimRight(string(nsOut), "\n"), "\n") {
				fields := strings.Split(line, "\t")
				if len(fields) < 2 {
					continue
				}
				if atLimit() {
					didHitLimit = true
					break
				}
				code := fields[0]             // e.g. "M", "A", "D", "R100"
				path := fields[len(fields)-1] // rename: old \t new — take new
				word := statusLetterWord(code[0])
				if word == "" {
					word = "modified"
				}
				changes = append(changes, FileChange{Path: path, Status: word, CommitStatus: word})
			}
		}
	}
	return changes, didHitLimit, nil
}

// FileDiffText computes one file's unified diff on demand. RepoDiff
// deliberately returns no diff text: a worktree with thousands of untracked
// files (an npm cache, a tool dir) used to fork one `git diff` per file and
// ship the whole content over IPC on every poll — minutes per refresh. Like
// VS Code's git extension, status lists names and the diff loads when a file
// is actually opened.
//
// mode: "untracked" (add-diff vs /dev/null), "branch" (merge-base vs HEAD),
// anything else = working tree vs HEAD.
func FileDiffText(repoPath, path, mode string) (string, error) {
	switch mode {
	case "untracked":
		// no-index exits non-zero when files differ — expected, ignore it.
		out, _ := exec.Command("git", "-C", repoPath, "diff", "--no-index", "--", "/dev/null", path).Output()
		return string(out), nil
	case "branch":
		mergeBase := MergeBaseAgainstDefault(repoPath)
		if mergeBase == "" {
			return "", nil
		}
		out, err := exec.Command("git", "-C", repoPath, "diff", mergeBase, "HEAD", "--", path).Output()
		return string(out), err
	default:
		out, err := exec.Command("git", "-C", repoPath, "diff", "HEAD", "--", path).Output()
		return string(out), err
	}
}

// defaultBaseCandidates lists the refs, in priority order, that RepoDiff treats
// as a branch's "base" for computing branch-vs-base changes.
var defaultBaseCandidates = []string{"origin/main", "origin/master", "main", "master"}

// MergeBaseAgainstDefault returns the merge-base commit between HEAD and the
// repo's base branch (origin/HEAD if set, else the first existing of
// origin/main, origin/master, main, master), queried live. It returns "" when
// no base can be determined or when the base is HEAD itself (nothing to diff) —
// e.g. the main checkout sitting on the default branch.
func MergeBaseAgainstDefault(repoPath string) string {
	base := detectBaseRef(repoPath)
	if base == "" {
		return ""
	}
	headSHA, err := revParse(repoPath, "HEAD")
	if err != nil {
		return ""
	}
	baseSHA, err := revParse(repoPath, base)
	if err != nil || baseSHA == headSHA {
		return "" // base is HEAD (on the base branch) — no branch changes
	}
	mb, err := exec.Command("git", "-C", repoPath, "merge-base", base, "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(mb))
	if sha == "" || sha == headSHA {
		return ""
	}
	return sha
}

// detectBaseRef returns the ref name of the repo's base branch, or "".
func detectBaseRef(repoPath string) string {
	// origin/HEAD, when set, names the remote's default branch.
	if out, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD").Output(); err == nil {
		if ref := strings.TrimSpace(string(out)); ref != "" {
			return ref
		}
	}
	for _, ref := range defaultBaseCandidates {
		if _, err := revParse(repoPath, ref); err == nil {
			return ref
		}
	}
	return ""
}

func revParse(repoPath, ref string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
