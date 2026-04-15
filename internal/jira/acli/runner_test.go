package acli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// buildFakeAcli compiles a tiny Go program into dir/acli that prints its args and exits 0.
func buildFakeAcli(t *testing.T, dir string) string {
	t.Helper()
	const src = `package main
import (
	"fmt"
	"os"
	"strings"
)
func main() {
	fmt.Println(strings.Join(os.Args[1:], " "))
}
`
	srcFile := filepath.Join(dir, "fake_acli_src.go")
	bin := filepath.Join(dir, "acli")
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o600))
	cmd := exec.Command("go", "build", "-o", bin, srcFile)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build fake acli: %s", out)
	return bin
}

// TestResolveBinPath verifies that resolveBinPath finds a binary in common macOS
// locations even when that directory is not present in the current PATH.
// This is the scenario that causes the auth banner to always appear in the compiled
// macOS app, where GUI apps inherit a minimal PATH that excludes Homebrew directories.
func TestResolveBinPath(t *testing.T) {
	t.Run("returns binary from current PATH when found", func(t *testing.T) {
		dir := t.TempDir()
		buildFakeAcli(t, dir)

		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		got := resolveBinPath("acli", nil)
		assert.Equal(t, filepath.Join(dir, "acli"), got)
	})

	t.Run("falls back to extra dirs when not in PATH", func(t *testing.T) {
		dir := t.TempDir()
		buildFakeAcli(t, dir)

		// Ensure dir is NOT in PATH (simulate minimal GUI-app PATH).
		original := os.Getenv("PATH")
		t.Setenv("PATH", "/usr/bin:/bin")
		defer t.Setenv("PATH", original)

		got := resolveBinPath("acli", []string{dir})
		assert.Equal(t, filepath.Join(dir, "acli"), got)
	})

	t.Run("returns name unchanged when not found anywhere", func(t *testing.T) {
		got := resolveBinPath("nonexistent-binary-zzz", []string{"/nonexistent-dir"})
		assert.Equal(t, "nonexistent-binary-zzz", got)
	})
}

// TestNewRunnerUsesExpandedPath verifies that a Runner constructed via NewRunner
// can execute a binary placed only in a known macOS Homebrew location, even when
// that location is absent from PATH (simulating a compiled macOS GUI app launch).
func TestNewRunnerUsesExpandedPath(t *testing.T) {
	dir := t.TempDir()

	const src = `package main
import (
	"fmt"
	"os"
	"strings"
)
func main() {
	args := os.Args[1:]
	// Mimic "acli jira auth status" stdout.
	if strings.Join(args, " ") == "jira auth status" {
		fmt.Println("Logged in as test@example.com")
		return
	}
	fmt.Println(strings.Join(args, " "))
}
`
	srcFile := filepath.Join(dir, "fake_runner_src.go")
	bin := filepath.Join(dir, "acli")
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o600))
	// Build the fake binary before restricting PATH so that "go" is still findable.
	cmd := exec.Command("go", "build", "-o", bin, srcFile)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build: %s", out)

	// Simulate the macOS GUI-app scenario: PATH has only system dirs, no Homebrew.
	// This must happen AFTER building the fake binary.
	t.Setenv("PATH", "/usr/bin:/bin")

	// Override knownExtraDirs so the runner discovers our temp binary without
	// touching real system directories like /opt/homebrew/bin.
	origDirs := knownExtraDirs
	knownExtraDirs = []string{dir}
	t.Cleanup(func() { knownExtraDirs = origDirs })

	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithTimeout(5*time.Second))

	// The runner must have resolved the absolute path at construction time.
	assert.Equal(t, bin, r.binPath, "runner should store absolute path to acli")

	ctx := t.Context()
	status, err := r.Run(ctx, []string{"jira", "auth", "status"}, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, status, "Logged in")
}
