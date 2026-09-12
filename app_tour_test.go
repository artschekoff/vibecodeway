package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/artschekoff/vibecodeway/internal/pty"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTourTestApp builds an App with an in-memory-backed sqlite store and a
// live (but session-less) pty manager — enough for StartCodeTour's contract
// and generation-guard paths, which never reach a.openAI.ChatCompletion in
// these tests (the "no changes" early return in internal/tour.Generate fires
// before any LLM call).
func newTourTestApp(t *testing.T) *App {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return &App{db: s, ctx: context.Background(), ptyMgr: pty.NewManager()}
}

// gitRun runs `git -C dir <args...>`, failing the test on error. Isolated
// git config env vars mirror internal/tour's test fixtures so a developer's
// global gitconfig (gpg signing, hooksPath, …) can't make this flaky.
func gitRun(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// initTourGitRepo creates a git repo with .vibecodeway/ gitignored and
// committed, then commits one file at commitDate. Gitignoring .vibecodeway/
// (spec §2's own recommendation) keeps the tour contract file itself from
// showing up as an untracked change when a test wants a genuinely
// changeless working tree.
func initTourGitRepo(t *testing.T, commitDate string) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, nil, "init", "--initial-branch=main")
	gitRun(t, dir, nil, "config", "user.email", "dev@example.com")
	gitRun(t, dir, nil, "config", "user.name", "Dev Example")

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".vibecodeway/\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	gitRun(t, dir, nil, "add", ".")
	gitRun(t, dir, []string{"GIT_AUTHOR_DATE=" + commitDate, "GIT_COMMITTER_DATE=" + commitDate}, "commit", "-m", "initial")
	return dir
}

// writeTourContract writes contents to <repoPath>/.vibecodeway/tour.json and
// sets its mtime, returning the file's absolute path.
func writeTourContract(t *testing.T, repoPath, contents string, modTime time.Time) string {
	t.Helper()
	dir := filepath.Join(repoPath, ".vibecodeway")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "tour.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
	return path
}

const validTourContractJSON = `{
	"version": 1,
	"title": "Add analytics dashboard",
	"steps": [
		{"file": "src/x.ts", "anchor": "func computeDashboard(", "what": "Added entry point", "why": "keeps it testable"}
	]
}`

func TestStartCodeTour_FreshContractFile_ImportsAndConsumes(t *testing.T) {
	a := newTourTestApp(t)
	commitTime := "2026-09-01T12:00:00+00:00"
	repoPath := initTourGitRepo(t, commitTime)

	fileModTime := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) // after the commit
	contractPath := writeTourContract(t, repoPath, validTourContractJSON, fileModTime)

	got, err := a.StartCodeTour("ctx-1", repoPath)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "agent", got.Source)
	assert.Equal(t, "Add analytics dashboard", got.Title)
	assert.NotEmpty(t, got.ID)
	if assert.Len(t, got.Steps, 1) {
		assert.Equal(t, appstore.TourStep{
			File:   "src/x.ts",
			Anchor: "func computeDashboard(",
			What:   "Added entry point",
			Why:    "keeps it testable",
		}, got.Steps[0])
	}

	// Consumed: the contract file is gone after a successful import.
	_, statErr := os.Stat(contractPath)
	assert.True(t, os.IsNotExist(statErr), "expected tour.json to be deleted after import, stat err: %v", statErr)

	// Stored: GetCodeTour returns the same row.
	stored, err := a.GetCodeTour("ctx-1", repoPath)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, got.ID, stored.ID)
	assert.Equal(t, "agent", stored.Source)
}

func TestStartCodeTour_StaleContractFile_FallsBackToGeneration_NoChangesError(t *testing.T) {
	a := newTourTestApp(t)
	commitTime := "2026-09-05T12:00:00+00:00"
	repoPath := initTourGitRepo(t, commitTime)

	// mtime BEFORE the last commit ⇒ stale, per spec §2's freshness rule.
	staleModTime := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	contractPath := writeTourContract(t, repoPath, validTourContractJSON, staleModTime)

	_, err := a.StartCodeTour("ctx-2", repoPath)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCodeTourNoChanges), "expected ErrCodeTourNoChanges, got: %v", err)

	// Not consumed: the stale file was never imported.
	_, statErr := os.Stat(contractPath)
	assert.NoError(t, statErr, "stale contract file should be left alone, not consumed")

	// Nothing stored either.
	stored, err := a.GetCodeTour("ctx-2", repoPath)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestStartCodeTour_InvalidContractFile_FallsBackToGeneration_NoChangesError(t *testing.T) {
	a := newTourTestApp(t)
	commitTime := "2026-09-05T12:00:00+00:00"
	repoPath := initTourGitRepo(t, commitTime)

	// Fresh mtime, but invalid schema (missing steps) — spec §2: invalid
	// file falls back to generation regardless of freshness.
	freshModTime := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	writeTourContract(t, repoPath, `{"version": 1, "title": "x", "steps": []}`, freshModTime)

	_, err := a.StartCodeTour("ctx-3", repoPath)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCodeTourNoChanges), "expected ErrCodeTourNoChanges, got: %v", err)
}

func TestStartCodeTour_NoContractFile_ChangelessRepo_NoChangesError(t *testing.T) {
	a := newTourTestApp(t)
	repoPath := initTourGitRepo(t, "2026-09-05T12:00:00+00:00")

	_, err := a.StartCodeTour("ctx-4", repoPath)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCodeTourNoChanges))
}

func TestGetCodeTour_Absent_ReturnsNilNoError(t *testing.T) {
	a := newTourTestApp(t)
	got, err := a.GetCodeTour("ctx-none", "/nowhere")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSetCodeTourStepAndDecision_RoundTrip(t *testing.T) {
	a := newTourTestApp(t)
	repoPath := t.TempDir()

	tour := &appstore.CodeTour{
		ContextID: "ctx-5",
		RepoPath:  repoPath,
		Source:    "agent",
		Title:     "t",
		Steps: []appstore.TourStep{
			{File: "a.go", Anchor: "func a(", What: "did a", Why: "because a"},
			{File: "b.go", Anchor: "func b(", What: "did b", Why: "because b"},
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, a.db.UpsertCodeTour(a.ctx, tour))

	require.NoError(t, a.SetCodeTourStep(tour.ID, 1))
	require.NoError(t, a.SetCodeTourDecision(tour.ID, 0, "staged"))

	got, err := a.GetCodeTour("ctx-5", repoPath)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.StepIndex)
	assert.Equal(t, "staged", got.Decisions[0])
}

func TestSetCodeTourDecision_UnknownID_ReturnsError(t *testing.T) {
	a := newTourTestApp(t)
	err := a.SetCodeTourDecision("does-not-exist", 0, "staged")
	require.Error(t, err)
}

// filterDiffForFile is exercised indirectly via AskTourQuestion in a full
// integration test would require a real AI client; here it's covered
// directly since it's a pure function, to lock down the naive
// file-path-filter behavior AskTourQuestion depends on.
func TestFilterDiffForFile(t *testing.T) {
	diff := "=== unstaged changes (git diff) ===\n" +
		"diff --git a/foo.go b/foo.go\n" +
		"index 111..222 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1 +1 @@\n" +
		"-old foo\n" +
		"+new foo\n" +
		"diff --git a/bar.go b/bar.go\n" +
		"index 333..444 100644\n" +
		"--- a/bar.go\n" +
		"+++ b/bar.go\n" +
		"@@ -1 +1 @@\n" +
		"-old bar\n" +
		"+new bar\n" +
		"=== untracked file: baz.go ===\n" +
		"package main\n"

	got := filterDiffForFile(diff, "bar.go")
	assert.Contains(t, got, "diff --git a/bar.go b/bar.go")
	assert.Contains(t, got, "-old bar")
	assert.Contains(t, got, "+new bar")
	assert.NotContains(t, got, "foo.go")
	assert.NotContains(t, got, "baz.go")

	got = filterDiffForFile(diff, "baz.go")
	assert.Contains(t, got, "package main")
	assert.NotContains(t, got, "foo.go")
	assert.NotContains(t, got, "bar.go")

	got = filterDiffForFile(diff, "nope.go")
	assert.Empty(t, got)
}

func TestAskTourQuestion_TourIDMismatch_ReturnsError(t *testing.T) {
	a := newTourTestApp(t)
	repoPath := t.TempDir()

	tour := &appstore.CodeTour{
		ContextID: "ctx-6",
		RepoPath:  repoPath,
		Source:    "agent",
		Title:     "t",
		Steps:     []appstore.TourStep{{File: "a.go", Anchor: "func a(", What: "did a", Why: "because a"}},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, a.db.UpsertCodeTour(a.ctx, tour))

	_, err := a.AskTourQuestion("wrong-id", "ctx-6", repoPath, 0, "why?")
	require.Error(t, err)
}

func TestAskTourQuestion_NoActiveTour_ReturnsError(t *testing.T) {
	a := newTourTestApp(t)
	_, err := a.AskTourQuestion("some-id", "ctx-none", t.TempDir(), 0, "why?")
	require.Error(t, err)
}

func TestAskTourQuestion_StepIndexOutOfRange_ReturnsError(t *testing.T) {
	a := newTourTestApp(t)
	repoPath := t.TempDir()

	tour := &appstore.CodeTour{
		ContextID: "ctx-7",
		RepoPath:  repoPath,
		Source:    "agent",
		Title:     "t",
		Steps:     []appstore.TourStep{{File: "a.go", Anchor: "func a(", What: "did a", Why: "because a"}},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, a.db.UpsertCodeTour(a.ctx, tour))

	_, err := a.AskTourQuestion(tour.ID, "ctx-7", repoPath, 5, "why?")
	require.Error(t, err)
}

// Sanity check that the contract JSON literal used above round-trips
// through the same schema the app-level tests assume.
func TestValidTourContractJSONFixtureIsWellFormed(t *testing.T) {
	var payload struct {
		Version int `json:"version"`
		Steps   []struct {
			File string `json:"file"`
		} `json:"steps"`
	}
	require.NoError(t, json.Unmarshal([]byte(validTourContractJSON), &payload))
	assert.Equal(t, 1, payload.Version)
	assert.Len(t, payload.Steps, 1)
}
