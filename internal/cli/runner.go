// Package cli provides a safe exec wrapper around the context-consolidator-cli binary.
// It mirrors the pattern used by internal/jira/acli and internal/slack.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/jira"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"go.uber.org/zap"
)

const defaultTimeout = 60 * time.Second

// passphraseEnvVar is the env var injected into CLI subprocesses so they can
// authenticate Slack and OpenAI operations.
const passphraseEnvVar = "CONTEXT_CONSOLIDATOR_PASSPHRASE"

// knownExtraDirs mirrors the list from internal/jira/acli: paths that macOS
// GUI and stdio processes often miss because they inherit a minimal PATH.
var knownExtraDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
}

// resolveBinPath checks PATH first, then extra dirs, and falls back to name
// unchanged so exec.Command surfaces a clear "executable not found" error.
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

// Runner executes context-consolidator-cli subcommands via exec.CommandContext.
type Runner struct {
	binPath        string
	timeout        time.Duration
	logger         *zap.Logger
	passphraseFunc func() string // optional: provides passphrase to inject into subprocesses
}

// Option configures a Runner.
type Option func(*Runner)

// WithBinPath overrides the CLI binary path.
func WithBinPath(path string) Option {
	return func(r *Runner) { r.binPath = path }
}

// WithTimeout overrides the per-command execution timeout (default 60s).
func WithTimeout(d time.Duration) Option {
	return func(r *Runner) { r.timeout = d }
}

// WithPassphraseFunc provides a function that returns the current passphrase,
// injected as CONTEXT_CONSOLIDATOR_PASSPHRASE into each CLI subprocess.
func WithPassphraseFunc(fn func() string) Option {
	return func(r *Runner) { r.passphraseFunc = fn }
}

// NewRunner creates a Runner resolving the binary from PATH and knownExtraDirs.
func NewRunner(logger *zap.Logger, opts ...Option) *Runner {
	r := &Runner{
		binPath: resolveBinPath("context-consolidator-cli", knownExtraDirs),
		timeout: defaultTimeout,
		logger:  logger,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// run executes the CLI with the given arguments, returns stdout bytes on success.
// Stderr is captured and wrapped into the returned error on non-zero exit.
func (r *Runner) run(ctx context.Context, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.binPath, args...)

	if r.passphraseFunc != nil {
		if pp := r.passphraseFunc(); pp != "" {
			cmd.Env = append(os.Environ(), passphraseEnvVar+"="+pp)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
	}
	r.logger.Debug("cli exec", zap.Strings("args", args))

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		r.logger.Error("cli exec failed",
			zap.String("subcmd", subcmd),
			zap.String("stderr", errMsg),
			zap.Error(err),
		)
		return nil, fmt.Errorf("context-consolidator-cli %s: %s", subcmd, errMsg)
	}

	return stdout.Bytes(), nil
}

// runNoOutput executes the CLI and discards stdout, returning only errors.
func (r *Runner) runNoOutput(ctx context.Context, args []string) error {
	_, err := r.run(ctx, args)
	return err
}

// unmarshal parses JSON bytes into v.
func unmarshal[T any](data []byte, subcmd string) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", subcmd, err)
	}
	return &result, nil
}

// unmarshalSlice parses JSON bytes into a slice of T.
func unmarshalSlice[T any](data []byte, subcmd string) ([]T, error) {
	var result []T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s response: %w", subcmd, err)
	}
	return result, nil
}

// buildArgs constructs a flag argument list skipping pairs where value is empty.
func buildArgs(subCmd string, flagsAndValues ...string) []string {
	args := []string{subCmd}
	for i := 0; i+1 < len(flagsAndValues); i += 2 {
		if flagsAndValues[i+1] != "" {
			args = append(args, flagsAndValues[i], flagsAndValues[i+1])
		}
	}
	return args
}

// --- add-text / add-file (Phase A) ---

// AddTextOpts holds parameters for the add-text subcommand.
type AddTextOpts struct {
	ContextID  string
	ContextKey string
	Name       string
	FilePath   string
	Content    string
}

// AddText calls `add-text` and returns the created ContextText record.
func (r *Runner) AddText(ctx context.Context, opts AddTextOpts) (*appstore.ContextText, error) {
	args := buildArgs("add-text",
		"--context-id", opts.ContextID,
		"--context-key", opts.ContextKey,
		"--name", opts.Name,
		"--file-path", opts.FilePath,
		"--content", opts.Content,
	)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.ContextText](out, "add-text")
}

// AddFileOpts holds parameters for the add-file subcommand.
type AddFileOpts struct {
	ContextID  string
	ContextKey string
	FilePath   string
}

// AddFile calls `add-file` and returns the created ContextUploadedFile record.
func (r *Runner) AddFile(ctx context.Context, opts AddFileOpts) (*appstore.ContextUploadedFile, error) {
	args := buildArgs("add-file",
		"--context-id", opts.ContextID,
		"--context-key", opts.ContextKey,
		"--file-path", opts.FilePath,
	)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.ContextUploadedFile](out, "add-file")
}

// --- Context CRUD ---

// ListContexts returns active contexts. When includeArchived is true, returns all.
func (r *Runner) ListContexts(ctx context.Context, includeArchived bool) ([]appstore.Context, error) {
	args := []string{"context-list"}
	if includeArchived {
		args = append(args, "--include-archived=true")
	}
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[appstore.Context](out, "context-list")
}

// CreateContext creates a new context.
func (r *Runner) CreateContext(ctx context.Context, key string) (*appstore.Context, error) {
	out, err := r.run(ctx, []string{"context-create", "--key", key})
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.Context](out, "context-create")
}

// UpdateContext updates a context.
func (r *Runner) UpdateContext(ctx context.Context, id, key, status string) (*appstore.Context, error) {
	args := buildArgs("context-update",
		"--id", id, "--key", key, "--status", status)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.Context](out, "context-update")
}

// DeleteContext deletes a context.
func (r *Runner) DeleteContext(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"context-delete", "--id", id})
}

// ToggleContextStatus flips a context between active and archived.
func (r *Runner) ToggleContextStatus(ctx context.Context, id string) (*appstore.Context, error) {
	out, err := r.run(ctx, []string{"context-toggle-status", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.Context](out, "context-toggle-status")
}

// GetContextDetail returns full context detail by ID.
func (r *Runner) GetContextDetail(ctx context.Context, id string) (*jiraservice.ContextDetail, error) {
	out, err := r.run(ctx, []string{"context-get", "--context-id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[jiraservice.ContextDetail](out, "context-get")
}

// SetJiraConfig sets the JQL query for a context.
func (r *Runner) SetJiraConfig(ctx context.Context, contextID, searchQuery string) (*jirastore.JiraProvider, error) {
	out, err := r.run(ctx, buildArgs("set-jira-config",
		"--context-id", contextID, "--search-query", searchQuery))
	if err != nil {
		return nil, err
	}
	return unmarshal[jirastore.JiraProvider](out, "set-jira-config")
}

// SetSlackConfig sets the Slack config for a context.
func (r *Runner) SetSlackConfig(ctx context.Context, contextID, workspace, searchQuery string) (*slackstore.SlackProvider, error) {
	out, err := r.run(ctx, buildArgs("set-slack-config",
		"--context-id", contextID, "--workspace", workspace, "--search-query", searchQuery))
	if err != nil {
		return nil, err
	}
	return unmarshal[slackstore.SlackProvider](out, "set-slack-config")
}

// --- Text entries ---

// ListContextTexts returns text entries for a context.
func (r *Runner) ListContextTexts(ctx context.Context, contextID string) ([]appstore.ContextText, error) {
	out, err := r.run(ctx, []string{"text-list", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[appstore.ContextText](out, "text-list")
}

// UpsertContextText creates or updates a text entry.
func (r *Runner) UpsertContextText(ctx context.Context, id, contextID, name, content string) (*appstore.ContextText, error) {
	args := buildArgs("add-text",
		"--context-id", contextID, "--name", name, "--content", content)
	if id != "" {
		args = append(args, "--id", id)
	}
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.ContextText](out, "add-text")
}

// DeleteContextText removes a text entry.
func (r *Runner) DeleteContextText(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"text-delete", "--id", id})
}

// ToggleContextTextActive flips the is_active flag.
func (r *Runner) ToggleContextTextActive(ctx context.Context, id string) (*appstore.ContextText, error) {
	out, err := r.run(ctx, []string{"text-toggle", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.ContextText](out, "text-toggle")
}

// --- Uploaded files ---

// ListUploadedFiles returns uploaded files for a context.
func (r *Runner) ListUploadedFiles(ctx context.Context, contextID string) ([]appstore.ContextUploadedFile, error) {
	out, err := r.run(ctx, []string{"file-list", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[appstore.ContextUploadedFile](out, "file-list")
}

// DeleteUploadedFile removes an uploaded file.
func (r *Runner) DeleteUploadedFile(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"file-delete", "--id", id})
}

// ToggleUploadedFileActive flips the is_active flag.
func (r *Runner) ToggleUploadedFileActive(ctx context.Context, id string) (*appstore.ContextUploadedFile, error) {
	out, err := r.run(ctx, []string{"file-toggle", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.ContextUploadedFile](out, "file-toggle")
}

// --- Jira operations ---

// SearchNewIssues runs JQL and returns a diff.
func (r *Runner) SearchNewIssues(ctx context.Context, contextID string) (*jiraservice.SearchDiff, error) {
	out, err := r.run(ctx, []string{"jira-search", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshal[jiraservice.SearchDiff](out, "jira-search")
}

// AddIssuesToContext adds Jira keys to a context and syncs comments.
func (r *Runner) AddIssuesToContext(ctx context.Context, contextID string, jiraKeys []string) (int, int, error) {
	args := []string{"jira-add",
		"--context-id", contextID,
		"--jira-keys", strings.Join(jiraKeys, ","),
	}
	out, err := r.run(ctx, args)
	if err != nil {
		return 0, 0, err
	}
	var m map[string]int
	if err := json.Unmarshal(out, &m); err != nil {
		return 0, 0, fmt.Errorf("parsing jira-add response: %w", err)
	}
	return m["added"], m["newComments"], nil
}

// SyncAllComments syncs comments for all issues in a context.
func (r *Runner) SyncAllComments(ctx context.Context, contextID string) (int, error) {
	out, err := r.run(ctx, []string{"jira-sync-comments", "--context-id", contextID})
	if err != nil {
		return 0, err
	}
	var m map[string]int
	if err := json.Unmarshal(out, &m); err != nil {
		return 0, fmt.Errorf("parsing jira-sync-comments response: %w", err)
	}
	return m["newComments"], nil
}

// SearchJQL runs an arbitrary JQL query.
func (r *Runner) SearchJQL(ctx context.Context, jql string) ([]jira.SearchResult, error) {
	out, err := r.run(ctx, []string{"jira-search-jql", "--jql", jql})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[jira.SearchResult](out, "jira-search-jql")
}

// RefreshContext re-fetches all items in a context.
func (r *Runner) RefreshContext(ctx context.Context, contextID string) (*jiraservice.RefreshResult, error) {
	out, err := r.run(ctx, []string{"jira-refresh", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshal[jiraservice.RefreshResult](out, "jira-refresh")
}

// DeleteJiraIssue removes a Jira issue.
func (r *Runner) DeleteJiraIssue(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"jira-delete-issue", "--id", id})
}

// DeleteJiraComment removes a Jira comment.
func (r *Runner) DeleteJiraComment(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"jira-delete-comment", "--id", id})
}

// ToggleIssueActive flips the is_active flag.
func (r *Runner) ToggleIssueActive(ctx context.Context, id string) (*jirastore.Issue, error) {
	out, err := r.run(ctx, []string{"jira-toggle-issue", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[jirastore.Issue](out, "jira-toggle-issue")
}

// ToggleCommentActive flips the is_active flag.
func (r *Runner) ToggleCommentActive(ctx context.Context, id string) (*jirastore.Comment, error) {
	out, err := r.run(ctx, []string{"jira-toggle-comment", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[jirastore.Comment](out, "jira-toggle-comment")
}

// --- Slack channels ---

// ListSlackWorkspaces returns available Slack workspaces.
func (r *Runner) ListSlackWorkspaces(ctx context.Context) ([]jiraservice.SlackWorkspaceResult, error) {
	out, err := r.run(ctx, []string{"slack-list-workspaces"})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[jiraservice.SlackWorkspaceResult](out, "slack-list-workspaces")
}

// SearchSlackChannels searches channels by pattern.
func (r *Runner) SearchSlackChannels(ctx context.Context, workspace, pattern string) ([]jiraservice.SlackChannelResult, error) { //nolint:unparam
	out, err := r.run(ctx, []string{"slack-search-channels",
		"--workspace", workspace, "--pattern", pattern})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[jiraservice.SlackChannelResult](out, "slack-search-channels")
}

// AddSlackChannelsOpts holds parameters for adding channels to a context.
type AddSlackChannelsOpts struct {
	ContextID    string
	Workspace    string
	ChannelIDs   []string
	ChannelNames []string
	StartFrom    string
}

// AddSlackChannels adds Slack channels to a context.
func (r *Runner) AddSlackChannels(ctx context.Context, opts AddSlackChannelsOpts) error {
	if len(opts.ChannelIDs) == 0 {
		return nil
	}
	args := []string{"slack-add-channels",
		"--context-id", opts.ContextID,
		"--workspace", opts.Workspace,
		"--channel-ids", strings.Join(opts.ChannelIDs, ","),
		"--channel-names", strings.Join(opts.ChannelNames, ","),
	}
	if opts.StartFrom != "" {
		args = append(args, "--start-from", opts.StartFrom)
	}
	return r.runNoOutput(ctx, args)
}

// ListSlackChannels returns Slack channels for a context.
func (r *Runner) ListSlackChannels(ctx context.Context, contextID string) ([]slackstore.SlackChannel, error) {
	out, err := r.run(ctx, []string{"slack-list-channels", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[slackstore.SlackChannel](out, "slack-list-channels")
}

// RemoveSlackChannel removes a Slack channel.
func (r *Runner) RemoveSlackChannel(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"slack-remove-channel", "--id", id})
}

// ToggleSlackChannelActive flips the is_active flag.
func (r *Runner) ToggleSlackChannelActive(ctx context.Context, id string) (*slackstore.SlackChannel, error) {
	out, err := r.run(ctx, []string{"slack-toggle-channel", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[slackstore.SlackChannel](out, "slack-toggle-channel")
}

// --- Slack threads ---

// LoadSlackContextOpts holds parameters for loading a Slack thread.
type LoadSlackContextOpts struct {
	ContextID   string
	Workspace   string
	ChannelID   string
	ThreadTS    string
	ChannelName string
	Permalink   string
	SearchQuery string
}

// LoadSlackContext loads a Slack thread into a context.
func (r *Runner) LoadSlackContext(ctx context.Context, opts LoadSlackContextOpts) error {
	args := []string{"slack-load-thread",
		"--context-id", opts.ContextID,
		"--workspace", opts.Workspace,
		"--channel-id", opts.ChannelID,
		"--thread-ts", opts.ThreadTS,
	}
	if opts.ChannelName != "" {
		args = append(args, "--channel-name", opts.ChannelName)
	}
	if opts.Permalink != "" {
		args = append(args, "--permalink", opts.Permalink)
	}
	if opts.SearchQuery != "" {
		args = append(args, "--search-query", opts.SearchQuery)
	}
	return r.runNoOutput(ctx, args)
}

// ListSlackThreads returns Slack threads for a context.
func (r *Runner) ListSlackThreads(ctx context.Context, contextID string) ([]slackstore.SlackThread, error) {
	out, err := r.run(ctx, []string{"slack-list-threads", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[slackstore.SlackThread](out, "slack-list-threads")
}

// DeleteSlackThread removes a Slack thread.
func (r *Runner) DeleteSlackThread(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"slack-delete-thread", "--id", id})
}

// ToggleSlackThreadActive flips the is_active flag.
func (r *Runner) ToggleSlackThreadActive(ctx context.Context, id string) (*slackstore.SlackThread, error) {
	out, err := r.run(ctx, []string{"slack-toggle-thread", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[slackstore.SlackThread](out, "slack-toggle-thread")
}

// --- Slack DMs ---

// ListSlackDMConfigs returns DM configs for a context.
func (r *Runner) ListSlackDMConfigs(ctx context.Context, contextID string) ([]slackstore.SlackDMConfig, error) {
	out, err := r.run(ctx, []string{"slack-list-dm-configs", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[slackstore.SlackDMConfig](out, "slack-list-dm-configs")
}

// UpsertSlackDMConfig creates or updates a DM config and loads content.
func (r *Runner) UpsertSlackDMConfig(ctx context.Context, contextID, workspace, startFrom string) (*slackstore.SlackDMConfig, error) {
	args := buildArgs("slack-upsert-dm-config",
		"--context-id", contextID, "--workspace", workspace, "--start-from", startFrom)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[slackstore.SlackDMConfig](out, "slack-upsert-dm-config")
}

// DeleteSlackDMConfig removes a DM config.
func (r *Runner) DeleteSlackDMConfig(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"slack-delete-dm-config", "--id", id})
}

// ToggleSlackDMConfigActive flips the is_active flag.
func (r *Runner) ToggleSlackDMConfigActive(ctx context.Context, id string) (*slackstore.SlackDMConfig, error) {
	out, err := r.run(ctx, []string{"slack-toggle-dm-config", "--id", id})
	if err != nil {
		return nil, err
	}
	return unmarshal[slackstore.SlackDMConfig](out, "slack-toggle-dm-config")
}

// RemoveSlackDMEntry removes a DM entry from a config.
func (r *Runner) RemoveSlackDMEntry(ctx context.Context, dmConfigID, entryID string) error {
	return r.runNoOutput(ctx, []string{"slack-remove-dm-entry",
		"--dm-config-id", dmConfigID, "--entry-id", entryID})
}

// PreviewSlackDMs previews DMs without persisting.
func (r *Runner) PreviewSlackDMs(ctx context.Context, workspace, startFrom string) (string, error) {
	args := buildArgs("slack-preview-dms", "--workspace", workspace, "--start-from", startFrom)
	out, err := r.run(ctx, args)
	if err != nil {
		return "", err
	}
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		return "", fmt.Errorf("parsing slack-preview-dms response: %w", err)
	}
	return m["content"], nil
}

// --- Context logs ---

// ListContextLogs returns log entries for a context.
func (r *Runner) ListContextLogs(ctx context.Context, contextID string) ([]contextlog.ContextLog, error) {
	out, err := r.run(ctx, []string{"log-list", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[contextlog.ContextLog](out, "log-list")
}

// CreateContextLog creates a manual log entry.
func (r *Runner) CreateContextLog(ctx context.Context, contextID, summaryMd string) (*contextlog.ContextLog, error) {
	out, err := r.run(ctx, []string{"log-create",
		"--context-id", contextID, "--summary-md", summaryMd})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ContextLog](out, "log-create")
}

// UpdateContextLog updates a log entry.
func (r *Runner) UpdateContextLog(ctx context.Context, id, summaryMd string) (*contextlog.ContextLog, error) {
	out, err := r.run(ctx, []string{"log-update",
		"--id", id, "--summary-md", summaryMd})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ContextLog](out, "log-update")
}

// DeleteContextLog removes a log entry.
func (r *Runner) DeleteContextLog(ctx context.Context, id string) error {
	return r.runNoOutput(ctx, []string{"log-delete", "--id", id})
}

// AttachLogFile links an uploaded file to a log entry.
func (r *Runner) AttachLogFile(ctx context.Context, logID, fileID string) error {
	return r.runNoOutput(ctx, []string{"log-attach-file", "--log-id", logID, "--file-id", fileID})
}

// DetachLogFile removes the link between a log entry and an uploaded file.
func (r *Runner) DetachLogFile(ctx context.Context, logID, fileID string) error {
	return r.runNoOutput(ctx, []string{"log-detach-file", "--log-id", logID, "--file-id", fileID})
}

// --- Consolidated log ---

// GetConsolidatedContextLog returns the consolidated log.
func (r *Runner) GetConsolidatedContextLog(ctx context.Context, contextID string) (*contextlog.ConsolidatedLog, error) {
	out, err := r.run(ctx, []string{"consolidated-get", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ConsolidatedLog](out, "consolidated-get")
}

// UpdateConsolidatedContextLog overwrites the consolidated log.
func (r *Runner) UpdateConsolidatedContextLog(ctx context.Context, contextID, contentMd string) (*contextlog.ConsolidatedLog, error) {
	out, err := r.run(ctx, []string{"consolidated-update",
		"--context-id", contextID, "--content-md", contentMd})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ConsolidatedLog](out, "consolidated-update")
}

// InitializeConsolidatedLog performs AI bootstrap of the consolidated log.
func (r *Runner) InitializeConsolidatedLog(ctx context.Context, contextID string) (*contextlog.ConsolidatedLog, error) {
	out, err := r.run(ctx, []string{"consolidated-init", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ConsolidatedLog](out, "consolidated-init")
}

// AnalyzeContextChanges triggers AI change-log analysis.
func (r *Runner) AnalyzeContextChanges(ctx context.Context, contextID string) (*contextlog.ContextLog, error) {
	out, err := r.run(ctx, []string{"consolidated-analyze", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ContextLog](out, "consolidated-analyze")
}

// --- Chat ---

// ListContextChat returns chat messages for a context.
func (r *Runner) ListContextChat(ctx context.Context, contextID string) ([]contextlog.ChatMessage, error) {
	out, err := r.run(ctx, []string{"chat-list", "--context-id", contextID})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[contextlog.ChatMessage](out, "chat-list")
}

// ChatWithContext sends a message and returns the AI reply.
func (r *Runner) ChatWithContext(ctx context.Context, contextID, message string) (*contextlog.ChatMessage, error) {
	out, err := r.run(ctx, []string{"chat-send",
		"--context-id", contextID, "--message", message})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ChatMessage](out, "chat-send")
}

// ApplyChatSuggestion applies an AI-proposed consolidated log rewrite.
func (r *Runner) ApplyChatSuggestion(ctx context.Context, messageID string) (*contextlog.ConsolidatedLog, error) {
	out, err := r.run(ctx, []string{"chat-apply-suggestion", "--message-id", messageID})
	if err != nil {
		return nil, err
	}
	return unmarshal[contextlog.ConsolidatedLog](out, "chat-apply-suggestion")
}

// --- Auth & AI settings ---

// AuthStatus returns the current acli authentication status.
func (r *Runner) AuthStatus(ctx context.Context) (string, error) {
	out, err := r.run(ctx, []string{"auth-status"})
	if err != nil {
		return "", err
	}
	var m map[string]string
	if err := json.Unmarshal(out, &m); err != nil {
		return "", fmt.Errorf("parsing auth-status response: %w", err)
	}
	return m["status"], nil
}

// AISettingsResult holds AI settings metadata plus the source of the active key,
// mirroring the JSON emitted by the ai-settings-get CLI subcommand.
type AISettingsResult struct {
	Provider        string `json:"provider"`
	HasKey          bool   `json:"hasKey"`
	HasOpenAIKey    bool   `json:"hasOpenAIKey"`
	HasAnthropicKey bool   `json:"hasAnthropicKey"`
	Model           string `json:"model"`
	BaseURL         string `json:"baseUrl"`
	Source          string `json:"source"`
	UpdatedAt       string `json:"updatedAt"`
}

// GetAISettings returns AI settings metadata including the source (env/db/none).
func (r *Runner) GetAISettings(ctx context.Context) (*AISettingsResult, error) {
	out, err := r.run(ctx, []string{"ai-settings-get"})
	if err != nil {
		return nil, err
	}
	return unmarshal[AISettingsResult](out, "ai-settings-get")
}

// ReloadAllContexts re-fetches data for every active context from all providers.
func (r *Runner) ReloadAllContexts(ctx context.Context) ([]jiraservice.ReloadAllResult, error) {
	out, err := r.run(ctx, []string{"reload-all"})
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[jiraservice.ReloadAllResult](out, "reload-all")
}

// SetAISettingsOpts holds AI settings to update.
type SetAISettingsOpts struct {
	Provider     string
	APIKey       string
	AnthropicKey string
	Model        string
	BaseURL      string
}

// SetAISettings saves AI configuration.
func (r *Runner) SetAISettings(ctx context.Context, opts SetAISettingsOpts) (*appstore.AISettings, error) {
	args := buildArgs("ai-settings-set",
		"--provider", opts.Provider,
		"--api-key", opts.APIKey,
		"--anthropic-key", opts.AnthropicKey,
		"--model", opts.Model,
		"--base-url", opts.BaseURL,
	)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshal[appstore.AISettings](out, "ai-settings-set")
}
