package vcs

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Commit is one entry from a repository's history.
type Commit struct {
	Hash       string `json:"hash"`
	Author     string `json:"author"`
	Email      string `json:"email"`
	AuthorDate string `json:"authorDate"` // when the work was written
	CommitDate string `json:"commitDate"` // when it last landed; a rebase moves this
	Subject    string `json:"subject"`
}

// LogOptions filters a GitLog call. Since and Until accept anything git's date
// parser understands ("2026-09-04", "2026-09-04T23:59:59", "3 days ago"); a bare
// YYYY-MM-DD is pinned to the whole day, so Since is inclusive from 00:00:00 and
// Until inclusive to 23:59:59. Author is matched as a substring against
// both the author's name and email. Limit 0 means no cap.
type LogOptions struct {
	Since  string
	Until  string
	Author string
	Limit  int
}

// ASCII unit and record separators. Neither can appear in a commit subject, so
// splitting on them cannot be confused by punctuation the way a "|" would be.
const (
	logFieldSep  = "\x1f"
	logRecordSep = "\x1e"
)

// GitLog returns commits from the repository at dir matching opts, newest first.
//
// --all walks every ref rather than just the checked-out branch, so work done on
// a feature branch still shows up. Linked worktrees share the repository's refs
// and object database, so scanning the main checkout also covers them — walking
// worktree paths separately would only double-report the same commits.
//
// ponytail: git's --since/--until filter on committer date while the work was
// written at the author date, so a rebased commit is dated by its rebase. Both
// fields are returned rather than picking one, since only the caller knows which
// question is being asked.
func GitLog(dir string, opts LogOptions) ([]Commit, error) {
	format := strings.Join([]string{"%H", "%an", "%ae", "%aI", "%cI", "%s"}, logFieldSep) + logRecordSep
	// --exclude must come BEFORE --all to take effect: git applies --exclude to
	// the ref globs that follow it on the command line, not retroactively. --all
	// expands to every ref under refs/, including refs/stash, and a single `git
	// stash` produces two commits ("WIP on ...", "index on ...") carrying the
	// user's own identity — they pass the author and date filters and get
	// reported as real work. Verified against git 2.50.1.
	args := []string{"-C", dir, "log", "--exclude=refs/stash", "--all", "--pretty=format:" + format}
	if since := pinTime(opts.Since, "T00:00:00"); since != "" {
		args = append(args, "--since="+since)
	}
	if until := pinTime(opts.Until, "T23:59:59"); until != "" {
		args = append(args, "--until="+until)
	}
	if opts.Author != "" {
		// -F: LogOptions.Author is documented as a plain substring match, but git
		// treats --author as a regex by default. Without -F, a metacharacter
		// either matches unintended commits or, for something like an unbalanced
		// "[", makes git exit 128 — which the caller can only read as "unreadable
		// path" and silently reports zero commits.
		args = append(args, "-F", "--author="+opts.Author)
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", opts.Limit))
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", dir, err)
	}
	return parseGitLog(string(out)), nil
}

func parseGitLog(data string) []Commit {
	var commits []Commit
	for _, rec := range strings.Split(data, logRecordSep) {
		rec = strings.TrimLeft(rec, "\r\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, logFieldSep)
		if len(f) != 6 {
			continue // malformed: skip rather than mis-attribute fields
		}
		commits = append(commits, Commit{
			Hash:       f[0],
			Author:     f[1],
			Email:      f[2],
			AuthorDate: f[3],
			CommitDate: f[4],
			Subject:    f[5],
		})
	}
	return commits
}

// pinTime gives a bare YYYY-MM-DD bound an explicit time of day.
//
// git's approxidate parser fills a missing time-of-day from the CURRENT CLOCK
// rather than from midnight: asked at 12:17, "--since=2026-09-04" becomes a
// 12:17 cutoff and silently drops that morning's commits, so the same question
// answers differently depending on when it is asked. Verified against git
// 2.50.1. Pinning happens here, in the only place that builds a git invocation,
// so no caller can reintroduce the trap.
//
// Values that already carry a time, or that are git's own relative expressions
// ("yesterday", "3 days ago"), pass through untouched.
func pinTime(v, timeOfDay string) string {
	if v == "" {
		return ""
	}
	// A caller (or model) may write a non-zero-padded date like "2026-9-4".
	// time.Parse("2006-01-02", ...) rejects that outright, so without
	// normalising it here the value falls through unpinned, reintroducing the
	// exact approxidate trap this function exists to close.
	if m := bareDatePattern.FindStringSubmatch(v); m != nil {
		v = fmt.Sprintf("%s-%02s-%02s", m[1], m[2], m[3])
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return v
	}
	return v + timeOfDay
}

// bareDatePattern matches a YYYY-M-D date with optionally unpadded month/day.
var bareDatePattern = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)

// GitAuthorEmail returns the user.email git would attribute a commit in dir to,
// or "" when the path cannot be read at all. Used as the default "me" when a
// history query does not name an author.
//
// This deliberately inherits git's own resolution order, so a repository with no
// local user.email reports the global identity — the same address its commits
// actually carry.
func GitAuthorEmail(dir string) string {
	out, err := exec.Command("git", "-C", dir, "config", "user.email").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
