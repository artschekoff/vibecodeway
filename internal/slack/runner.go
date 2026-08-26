// Package slack provides a safe wrapper around the slack-cli binary.
package slack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultBinName = "slack-cli"
	defaultTimeout = 60 * time.Second
)

// knownExtraDirs lists common macOS installation prefixes that are not present in
// the minimal PATH that macOS GUI apps receive when launched outside a terminal
// (e.g. from Finder or via open(1)). Exposed as a var so tests can override it.
var knownExtraDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
}

// resolveBinPath returns the absolute path for name by first consulting the
// current PATH (via exec.LookPath), then checking each dir in extraDirs.
// Falls back to returning name unchanged so exec.Command will surface a
// clear "executable not found" error rather than a silent mismatch.
func resolveBinPath(name string, extraDirs []string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range extraDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

// allowedSubcmds is the set of slack-cli subcommands this runner may execute.
// User-supplied data is only ever passed as flag values, never as subcommands.
var allowedSubcmds = map[string]bool{
	"search":          true,
	"search-channels": true,
	"load-context":    true,
	"list-workspaces": true,
	"list-dms":        true,
}

// Runner executes slack-cli subcommands via exec.CommandContext.
type Runner struct {
	binPath    string
	timeout    time.Duration
	logger     *zap.Logger
	mu         sync.RWMutex
	passphrase string
}

// Option configures a Runner.
type Option func(*Runner)

// WithBinPath overrides the slack-cli binary path (default: resolved via PATH + knownExtraDirs).
func WithBinPath(path string) Option {
	return func(r *Runner) { r.binPath = path }
}

// WithTimeout overrides the per-command execution timeout (default 60s).
func WithTimeout(d time.Duration) Option {
	return func(r *Runner) { r.timeout = d }
}

// WithPassphrase sets the initial SLACK_MCP_PASSPHRASE value injected into every slack-cli call.
// Panics if passphrase contains newline characters.
func WithPassphrase(passphrase string) Option {
	if strings.ContainsAny(passphrase, "\n\r") {
		panic("slack: passphrase must not contain newline characters")
	}
	return func(r *Runner) { r.passphrase = passphrase }
}

// SetPassphrase updates the SLACK_MCP_PASSPHRASE value used for subsequent slack-cli calls.
// Returns an error if passphrase contains a newline (would inject extra env vars).
// Safe for concurrent use.
func (r *Runner) SetPassphrase(passphrase string) error {
	if strings.ContainsAny(passphrase, "\n\r") {
		return fmt.Errorf("passphrase must not contain newline characters")
	}
	r.mu.Lock()
	r.passphrase = passphrase
	r.mu.Unlock()
	return nil
}

// Passphrase returns the current passphrase. Empty string when unset.
// Safe for concurrent use.
func (r *Runner) Passphrase() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.passphrase
}

// NewRunner creates a Runner with the given logger and options.
// It resolves the slack-cli binary at construction time by checking both the
// current PATH and knownExtraDirs, so that the runner works correctly in macOS
// GUI apps which inherit a minimal PATH that excludes Homebrew directories.
func NewRunner(logger *zap.Logger, opts ...Option) *Runner {
	r := &Runner{
		binPath: resolveBinPath(defaultBinName, knownExtraDirs),
		timeout: defaultTimeout,
		logger:  logger,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run executes a slack-cli subcommand with the provided flag arguments.
//
// subCmdPath is a slice of path tokens, e.g. ["search"] or ["load-context"].
// flags are additional positional/flag arguments, e.g. ["myworkspace", "query", "--count", "20"].
//
// Returns trimmed stdout on success. On non-zero exit the stderr is wrapped into the error.
func (r *Runner) Run(ctx context.Context, subCmdPath []string, flags []string) (string, error) {
	key := strings.Join(subCmdPath, " ")
	if !allowedSubcmds[key] {
		return "", fmt.Errorf("slack-cli subcommand %q is not in the allowed list", key)
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	args := append(subCmdPath, flags...) //nolint:gocritic
	cmd := exec.CommandContext(ctx, r.binPath, args...)

	r.mu.RLock()
	passphrase := r.passphrase
	r.mu.RUnlock()

	env := os.Environ()
	env = append(env, "SLACK_MCP_PASSPHRASE="+passphrase)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	r.logger.Debug("slack-cli exec", zap.String("cmd", key), zap.Strings("flags", flags))

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		r.logger.Error(
			"slack-cli exec failed",
			zap.String("cmd", key),
			zap.String("stderr", errMsg),
			zap.Error(err),
		)
		return "", fmt.Errorf("slack-cli %s: %s", key, errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
