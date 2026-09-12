package tour

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeContract writes raw JSON to <repoPath>/.vibecodeway/tour.json,
// creating the directory as needed.
func writeContract(t *testing.T, repoPath, contents string) {
	t.Helper()
	dir := filepath.Join(repoPath, ".vibecodeway")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tour.json"), []byte(contents), 0o644))
}

func TestLoad(t *testing.T) {
	validJSON := `{
		"version": 1,
		"title": "Add analytics dashboard",
		"steps": [
			{"file": "src/x.ts", "anchor": "func computeDashboard(", "what": "Added entry point", "why": "keeps it testable"}
		]
	}`

	tests := []struct {
		name        string
		contents    string // empty means "do not write the file"
		wantErr     bool
		wantInvalid bool
		wantNil     bool
		check       func(t *testing.T, f *File)
	}{
		{
			name:     "valid file round-trip",
			contents: validJSON,
			check: func(t *testing.T, f *File) {
				require.NotNil(t, f)
				assert.Equal(t, 1, f.Version)
				assert.Equal(t, "Add analytics dashboard", f.Title)
				require.Len(t, f.Steps, 1)
				assert.Equal(t, appstore.TourStep{
					File:   "src/x.ts",
					Anchor: "func computeDashboard(",
					What:   "Added entry point",
					Why:    "keeps it testable",
				}, f.Steps[0])
			},
		},
		{
			name:    "missing file",
			wantNil: true,
		},
		{
			name:        "invalid version",
			contents:    `{"version": 2, "title": "x", "steps": [{"file":"a","anchor":"b","what":"c"}]}`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "empty steps",
			contents:    `{"version": 1, "title": "x", "steps": []}`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "missing anchor",
			contents:    `{"version": 1, "title": "x", "steps": [{"file":"a","anchor":"","what":"c"}]}`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "missing file field",
			contents:    `{"version": 1, "title": "x", "steps": [{"file":"","anchor":"b","what":"c"}]}`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "missing what",
			contents:    `{"version": 1, "title": "x", "steps": [{"file":"a","anchor":"b","what":""}]}`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:     "empty why is allowed",
			contents: `{"version": 1, "title": "x", "steps": [{"file":"a","anchor":"b","what":"c","why":""}]}`,
			check: func(t *testing.T, f *File) {
				require.NotNil(t, f)
				assert.Equal(t, "", f.Steps[0].Why)
			},
		},
		{
			name:        "malformed json",
			contents:    `{not json`,
			wantErr:     true,
			wantInvalid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			if tt.contents != "" {
				writeContract(t, repoPath, tt.contents)
			}

			f, err := Load(repoPath)

			if tt.wantNil {
				assert.NoError(t, err)
				assert.Nil(t, f)
				return
			}
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, f)
				if tt.wantInvalid {
					assert.True(t, errors.Is(err, ErrInvalid), "expected ErrInvalid, got %v", err)
				}
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, f)
			}
		})
	}
}

// initGitRepo creates a throwaway repository, isolated from the developer's
// ambient git config the same way internal/vcs's tests are, so a global
// commit.gpgsign=true or core.hooksPath cannot make this fixture fail for
// reasons unrelated to the code under test.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		cmd.Env = append(cmd.Env, env...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}

	run(nil, "init", "--initial-branch=main")
	run(nil, "config", "user.email", "dev@example.com")
	run(nil, "config", "user.name", "Dev Example")
	return dir
}

func commitAt(t *testing.T, dir, date string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte(date), 0o644))
	cmd := exec.Command("git", "-C", dir, "add", ".")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "-C", dir, "commit", "-m", "commit at "+date)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git commit: %s", out)
}

func TestFresh(t *testing.T) {
	t.Run("file newer than last commit is fresh", func(t *testing.T) {
		repoPath := initGitRepo(t)
		commitAt(t, repoPath, "2026-09-01T12:00:00+00:00")

		fileModTime := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
		fresh, err := Fresh(repoPath, fileModTime)
		require.NoError(t, err)
		assert.True(t, fresh)
	})

	t.Run("file older than last commit is stale", func(t *testing.T) {
		repoPath := initGitRepo(t)
		commitAt(t, repoPath, "2026-09-05T12:00:00+00:00")

		fileModTime := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
		fresh, err := Fresh(repoPath, fileModTime)
		require.NoError(t, err)
		assert.False(t, fresh)
	})

	t.Run("repo with zero commits counts as fresh", func(t *testing.T) {
		repoPath := initGitRepo(t) // no commits made

		fresh, err := Fresh(repoPath, time.Now())
		require.NoError(t, err)
		assert.True(t, fresh)
	})

	t.Run("not a git repository is a hard error, not fresh", func(t *testing.T) {
		repoPath := t.TempDir() // no git init at all

		fresh, err := Fresh(repoPath, time.Now())
		require.Error(t, err)
		assert.False(t, fresh)
	})
}

func TestConsume(t *testing.T) {
	t.Run("removes existing file", func(t *testing.T) {
		repoPath := t.TempDir()
		writeContract(t, repoPath, `{"version":1,"steps":[{"file":"a","anchor":"b","what":"c"}]}`)

		err := Consume(repoPath)
		require.NoError(t, err)

		_, statErr := os.Stat(contractPath(repoPath))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("missing file is not an error and is idempotent", func(t *testing.T) {
		repoPath := t.TempDir()

		require.NoError(t, Consume(repoPath))
		require.NoError(t, Consume(repoPath)) // second call: still no error
	})
}
