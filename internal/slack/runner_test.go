package slack

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// buildFakeSlackCLI compiles a tiny Go program into dir/slack-cli that prints its args and exits 0.
func buildFakeSlackCLI(t *testing.T, dir string) string {
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
	srcFile := filepath.Join(dir, "fake_slack_cli_src.go")
	bin := filepath.Join(dir, "slack-cli")
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o600))
	cmd := exec.Command("go", "build", "-o", bin, srcFile)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build fake slack-cli: %s", out)
	return bin
}

func TestResolveBinPath(t *testing.T) {
	t.Run("returns binary from current PATH when found", func(t *testing.T) {
		dir := t.TempDir()
		buildFakeSlackCLI(t, dir)

		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		got := resolveBinPath("slack-cli", nil)
		assert.Equal(t, filepath.Join(dir, "slack-cli"), got)
	})

	t.Run("falls back to extra dirs when not in PATH", func(t *testing.T) {
		dir := t.TempDir()
		buildFakeSlackCLI(t, dir)

		t.Setenv("PATH", "/usr/bin:/bin")

		got := resolveBinPath("slack-cli", []string{dir})
		assert.Equal(t, filepath.Join(dir, "slack-cli"), got)
	})

	t.Run("returns name unchanged when not found anywhere", func(t *testing.T) {
		got := resolveBinPath("nonexistent-binary-zzz", []string{"/nonexistent-dir"})
		assert.Equal(t, "nonexistent-binary-zzz", got)
	})
}

func TestNewRunnerUsesExpandedPath(t *testing.T) {
	dir := t.TempDir()
	bin := buildFakeSlackCLI(t, dir)

	// Simulate macOS GUI-app scenario: PATH has only system dirs, no Homebrew.
	t.Setenv("PATH", "/usr/bin:/bin")

	origDirs := knownExtraDirs
	knownExtraDirs = []string{dir}
	t.Cleanup(func() { knownExtraDirs = origDirs })

	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithTimeout(5*time.Second))

	assert.Equal(t, bin, r.binPath, "runner should store absolute path to slack-cli")

	ctx := t.Context()
	out, err := r.Run(ctx, []string{"list-workspaces"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "list-workspaces", out)
}

// buildFakeBinary compiles the testhelper program into a temp dir and returns its path.
// The helper prints its args to stdout and exits 0, simulating a successful slack-cli run.
func buildFakeBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "fake.go")
	bin := filepath.Join(dir, "fake-slack-cli")

	const src_ = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println(strings.Join(os.Args[1:], " "))
}
`
	if err := os.WriteFile(src, []byte(src_), 0o600); err != nil {
		t.Fatalf("write fake source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake binary: %v\n%s", err, out)
	}
	return bin
}

func TestRunnerAllowedSubcommands(t *testing.T) {
	bin := buildFakeBinary(t)
	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithBinPath(bin), WithTimeout(5*time.Second))

	tests := []struct {
		name       string
		subCmdPath []string
		flags      []string
		wantErr    bool
		wantOut    string
	}{
		{
			name:       "search is allowed",
			subCmdPath: []string{"search"},
			flags:      []string{"myworkspace", "deployment failed"},
			wantErr:    false,
			wantOut:    "search myworkspace deployment failed",
		},
		{
			name:       "search-channels is allowed",
			subCmdPath: []string{"search-channels"},
			flags:      []string{"myworkspace", "*deploy*"},
			wantErr:    false,
			wantOut:    "search-channels myworkspace *deploy*",
		},
		{
			name:       "load-context is allowed",
			subCmdPath: []string{"load-context"},
			flags:      []string{"myworkspace", "C01234567", "1700000000.123456"},
			wantErr:    false,
			wantOut:    "load-context myworkspace C01234567 1700000000.123456",
		},
		{
			name:       "list-workspaces is allowed",
			subCmdPath: []string{"list-workspaces"},
			flags:      nil,
			wantErr:    false,
			wantOut:    "list-workspaces",
		},
		{
			name:       "unknown subcommand is rejected",
			subCmdPath: []string{"auth"},
			flags:      nil,
			wantErr:    true,
		},
		{
			name:       "remove-creds is rejected",
			subCmdPath: []string{"remove-creds"},
			flags:      nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.Run(context.Background(), tt.subCmdPath, tt.flags)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOut, got)
		})
	}
}

func TestRunnerTimeout(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "slow.go")
	bin := filepath.Join(dir, "slow-slack-cli")

	const src_ = `package main

import "time"

func main() {
	time.Sleep(10 * time.Second)
}
`
	if err := os.WriteFile(src, []byte(src_), 0o600); err != nil {
		t.Fatalf("write slow source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build slow binary: %v\n%s", err, out)
	}

	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithBinPath(bin), WithTimeout(100*time.Millisecond))

	_, err := r.Run(context.Background(), []string{"search"}, []string{"ws", "q"})
	require.Error(t, err)
}

func TestRunnerNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "fail.go")
	bin := filepath.Join(dir, "fail-slack-cli")

	const src_ = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "credentials not found")
	os.Exit(1)
}
`
	if err := os.WriteFile(src, []byte(src_), 0o600); err != nil {
		t.Fatalf("write fail source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fail binary: %v\n%s", err, out)
	}

	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithBinPath(bin), WithTimeout(5*time.Second))

	_, err := r.Run(context.Background(), []string{"search"}, []string{"ws", "q"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not found")
}

func TestRunnerContextCancellation(t *testing.T) {
	bin := buildFakeBinary(t)
	logger, _ := zap.NewDevelopment()
	r := NewRunner(logger, WithBinPath(bin), WithTimeout(30*time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, []string{"search"}, []string{"ws", "q"})
	require.Error(t, err)
}

// buildEnvEchoBinary compiles a helper that prints the value of SLACK_MCP_PASSPHRASE to stdout.
func buildEnvEchoBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "envecho.go")
	bin := filepath.Join(dir, "envecho-slack-cli")

	const src_ = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Getenv("SLACK_MCP_PASSPHRASE"))
}
`
	if err := os.WriteFile(src, []byte(src_), 0o600); err != nil {
		t.Fatalf("write envecho source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build envecho binary: %v\n%s", err, out)
	}
	return bin
}

func TestRunnerPassphraseEnvVar(t *testing.T) {
	bin := buildEnvEchoBinary(t)
	logger, _ := zap.NewDevelopment()

	t.Run("WithPassphrase option sets env var", func(t *testing.T) {
		r := NewRunner(logger, WithBinPath(bin), WithTimeout(5*time.Second), WithPassphrase("s3cr3t"))
		out, err := r.Run(context.Background(), []string{"search"}, []string{"ws", "q"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", out)
	})

	t.Run("SetPassphrase updates env var dynamically", func(t *testing.T) {
		r := NewRunner(logger, WithBinPath(bin), WithTimeout(5*time.Second))
		require.NoError(t, r.SetPassphrase("updated-secret"))
		out, err := r.Run(context.Background(), []string{"search"}, []string{"ws", "q"})
		require.NoError(t, err)
		assert.Equal(t, "updated-secret", out)
	})

	t.Run("no passphrase yields empty env var", func(t *testing.T) {
		r := NewRunner(logger, WithBinPath(bin), WithTimeout(5*time.Second))
		out, err := r.Run(context.Background(), []string{"search"}, []string{"ws", "q"})
		require.NoError(t, err)
		assert.Equal(t, "", out)
	})
}
