package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initLogRepo builds a throwaway repository with three commits on fixed dates,
// one of them on a branch, so branch coverage and date filtering are both
// exercised against real git rather than a parser fixture.
func initLogRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	commit := func(date, message, file string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(message), 0o644))
		run(nil, "add", ".")
		run([]string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-m", message)
	}

	run(nil, "init", "--initial-branch=main")
	run(nil, "config", "user.email", "dev@example.com")
	run(nil, "config", "user.name", "Dev Example")

	commit("2026-09-02T12:00:00+00:00", "wednesday work", "a.txt")
	commit("2026-09-04T12:00:00+00:00", "friday work", "b.txt")
	// On a branch: a question about Friday must still find work that never
	// reached main.
	run(nil, "checkout", "-b", "feature")
	commit("2026-09-04T18:00:00+00:00", "friday branch work", "c.txt")
	run(nil, "checkout", "main")
	return dir
}

func subjects(commits []Commit) []string {
	out := make([]string, 0, len(commits))
	for _, c := range commits {
		out = append(out, c.Subject)
	}
	return out
}

func TestGitLogFiltersToASingleDay(t *testing.T) {
	dir := initLogRepo(t)

	// Bare dates on both ends: GitLog pins them to the whole day itself.
	got, err := GitLog(dir, LogOptions{Since: "2026-09-04", Until: "2026-09-04"})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"friday work", "friday branch work"}, subjects(got))
}

// --all, not the checked-out branch: the repo is on main, but Friday's work
// includes a commit that only exists on `feature`.
func TestGitLogSeesCommitsOnOtherBranches(t *testing.T) {
	dir := initLogRepo(t)

	got, err := GitLog(dir, LogOptions{Since: "2026-09-04", Until: "2026-09-04"})
	require.NoError(t, err)

	assert.Contains(t, subjects(got), "friday branch work")
}

func TestGitLogReturnsBothDatesAndIdentity(t *testing.T) {
	dir := initLogRepo(t)

	got, err := GitLog(dir, LogOptions{Since: "2026-09-04", Until: "2026-09-04", Limit: 1})
	require.NoError(t, err)
	require.Len(t, got, 1)

	c := got[0]
	assert.NotEmpty(t, c.Hash)
	assert.Equal(t, "Dev Example", c.Author)
	assert.Equal(t, "dev@example.com", c.Email)
	assert.Contains(t, c.AuthorDate, "2026-09-04")
	assert.Contains(t, c.CommitDate, "2026-09-04")
}

func TestGitLogFiltersByAuthor(t *testing.T) {
	dir := initLogRepo(t)

	got, err := GitLog(dir, LogOptions{Since: "2026-09-01", Author: "dev@example.com"})
	require.NoError(t, err)
	assert.Len(t, got, 3)

	got, err = GitLog(dir, LogOptions{Since: "2026-09-01", Author: "someone-else@example.com"})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGitLogEmptyWhenNothingInRange(t *testing.T) {
	dir := initLogRepo(t)

	got, err := GitLog(dir, LogOptions{Since: "2026-09-05", Until: "2026-09-06"})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGitLogErrorsOutsideARepository(t *testing.T) {
	_, err := GitLog(t.TempDir(), LogOptions{Since: "2026-09-01"})
	assert.Error(t, err)
}

func TestPinTimeOnlyExpandsBareDates(t *testing.T) {
	assert.Equal(t, "2026-09-04T00:00:00", pinTime("2026-09-04", "T00:00:00"))
	assert.Equal(t, "2026-09-04T23:59:59", pinTime("2026-09-04", "T23:59:59"))
	// Already carries a time, or is a relative expression git understands: leave
	// it alone rather than corrupting it.
	assert.Equal(t, "2026-09-04T10:00:00", pinTime("2026-09-04T10:00:00", "T23:59:59"))
	assert.Equal(t, "yesterday", pinTime("yesterday", "T00:00:00"))
	assert.Equal(t, "", pinTime("", "T00:00:00"))
}

func TestGitAuthorEmail(t *testing.T) {
	assert.Equal(t, "dev@example.com", GitAuthorEmail(initLogRepo(t)))
	// A path git cannot enter at all yields no identity, and no crash. (A plain
	// non-repo directory is NOT this case — git happily answers there from the
	// user's global config, which is the intended fallback.)
	assert.Equal(t, "", GitAuthorEmail(filepath.Join(t.TempDir(), "does-not-exist")))
}

// A subject containing punctuation must not be mistaken for a field boundary.
func TestParseGitLogHandlesAwkwardSubjects(t *testing.T) {
	rec := "abc123" + logFieldSep + "Dev" + logFieldSep + "d@e.com" + logFieldSep +
		"2026-09-04T12:00:00+00:00" + logFieldSep + "2026-09-04T12:00:00+00:00" + logFieldSep +
		"fix: handle a|b, tabs\tand \"quotes\"" + logRecordSep

	got := parseGitLog(rec)
	require.Len(t, got, 1)
	assert.Equal(t, `fix: handle a|b, tabs	and "quotes"`, got[0].Subject)
}

func TestParseGitLogSkipsMalformedRecords(t *testing.T) {
	good := "abc" + logFieldSep + "D" + logFieldSep + "d@e" + logFieldSep + "d1" + logFieldSep + "d2" + logFieldSep + "s" + logRecordSep
	assert.Len(t, parseGitLog("only-one-field"+logRecordSep+good), 1)
	assert.Empty(t, parseGitLog(""))
}
