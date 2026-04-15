// Package acli provides a safe wrapper around the Atlassian CLI (acli) binary.
package acli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	defaultBinPath = "acli"
	defaultTimeout = 30 * time.Second
)

// allowedSubcmds is the set of acli subcommand paths this runner may execute.
// User-supplied data is only ever passed as flag values, never as subcommands.
var allowedSubcmds = map[string]bool{
	"jira workitem search":       true,
	"jira workitem view":         true,
	"jira workitem comment list": true,
	"jira auth status":           true,
}

// Runner executes acli subcommands via exec.CommandContext.
type Runner struct {
	binPath string
	timeout time.Duration
	logger  *zap.Logger
}

// Option configures a Runner.
type Option func(*Runner)

// WithBinPath overrides the acli binary path (default: "acli" resolved via PATH).
func WithBinPath(path string) Option {
	return func(r *Runner) { r.binPath = path }
}

// WithTimeout overrides the per-command execution timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(r *Runner) { r.timeout = d }
}

// NewRunner creates a Runner with the given logger and options.
func NewRunner(logger *zap.Logger, opts ...Option) *Runner {
	r := &Runner{
		binPath: defaultBinPath,
		timeout: defaultTimeout,
		logger:  logger,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run executes an acli subcommand with optional positional arguments and flag arguments.
//
// subCmdPath is a slice of path tokens used for the allowlist check only,
// e.g. ["jira", "workitem", "view"].
//
// positionalArgs are user-supplied values placed between the subcommand and the
// flags, e.g. ["JH-926"] for issue key. They are NOT part of the allowlist key.
//
// flags are named flag/value arguments, e.g. ["--json"].
//
// Returns trimmed stdout on success. On non-zero exit the stderr is wrapped into the error.
func (r *Runner) Run(ctx context.Context, subCmdPath []string, positionalArgs []string, flags []string) (string, error) {
	subcmd := strings.Join(subCmdPath, " ")
	if !allowedSubcmds[subcmd] {
		return "", fmt.Errorf("acli subcommand %q is not in the allowed list", subcmd)
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	args := make([]string, 0, len(subCmdPath)+len(positionalArgs)+len(flags))
	args = append(args, subCmdPath...)
	args = append(args, positionalArgs...)
	args = append(args, flags...)
	cmd := exec.CommandContext(ctx, r.binPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	r.logger.Debug("acli exec", zap.String("cmd", subcmd), zap.Strings("positional", positionalArgs), zap.Strings("flags", flags))

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		r.logger.Error("acli exec failed",
			zap.String("cmd", subcmd),
			zap.String("stderr", errMsg),
			zap.Error(err),
		)
		return "", fmt.Errorf("acli %s: %s", subcmd, errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
