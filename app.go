package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/artschekoff/context-consolidator/internal/jira"
	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/slack"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// App is the Wails application struct. All exported methods are bound to the frontend.
type App struct {
	ctx           context.Context
	svc           *jiraservice.ContextService
	db            *appstore.Store
	logger        *zap.Logger
	slackRunner   *slack.Runner
	cancelRefresh context.CancelFunc
	cancelMu      sync.Mutex
}

// NewApp creates a new App with uninitialized service (configured in startup).
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	logger, err := buildLogger()
	if err != nil {
		panic(fmt.Sprintf("initializing logger: %v", err))
	}
	a.logger = logger

	s, err := appstore.Open(appstore.DefaultDBPath())
	if err != nil {
		logger.Fatal("opening database", zap.Error(err))
	}
	a.db = s

	aRunner := acli.NewRunner(logger)
	jiraClient := jira.NewClient(aRunner, logger)
	slackRunner := slack.NewRunner(logger)
	a.slackRunner = slackRunner

	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())

	jiraProvider := jiraservice.NewJiraProvider(jiraRepo, jiraClient, logger)
	slackProvider := jiraservice.NewSlackProvider(slackRepo, slackRunner, logger)

	a.svc = jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider)

	logger.Info("context-consolidator desktop started")
}

// shutdown is called by Wails when the app is closing.
func (a *App) shutdown(_ context.Context) {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Warn("closing database", zap.Error(err))
		}
	}
	if a.logger != nil {
		a.logger.Sync() //nolint:errcheck
	}
}

// --- Auth ---

// AuthStatus returns the current acli authentication status.
func (a *App) AuthStatus() string {
	status, err := a.svc.AuthStatus(a.ctx)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return status
}

// SetSlackPassphrase stores the SLACK_MCP_PASSPHRASE value that will be injected
// as an environment variable into every subsequent slack-cli invocation.
func (a *App) SetSlackPassphrase(passphrase string) {
	a.slackRunner.SetPassphrase(passphrase)
}

// --- Context CRUD ---

// ListContexts returns all saved contexts.
func (a *App) ListContexts() ([]appstore.Context, error) {
	return a.svc.ListContexts(a.ctx)
}

// CreateContext creates a new context.
func (a *App) CreateContext(key string) (*appstore.Context, error) {
	return a.svc.CreateContext(a.ctx, key)
}

// UpdateContext updates an existing context.
func (a *App) UpdateContext(id, key, status string) (*appstore.Context, error) {
	return a.svc.UpdateContext(a.ctx, id, key, status)
}

// DeleteContext removes a context and all its data.
func (a *App) DeleteContext(id string) error {
	return a.svc.DeleteContext(a.ctx, id)
}

// GetContextDetail returns a context with provider configs, issues, and Slack threads.
func (a *App) GetContextDetail(id string) (*jiraservice.ContextDetail, error) {
	return a.svc.GetContextDetail(a.ctx, id)
}

// --- Provider config ---

// SetJiraConfig sets the Jira search query (JQL) for a context.
func (a *App) SetJiraConfig(contextID, searchQuery string) (*jirastore.JiraProvider, error) {
	return a.svc.SetJiraConfig(a.ctx, contextID, searchQuery)
}

// SetSlackConfig sets the Slack workspace and search query for a context.
func (a *App) SetSlackConfig(contextID, workspace, searchQuery string) (*slackstore.SlackProvider, error) {
	return a.svc.SetSlackConfig(a.ctx, contextID, workspace, searchQuery)
}

// --- Jira refresh flow ---

// SearchNewIssues runs the context's saved JQL and returns which issues are new vs existing.
func (a *App) SearchNewIssues(contextID string) (*jiraservice.SearchDiff, error) {
	return a.svc.SearchNewIssues(a.ctx, contextID)
}

// AddIssuesToContext fetches and saves the given Jira keys into a context.
// Returns the count of issues successfully added.
func (a *App) AddIssuesToContext(contextID string, jiraKeys []string) (int, error) {
	return a.svc.AddIssuesToContext(a.ctx, contextID, jiraKeys)
}

// SyncAllComments fetches and deduplicates comments for all issues in a context.
func (a *App) SyncAllComments(contextID string) (int, error) {
	return a.svc.SyncAllComments(a.ctx, contextID)
}

// RefreshContext re-fetches field values and comments for every existing issue in the
// context and reloads all linked Slack threads. It does NOT run the JQL query.
// Emits "refresh:progress" events to the frontend as each provider is processed.
// A concurrent call to CancelRefresh will abort the operation early.
func (a *App) RefreshContext(contextID string) (*jiraservice.RefreshResult, error) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelMu.Lock()
	a.cancelRefresh = cancel
	a.cancelMu.Unlock()
	defer func() {
		cancel()
		a.cancelMu.Lock()
		a.cancelRefresh = nil
		a.cancelMu.Unlock()
	}()

	return a.svc.RefreshContext(ctx, contextID, func(current, total int, step string) {
		runtime.EventsEmit(a.ctx, "refresh:progress", map[string]any{
			"current": current,
			"total":   total,
			"step":    step,
		})
	})
}

// CancelRefresh aborts a running RefreshContext call, if any.
func (a *App) CancelRefresh() {
	a.cancelMu.Lock()
	fn := a.cancelRefresh
	a.cancelMu.Unlock()
	if fn != nil {
		fn()
	}
}

// --- Slack ---

// LoadSlackContext loads a Slack thread and stores it in the context.
func (a *App) LoadSlackContext(contextID, workspace, channelID, threadTS, channelName, permalink, searchQuery string) error {
	return a.svc.LoadSlackContext(a.ctx, jiraservice.LoadSlackContextArgs{
		ContextID:   contextID,
		Workspace:   workspace,
		ChannelID:   channelID,
		ThreadTS:    threadTS,
		ChannelName: channelName,
		Permalink:   permalink,
		SearchQuery: searchQuery,
	})
}

// ListSlackThreads returns all stored Slack threads for a context.
func (a *App) ListSlackThreads(contextID string) ([]slackstore.SlackThread, error) {
	return a.svc.ListSlackThreads(a.ctx, contextID)
}

// --- Slack channels ---

// ListSlackWorkspaces returns the workspaces available via slack-cli.
func (a *App) ListSlackWorkspaces() ([]jiraservice.SlackWorkspaceResult, error) {
	return a.svc.ListSlackWorkspaces(a.ctx)
}

// SearchSlackChannels searches Slack channels by name pattern via slack-cli.
// namePattern supports glob-style wildcards, e.g. "*deploy*".
func (a *App) SearchSlackChannels(workspace, namePattern string) ([]jiraservice.SlackChannelResult, error) {
	return a.svc.SearchSlackChannels(a.ctx, workspace, namePattern)
}

// AddSlackChannels adds one or more Slack channels to a context.
func (a *App) AddSlackChannels(contextID, workspace string, channels []slackstore.SlackChannelInput) error {
	return a.svc.AddSlackChannels(a.ctx, contextID, workspace, channels)
}

// ListSlackChannels returns all Slack channels configured for a context.
func (a *App) ListSlackChannels(contextID string) ([]slackstore.SlackChannel, error) {
	return a.svc.ListSlackChannels(a.ctx, contextID)
}

// RemoveSlackChannel removes a Slack channel from a context by its record ID.
func (a *App) RemoveSlackChannel(id string) error {
	return a.svc.RemoveSlackChannel(a.ctx, id)
}

// DeleteJiraIssue removes a single Jira issue and all its comments from a context.
func (a *App) DeleteJiraIssue(id string) error {
	return a.svc.DeleteJiraIssue(a.ctx, id)
}

// DeleteJiraComment removes a single Jira comment by its internal ID.
func (a *App) DeleteJiraComment(id string) error {
	return a.svc.DeleteJiraComment(a.ctx, id)
}

// DeleteSlackThread removes a single Slack thread by its internal ID.
func (a *App) DeleteSlackThread(id string) error {
	return a.svc.DeleteSlackThread(a.ctx, id)
}

// SearchJQL runs an arbitrary JQL query and returns matching issues for preview / validation.
// It does not require a saved context — results are returned directly to the caller.
func (a *App) SearchJQL(jql string) ([]jira.SearchResult, error) {
	return a.svc.SearchJQL(a.ctx, jql)
}

// ReloadAllContexts fetches fresh data from every context's linked Jira query and Slack threads.
// Returns one result per context; per-context errors are embedded in the result, not returned as
// a top-level error, so the caller always receives the full picture.
// Emits "reload:progress" events to the frontend as each context is processed.
func (a *App) ReloadAllContexts() ([]jiraservice.ReloadAllResult, error) {
	return a.svc.ReloadAllContexts(a.ctx, func(current, total int, contextKey string) {
		runtime.EventsEmit(a.ctx, "reload:progress", map[string]any{
			"current":    current,
			"total":      total,
			"contextKey": contextKey,
		})
	})
}

func buildLogger() (*zap.Logger, error) {
	logPath := logFilePath()
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stderr", logPath}
	cfg.ErrorOutputPaths = []string{"stderr", logPath}
	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}
	return logger, nil
}

func logFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".context-consolidator")
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "app.log")
}
