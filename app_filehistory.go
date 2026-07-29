package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FileCommit is one entry in a file's git history.
type FileCommit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// FileDiff is a file's content on both sides of a single commit.
type FileDiff struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// BlameLine is the commit responsible for one line of the working-tree file.
type BlameLine struct {
	Line    int    `json:"line"`
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
}

// fieldSep is an ASCII unit separator — safe inside commit subjects.
const fieldSep = "\x1f"

var hashRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// FileHistory returns the commits that touched path, newest first, following
// renames. Empty slice when the file has no history (or an escaping path).
func (a *App) FileHistory(repoPath, path string) ([]FileCommit, error) {
	if _, err := safeJoin(repoPath, path); err != nil {
		return []FileCommit{}, nil
	}
	format := "%H" + fieldSep + "%an" + fieldSep + "%aI" + fieldSep + "%s"
	out, err := exec.Command("git", "-C", repoPath, "log", "--follow", "--format="+format, "--", path).Output()
	if err != nil {
		return []FileCommit{}, nil // no history / not tracked
	}
	var commits []FileCommit
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, fieldSep, 4)
		if len(parts) != 4 {
			continue
		}
		commits = append(commits, FileCommit{Hash: parts[0], Author: parts[1], Date: parts[2], Subject: parts[3]})
	}
	return commits, nil
}

// FileDiffAtCommit returns the file's content before and after the given commit,
// for rendering a Monaco diff of that single commit. `before` is empty when the
// commit added the file; `after` is empty when it deleted it. hash must be hex.
func (a *App) FileDiffAtCommit(repoPath, path, hash string) (*FileDiff, error) {
	if _, jerr := safeJoin(repoPath, path); jerr != nil {
		return nil, fmt.Errorf("path escapes repository")
	}
	if !hashRe.MatchString(hash) {
		return nil, fmt.Errorf("invalid commit hash")
	}
	var d FileDiff
	// Missing object (file added/deleted at this commit) is not an error — it maps
	// to an empty side of the diff.
	if out, gerr := exec.Command("git", "-C", repoPath, "show", hash+"^:"+path).Output(); gerr == nil {
		d.Before = string(out)
	}
	if out, gerr := exec.Command("git", "-C", repoPath, "show", hash+":"+path).Output(); gerr == nil {
		d.After = string(out)
	}
	return &d, nil
}

// FileBlame returns per-line blame for the working-tree file. nil for an
// untracked/escaping path (advisory feature — never an error the UI must handle).
func (a *App) FileBlame(repoPath, path string) ([]BlameLine, error) {
	if _, err := safeJoin(repoPath, path); err != nil {
		return nil, nil
	}
	out, err := exec.Command("git", "-C", repoPath, "blame", "--porcelain", "--", path).Output()
	if err != nil {
		return nil, nil
	}
	return parseBlamePorcelain(string(out)), nil
}

// parseBlamePorcelain turns `git blame --porcelain` output into one BlameLine per
// source line. Commit metadata (author/time/summary) appears only the first time
// a commit is seen, so we cache it by hash.
func parseBlamePorcelain(out string) []BlameLine {
	type meta struct{ author, date, summary string }
	cache := map[string]*meta{}
	var (
		lines   []BlameLine
		curHash string
		curLine int
	)
	headerRe := regexp.MustCompile(`^([0-9a-f]{40}) \d+ (\d+)`)
	for _, line := range strings.Split(out, "\n") {
		if m := headerRe.FindStringSubmatch(line); m != nil {
			curHash = m[1]
			curLine, _ = strconv.Atoi(m[2])
			if cache[curHash] == nil {
				cache[curHash] = &meta{}
			}
			continue
		}
		md := cache[curHash]
		switch {
		case md == nil:
			// content or field before a header — ignore
		case strings.HasPrefix(line, "author "):
			md.author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-time "):
			if epoch, e := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64); e == nil {
				md.date = time.Unix(epoch, 0).UTC().Format("2006-01-02")
			}
		case strings.HasPrefix(line, "summary "):
			md.summary = strings.TrimPrefix(line, "summary ")
		case strings.HasPrefix(line, "\t"):
			short := curHash
			if len(short) > 8 {
				short = short[:8]
			}
			lines = append(lines, BlameLine{
				Line: curLine, Commit: short, Author: md.author, Date: md.date, Summary: md.summary,
			})
		}
	}
	return lines
}
