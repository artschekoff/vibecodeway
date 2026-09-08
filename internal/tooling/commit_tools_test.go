package tooling

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeRepos stands in for *vcsstore.Store so the tool can be tested against a
// throwaway git repository instead of a populated database.
type fakeRepos struct {
	repos []vcsstore.RepoWithLocations
	err   error
}

func (f fakeRepos) ListReposWithLocations(context.Context) ([]vcsstore.RepoWithLocations, error) {
	return f.repos, f.err
}

// initToolRepo builds a repository with one Wednesday and one Friday commit.
func initToolRepo(t *testing.T) string {
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
	commit("2026-09-02T12:00:00+00:00", "JH-1 wednesday work", "a.txt")
	commit("2026-09-04T12:00:00+00:00", "JH-2 friday work", "b.txt")
	return dir
}

func reposAt(dir string) fakeRepos {
	return fakeRepos{repos: []vcsstore.RepoWithLocations{{
		VCSRepo: vcsstore.VCSRepo{ID: "r1", Platform: "github", Owner: "acme", Name: "app"},
		Paths:   []vcsstore.VCSRepoPath{{ID: "p1", RepoID: "r1", Path: dir}},
	}}}
}

func callListCommits(t *testing.T, d Deps, args map[string]any) map[string]any {
	t.Helper()
	reg := NewRegistry(d)
	out, err := reg.Call(context.Background(), "list_commits", args, Ctx{})
	require.NoError(t, err)
	res, ok := out.(map[string]any)
	require.True(t, ok, "expected a map result, got %T", out)
	return res
}

// Bare dates must cover the whole day at both ends. Git fills a missing
// time-of-day from the current clock, so without pinning, this same query
// returns different results depending on when the test runs.
func TestListCommitsBareDatesCoverTheWholeDay(t *testing.T) {
	dir := initToolRepo(t)
	d := Deps{Repos: reposAt(dir), Logger: zap.NewNop()}

	res := callListCommits(t, d, map[string]any{"since": "2026-09-04", "until": "2026-09-04"})

	require.Equal(t, 1, res["count"])
	commits := res["commits"].([]map[string]any)
	assert.Equal(t, "JH-2 friday work", commits[0]["subject"])
	assert.Equal(t, "acme/app", commits[0]["repo"])
}

// Regression test for the approxidate trap: a commit made early on the target
// day must be found no matter what time of day the query is made. With a bare
// "--since=<date>" this fails for any commit earlier than the current clock time.
func TestListCommitsFindsEarlyMorningCommits(t *testing.T) {
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run(nil, "init", "--initial-branch=main")
	run(nil, "config", "user.email", "dev@example.com")
	run(nil, "config", "user.name", "Dev Example")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644))
	run(nil, "add", ".")
	// 00:05 local on the target day — earlier than any plausible wall-clock time
	// at which the suite runs.
	run([]string{"GIT_AUTHOR_DATE=2026-09-04T00:05:00", "GIT_COMMITTER_DATE=2026-09-04T00:05:00"},
		"commit", "-m", "JH-3 very early friday work")

	d := Deps{Repos: reposAt(dir), Logger: zap.NewNop()}
	res := callListCommits(t, d, map[string]any{"since": "2026-09-04", "until": "2026-09-04"})

	assert.Equal(t, 1, res["count"], "an early-morning commit must not be cut off by the current time of day")
}

func TestListCommitsRequiresSince(t *testing.T) {
	d := Deps{Repos: reposAt(initToolRepo(t)), Logger: zap.NewNop()}
	reg := NewRegistry(d)

	_, err := reg.Call(context.Background(), "list_commits", map[string]any{}, Ctx{})
	assert.Error(t, err)
}

// Without an author the tool falls back to the repository's own git identity, so
// "what did I do" does not silently include everyone else's commits.
func TestListCommitsDefaultsToTheRepositoryIdentity(t *testing.T) {
	dir := initToolRepo(t)
	d := Deps{Repos: reposAt(dir), Logger: zap.NewNop()}

	res := callListCommits(t, d, map[string]any{"since": "2026-09-01"})
	assert.Equal(t, 2, res["count"])

	res = callListCommits(t, d, map[string]any{"since": "2026-09-01", "author": "nobody@example.com"})
	assert.Equal(t, 0, res["count"])
}

// Regression test: when no explicit author is given and the path's git
// identity cannot be resolved at all (no local or global user.email), the
// tool must not silently widen the query to every author on that path —
// vcs.GitLog omits --author entirely when Author is "". The path must be
// skipped and the repo surfaced in reposWithoutGitIdentity instead.
func TestListCommitsSkipsPathsWithNoResolvableIdentity(t *testing.T) {
	// git config user.email falls back to the global config, so merely
	// unsetting the local value is not enough to make a repo identity-less on
	// a machine that has a global identity configured. Point HOME and the git
	// config lookup itself at nowhere for the duration of this test.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_CONFIG_HOME", fakeHome)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "--initial-branch=main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644))
	run("add", ".")
	// -c sets the identity for this one commit only, without writing it to
	// any config file, so the repository itself stays identity-less.
	run("-c", "user.email=someone@example.com", "-c", "user.name=Someone",
		"commit", "-m", "JH-4 someone else's work")

	// Sanity-check the fixture actually produces an unresolvable identity;
	// otherwise this test would pass for the wrong reason.
	require.Empty(t, vcs.GitAuthorEmail(dir), "fixture must have no resolvable git identity")

	d := Deps{Repos: reposAt(dir), Logger: zap.NewNop()}
	res := callListCommits(t, d, map[string]any{"since": "2026-09-01"})

	assert.Equal(t, 0, res["count"], "must not silently fall back to unfiltered history")
	assert.Equal(t, []string{"acme/app"}, res["reposWithoutGitIdentity"])
}

func TestListCommitsFiltersByRepo(t *testing.T) {
	dir := initToolRepo(t)
	d := Deps{Repos: reposAt(dir), Logger: zap.NewNop()}

	res := callListCommits(t, d, map[string]any{"since": "2026-09-01", "repo": "acme/app"})
	assert.Equal(t, 2, res["count"])

	res = callListCommits(t, d, map[string]any{"since": "2026-09-01", "repo": "other/repo"})
	assert.Equal(t, 0, res["count"])
}

// A repo the app tracks but has never cloned locally cannot be searched. Say so
// instead of reporting "no commits", which reads as "you did nothing".
func TestListCommitsReportsReposWithNoLocalCheckout(t *testing.T) {
	d := Deps{Logger: zap.NewNop(), Repos: fakeRepos{repos: []vcsstore.RepoWithLocations{{
		VCSRepo: vcsstore.VCSRepo{ID: "r2", Platform: "github", Owner: "acme", Name: "nolocal"},
		Paths:   []vcsstore.VCSRepoPath{},
	}}}}

	res := callListCommits(t, d, map[string]any{"since": "2026-09-01"})

	assert.Equal(t, 0, res["count"])
	assert.Equal(t, []string{"acme/nolocal"}, res["reposWithoutLocalPath"])
}

// An unreadable path must not fail the whole query — the other repos still have
// answers in them.
func TestListCommitsSkipsBrokenPaths(t *testing.T) {
	good := initToolRepo(t)
	d := Deps{Logger: zap.NewNop(), Repos: fakeRepos{repos: []vcsstore.RepoWithLocations{
		{
			VCSRepo: vcsstore.VCSRepo{ID: "r1", Platform: "github", Owner: "acme", Name: "broken"},
			Paths:   []vcsstore.VCSRepoPath{{ID: "p0", RepoID: "r1", Path: filepath.Join(t.TempDir(), "gone")}},
		},
		{
			VCSRepo: vcsstore.VCSRepo{ID: "r2", Platform: "github", Owner: "acme", Name: "app"},
			Paths:   []vcsstore.VCSRepoPath{{ID: "p1", RepoID: "r2", Path: good}},
		},
	}}}

	res := callListCommits(t, d, map[string]any{"since": "2026-09-01"})
	assert.Equal(t, 2, res["count"])
}

// Mirrors the existing `if d.VCS != nil` guard: no repo source, no tool, no panic.
func TestListCommitsNotRegisteredWithoutARepoSource(t *testing.T) {
	reg := NewRegistry(Deps{Logger: zap.NewNop()})

	_, ok := reg.Get("list_commits")
	assert.False(t, ok)

	reg = NewRegistry(Deps{Repos: reposAt(initToolRepo(t)), Logger: zap.NewNop()})
	_, ok = reg.Get("list_commits")
	assert.True(t, ok)
}

// The description is what steers the model away from search_contexts, so it has
// to name the situation it is for.
func TestListCommitsDescriptionMentionsDateQuestions(t *testing.T) {
	reg := NewRegistry(Deps{Repos: reposAt(initToolRepo(t)), Logger: zap.NewNop()})

	tool, ok := reg.Get("list_commits")
	require.True(t, ok)
	assert.Contains(t, tool.Description, "YYYY-MM-DD")
	assert.False(t, tool.Write, "reading history must not require confirmation")
}
