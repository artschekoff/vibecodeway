package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artschekoff/vibecodeway/internal/agent"
	"github.com/artschekoff/vibecodeway/internal/analysis"
	"github.com/artschekoff/vibecodeway/internal/applog"
	cliclient "github.com/artschekoff/vibecodeway/internal/cli"
	"github.com/artschekoff/vibecodeway/internal/extract"
	"github.com/artschekoff/vibecodeway/internal/hotkey"
	"github.com/artschekoff/vibecodeway/internal/jira"
	"github.com/artschekoff/vibecodeway/internal/jira/acli"
	jiraservice "github.com/artschekoff/vibecodeway/internal/jira/service"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	openaiclient "github.com/artschekoff/vibecodeway/internal/openai"
	"github.com/artschekoff/vibecodeway/internal/pty"
	"github.com/artschekoff/vibecodeway/internal/slack"
	slackstore "github.com/artschekoff/vibecodeway/internal/slack/store"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	githubrunner "github.com/artschekoff/vibecodeway/internal/vcs/github"
	vcsservice "github.com/artschekoff/vibecodeway/internal/vcs/service"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// PendingUpload holds extracted file data in memory before a context is created.
type PendingUpload struct {
	OriginalName  string `json:"-"`
	FileType      string `json:"-"`
	ExtractedText string `json:"-"`
	FileContent   []byte `json:"-"`
	FileSize      int64  `json:"-"`
}

// PendingUploadPreview is the JSON-safe shape returned to the frontend.
type PendingUploadPreview struct {
	TempID        string `json:"tempId"`
	OriginalName  string `json:"originalName"`
	FileType      string `json:"fileType"`
	ExtractedText string `json:"extractedText"`
	FileSize      int64  `json:"fileSize"`
}

// userVisibleExtractionError returns a non-empty user-facing message for errors
// that should be shown directly to the user (e.g. blank image, file too large).
// Returns "" for internal errors that should only be logged.
func userVisibleExtractionError(err error, name string) string {
	var noText *extract.ErrNoTextFound
	if errors.As(err, &noText) {
		return name + ": no readable text found — the image may be blank or contain non-text content"
	}
	var tooLarge *extract.ErrFileTooLarge
	if errors.As(err, &tooLarge) {
		return fmt.Sprintf("%s: file is too large (%d MB, max %d MB)", name,
			tooLarge.Size/(1024*1024), extract.MaxFileBytes/(1024*1024))
	}
	return ""
}

// UploadBatchResult is returned by PickAndExtractFiles. Files contains the
// successfully processed uploads; Errors contains user-readable messages for
// files that were rejected (e.g. images with no readable text).
type UploadBatchResult struct {
	Files  []appstore.ContextUploadedFile `json:"files"`
	Errors []string                       `json:"errors"`
}

// UploadPreviewBatchResult is returned by PickAndExtractFilesPreview.
type UploadPreviewBatchResult struct {
	Previews []PendingUploadPreview `json:"previews"`
	Errors   []string               `json:"errors"`
}

// App is the Wails application struct. All exported methods are bound to the frontend.
type App struct {
	ctx           context.Context
	svc           *jiraservice.ContextService
	analysisSvc   *analysis.Service
	agent         *agent.Agent
	openAI        *openaiclient.Client
	db            *appstore.Store
	logger        *zap.Logger
	slackRunner   *slack.Runner
	cliRunner     *cliclient.Runner
	vcsStore      *vcsstore.Store
	vcsSvc        *vcsservice.VCSProvider
	appLogRepo    *appstore.AppLogRepo
	cancelRefresh context.CancelFunc
	cancelMu      sync.Mutex

	pendingMu      sync.Mutex
	pendingUploads map[string]*PendingUpload

	hotkeyService  *hotkey.Service
	pickerMu       sync.Mutex
	pendingText    string
	hotkeyInFlight atomic.Bool
	a11yPolling    atomic.Bool

	bg bgRefreshState

	// orgBackfillRunning guards the board's per-key org lookup so overlapping
	// board refreshes don't start a second (slow) backfill loop.
	orgBackfillRunning atomic.Bool

	ptyMgr *pty.Manager
	emit   func(name string, data ...interface{})
}

// NewApp creates a new App with uninitialized service (configured in startup).
func NewApp() *App {
	app := &App{}
	app.ptyMgr = pty.NewManager()
	return app
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if a.emit == nil {
		a.emit = func(name string, data ...interface{}) {
			runtime.EventsEmit(a.ctx, name, data...)
		}
	}

	logger, err := buildLogger()
	if err != nil {
		panic(fmt.Sprintf("initializing logger: %v", err))
	}
	a.logger = logger

	if err := appstore.MigrateLegacyDataDir(); err != nil {
		logger.Warn("data dir migration failed; continuing", zap.Error(err))
	}

	s, err := appstore.Open(appstore.DefaultDBPath())
	if err != nil {
		logger.Fatal("opening database", zap.Error(err))
	}
	a.db = s

	a.appLogRepo = appstore.NewAppLogRepo(s.DB())
	a.logger = logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, applog.NewDBCore(ctx, a.appLogRepo))
	}))
	logger = a.logger

	aRunner := acli.NewRunner(logger)
	jiraClient := jira.NewClient(aRunner, logger)
	slackRunner := slack.NewRunner(logger)
	a.slackRunner = slackRunner
	a.cliRunner = cliclient.NewRunner(logger, cliclient.WithPassphraseFunc(slackRunner.Passphrase))

	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())

	jiraProvider := jiraservice.NewJiraProvider(jiraRepo, jiraClient, logger, s)
	jiraProvider.WithNotifyFunc(a.onJiraNotify)
	slackProvider := jiraservice.NewSlackProvider(slackRepo, slackRunner, logger, s)
	textProvider := jiraservice.NewTextProvider(s, logger)

	ghRunner := githubrunner.NewRunner(logger)
	a.vcsStore = vcsstore.NewStore(s.DB())
	a.vcsSvc = vcsservice.NewVCSProvider(a.vcsStore, logger, s, ghRunner)
	// Bootstrap the runner with any previously stored GitHub token so gh CLI calls
	// work immediately on startup without requiring keychain access.
	if auth, err := a.vcsStore.Auth.GetAuth(ctx, "github"); err == nil && auth.Token != "" {
		ghRunner.SetToken(auth.Token)
	}
	a.vcsSvc.WithNotifyFunc(a.onVCSNotify)
	a.vcsSvc.WithReviewNotifyFunc(a.onReviewNotify)
	a.vcsSvc.WithCommentNotifyFunc(func(ctx context.Context, contextID, prID, externalID, author, body string) {
		summary := "New comment"
		if author != "" {
			summary = "New comment from " + author
		}
		a.emitDomainEvent(contextID, appstore.EventTypeCommentAdded, appstore.EventSourceGitHub, externalID, author, summary)
	})
	vcsProvider := a.vcsSvc

	a.svc = jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider, textProvider, vcsProvider)
	a.svc.SetVCSStore(a.vcsStore)

	openAI := openaiclient.NewClient(s, logger)
	a.openAI = openAI
	a.analysisSvc = analysis.NewService(s, a.svc, openAI, logger)
	a.analysisSvc.SetVCSStore(a.vcsStore)
	reg := tooling.NewRegistry(tooling.Deps{Store: s, Svc: a.svc, VCS: ghRunner, Logger: logger})
	a.agent = agent.New(openAI, reg, s, logger)

	a.hotkeyService = hotkey.New(a.onHotkey)
	{
		kb, err := s.GetKeybindingByCommand(context.Background(), "quick_attach")
		if err != nil {
			kb = &appstore.Keybinding{ModifierKeys: "cmd,shift", KeyCode: "c"}
			logger.Warn("hotkey: could not load saved binding, using default", zap.Error(err))
		}
		if err := a.hotkeyService.Start(kb.ModifierKeys, kb.KeyCode); err != nil {
			logger.Warn("hotkey service: failed to register", zap.Error(err))
		}
	}

	a.startBackgroundRefresh()

	if err := a.db.BackfillMessages(ctx); err != nil {
		a.logger.Warn("startup: backfill messages failed", zap.Error(err))
	}

	logger.Info("vibecodeway desktop started")
}

// domReady is called by Wails once the frontend DOM is loaded and event listeners
// are active.
func (a *App) domReady(_ context.Context) {}

// shutdown is called by Wails when the app is closing.
func (a *App) shutdown(_ context.Context) {
	a.stopBackgroundRefresh()
	if a.ptyMgr != nil {
		a.ptyMgr.CloseAll()
	}
	if a.hotkeyService != nil {
		a.hotkeyService.Stop()
	}
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
	status, err := a.cliRunner.AuthStatus(a.ctx)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return status
}

// GetJiraInstanceURL returns the Jira instance base URL derived from acli auth status.
// Returns empty string when not authenticated or URL cannot be determined.
func (a *App) GetJiraInstanceURL() string {
	return a.svc.GetJiraInstanceURL(a.ctx)
}

// TransitionJiraIssue changes a Jira issue's status remotely (sent to Jira via
// acli) and updates the local copy. status is the target status name.
func (a *App) TransitionJiraIssue(key, status string) error {
	return a.svc.TransitionJiraIssue(a.ctx, key, status)
}

// ListKnownJiraStatuses returns the distinct statuses seen on synced Jira issues.
// Feeds the status-change dropdown, which would otherwise offer invented names.
func (a *App) ListKnownJiraStatuses() ([]string, error) {
	return a.db.ListKnownJiraStatuses(a.ctx)
}

// SetSlackPassphrase stores the SLACK_MCP_PASSPHRASE value that will be injected
// as an environment variable into every subsequent slack-cli invocation.
func (a *App) SetSlackPassphrase(passphrase string) error {
	return a.slackRunner.SetPassphrase(passphrase)
}

// HasSlackPassphrase reports whether a non-empty Slack passphrase is currently held
// in memory. Returns true when set via the env var at startup or via SetSlackPassphrase.
func (a *App) HasSlackPassphrase() bool {
	return a.slackRunner.Passphrase() != ""
}

// ValidateSlackPassphrase sets the passphrase and verifies it is correct by making a
// lightweight list-workspaces call. Returns nil when the passphrase decrypts credentials
// successfully. On failure the passphrase is cleared and the slack-cli error is returned.
func (a *App) ValidateSlackPassphrase(passphrase string) error {
	if err := a.slackRunner.SetPassphrase(passphrase); err != nil {
		return err
	}
	if _, err := a.slackRunner.Run(context.Background(), []string{"list-workspaces"}, nil); err != nil {
		_ = a.slackRunner.SetPassphrase("")
		return err
	}
	return nil
}

// --- Context CRUD ---

// ListContexts returns active contexts only.
func (a *App) ListContexts() ([]appstore.Context, error) {
	return a.cliRunner.ListContexts(a.ctx, false)
}

// ListAllContexts returns all contexts including archived ones.
func (a *App) ListAllContexts() ([]appstore.Context, error) {
	return a.cliRunner.ListContexts(a.ctx, true)
}

// SearchContextIDs returns IDs of contexts matching the query across all indexed
// text: keys, Jira summaries/descriptions/comments, Slack channels & thread
// content, VCS branches/repos, text names & bodies, and uploaded file text.
func (a *App) SearchContextIDs(query string) ([]string, error) {
	return a.db.SearchContextIDs(a.ctx, query)
}

// SearchContexts returns ranked cross-org search hits (id, key, org, primary
// summary) for the global search palette.
func (a *App) SearchContexts(query string) ([]appstore.ContextSearchHit, error) {
	return a.db.SearchContexts(a.ctx, query)
}

// SearchContextItems returns the elements inside a single context whose indexed
// text matches the query, grouped by element type. Powers the in-context filter.
func (a *App) SearchContextItems(contextID, query string) (*appstore.ContextItemMatches, error) {
	return a.db.SearchContextItems(a.ctx, contextID, query)
}

// AskAI answers a natural-language question by letting an LLM agent navigate the
// stored contexts through read-only tools (cross-context search, drill-down,
// full-text read, and relation following). Returns the answer plus the tool trace.
// orgID restricts search to one organisation; empty string searches all.
func (a *App) AskAI(question, orgID string) (*agent.Answer, error) {
	return a.agent.Ask(a.ctx, question, orgID, func(ev agent.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "ask:progress", ev)
	})
}

// ListAgentTools returns the tools the Ask AI agent can call, for the on-page reference.
func (a *App) ListAgentTools() []agent.ToolInfo {
	return a.agent.ToolCatalogue()
}

// ResolveToolConfirmation delivers the user's decision for a write tool the Ask
// AI agent proposed. callID is the CallID from the "confirm" progress event.
func (a *App) ResolveToolConfirmation(callID string, approved bool) {
	a.agent.ResolveConfirmation(callID, approved)
}

// ToggleContextStatus flips a context between "active" and "archived".
func (a *App) ToggleContextStatus(id string) (*appstore.Context, error) {
	return a.cliRunner.ToggleContextStatus(a.ctx, id)
}

// CountOpenTickets returns how many of a context's Jira tickets are not yet
// terminal (done/closed/…). The UI calls this before a manual deactivation so it
// can warn when hiding a context that still has open work.
func (a *App) CountOpenTickets(id string) (int, error) {
	return a.db.CountOpenTickets(a.ctx, id)
}

// SetContextsOrder persists the display order of contexts.
// ids must contain the full ordered list of context IDs from top to bottom.
func (a *App) SetContextsOrder(ids []string) error {
	return a.db.SetContextsOrder(a.ctx, ids)
}

// ListOrganizations returns all organizations ordered by sort_order.
func (a *App) ListOrganizations() ([]appstore.Organization, error) {
	return a.db.ListOrganizations(a.ctx)
}

// CreateOrganization creates a new named organization.
func (a *App) CreateOrganization(name string) (*appstore.Organization, error) {
	return a.db.CreateOrganization(a.ctx, name)
}

// UpdateOrganization renames an organization.
func (a *App) UpdateOrganization(id, name string) (*appstore.Organization, error) {
	return a.db.UpdateOrganization(a.ctx, id, name)
}

// DeleteOrganization removes an organization. Contexts' org_id is set to NULL via FK cascade.
func (a *App) DeleteOrganization(id string) error {
	return a.db.DeleteOrganization(a.ctx, id)
}

// SetOrganizationsOrder persists the display order of organizations.
func (a *App) SetOrganizationsOrder(ids []string) error {
	return a.db.SetOrganizationsOrder(a.ctx, ids)
}

// SetContextOrg reassigns a context to a different organization.
func (a *App) SetContextOrg(contextID, orgID string) error {
	return a.db.SetContextOrg(a.ctx, contextID, orgID)
}

// ListContextsByOrg returns all contexts belonging to a given organization.
func (a *App) ListContextsByOrg(orgID string) ([]appstore.Context, error) {
	// needs_attention is set by sync notifications and was never taken back off.
	// Sweep the flags that no longer mean anything before reading the list, so the
	// Needs Attention panel only shows contexts the user still has to act on.
	if n, err := a.db.ClearStaleNeedsAttention(a.ctx); err != nil {
		a.logger.Warn("clear stale needs_attention failed", zap.Error(err))
	} else if n > 0 {
		a.logger.Info("cleared stale needs_attention flags", zap.Int("count", n))
	}
	return a.db.ListContextsByOrg(a.ctx, orgID)
}

// CreateContext creates a new context inside the specified organization.
func (a *App) CreateContext(key, orgID string) (*appstore.Context, error) {
	return a.cliRunner.CreateContext(a.ctx, key, orgID)
}

// UpdateContext updates an existing context.
func (a *App) UpdateContext(id, key, status string) (*appstore.Context, error) {
	return a.cliRunner.UpdateContext(a.ctx, id, key, status)
}

// DeleteContext removes a context and all its data.
func (a *App) DeleteContext(id string) error {
	return a.cliRunner.DeleteContext(a.ctx, id)
}

// GetContextDetail returns a context with provider configs, issues, and Slack threads.
func (a *App) GetContextDetail(id string) (*jiraservice.ContextDetail, error) {
	return a.cliRunner.GetContextDetail(a.ctx, id)
}

// --- Provider config ---

// SetJiraConfig sets the Jira search query (JQL) for a context.
func (a *App) SetJiraConfig(contextID, searchQuery string) (*jirastore.JiraProvider, error) {
	return a.cliRunner.SetJiraConfig(a.ctx, contextID, searchQuery)
}

// SetSlackConfig sets the Slack workspace and search query for a context.
func (a *App) SetSlackConfig(contextID, workspace, searchQuery string) (*slackstore.SlackProvider, error) {
	return a.cliRunner.SetSlackConfig(a.ctx, contextID, workspace, searchQuery)
}

// --- Jira refresh flow ---

// SearchNewIssues runs the context's saved JQL and returns which issues are new vs existing.
func (a *App) SearchNewIssues(contextID string) (*jiraservice.SearchDiff, error) {
	return a.cliRunner.SearchNewIssues(a.ctx, contextID)
}

// AddIssuesToContext fetches and saves the given Jira keys into a context.
// Also adds subtask children and auto-links any GitHub repositories found in issue descriptions.
// Returns the count of issues successfully added.
func (a *App) AddIssuesToContext(contextID string, jiraKeys []string) (int, error) {
	added, err := a.svc.AddIssuesToContext(a.ctx, contextID, jiraKeys)
	if err != nil {
		return added, err
	}
	// Sync comments (best-effort — same path as CLI jira-add)
	if _, syncErr := a.svc.SyncAllComments(a.ctx, contextID); syncErr != nil {
		a.logger.Sugar().Warnf("sync comments after issue add: %v", syncErr)
	}
	// Auto-add GitHub repos detected in issue descriptions (best-effort)
	repos := a.svc.ExtractGitHubReposFromIssues(a.ctx, contextID, jiraKeys)
	for _, ownerRepo := range repos {
		if _, addErr := a.AddVCSBranch(contextID, "github", ownerRepo, "", "", true); addErr != nil {
			a.logger.Sugar().Warnf("auto-add repo %q from Jira issue: %v", ownerRepo, addErr)
		}
	}
	return added, nil
}

// SyncAllComments fetches and deduplicates comments for all issues in a context.
func (a *App) SyncAllComments(contextID string) (int, error) {
	return a.cliRunner.SyncAllComments(a.ctx, contextID)
}

// RefreshContext re-fetches field values and comments for every existing item in the
// context (issues, Slack threads, pull requests). It does NOT run JQL or add new items.
// Progress events ("refresh:progress") are emitted via the Wails runtime before each provider.
// notifyReason builds a short human-readable reason for an automatic status flip
// to needs_attention, from a change notification's fields. Shown next to the
// context Status so the user can see WHY it needs attention.
func notifyReason(entityType, eventType, oldValue, newValue string) string {
	label := strings.TrimSpace(entityType + " " + eventType)
	if label == "" {
		label = "activity"
	}
	switch {
	case oldValue != "" && newValue != "":
		return fmt.Sprintf("New %s (%s → %s)", label, oldValue, newValue)
	case newValue != "":
		return fmt.Sprintf("New %s: %s", label, newValue)
	default:
		return fmt.Sprintf("New %s", label)
	}
}

// onJiraNotify records a detected Jira change: de-duped notification, needs-attention
// flag, hook dispatch, and (on a newly-inserted notification) a timeline event.
func (a *App) onJiraNotify(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error {
	inserted, err := a.db.CreateNotification(ctx, contextID, entityType, entityID, eventType, oldValue, newValue, "")
	if err != nil {
		return err
	}
	if inserted {
		a.dispatchContextHooks(contextID, "jira_update")
		// Prefix with the issue key so two different issues with the same transition
		// (e.g. a parent and its dev sub-task both → Ready for QA) don't render as
		// identical timeline summaries. entityID is the jira_issues row id.
		reason := notifyReason(entityType, eventType, oldValue, newValue)
		if key, keyErr := a.db.GetJiraKeyByID(ctx, entityID); keyErr == nil && key != "" {
			reason = key + " — " + reason
		}
		a.emitDomainEvent(contextID, eventType, appstore.EventSourceJira, entityID, "", reason)
	}
	// A ticket reaching a terminal status ("… → Done") needs no reaction — flagging
	// it only fills the Needs Attention panel with tiles the user must dismiss.
	if !appstore.IsTerminalTicketStatus(newValue) {
		if err := a.db.FlagNeedsAttention(ctx, contextID, notifyReason(entityType, eventType, oldValue, newValue)); err != nil {
			a.logger.Warn("jira notify: flag needs_attention failed",
				zap.String("contextID", contextID), zap.Error(err))
		}
	}
	return nil
}

// onVCSNotify records a detected PR change: de-duped notification, needs-attention
// flag, hook dispatch, and (on a newly-inserted notification) a timeline event.
func (a *App) onVCSNotify(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error {
	inserted, err := a.db.CreateNotification(ctx, contextID, entityType, entityID, eventType, oldValue, newValue, "")
	if err != nil {
		return err
	}
	if inserted {
		a.dispatchContextHooks(contextID, "pr_update")
		a.emitDomainEvent(contextID, eventType, appstore.EventSourceGitHub, entityID, "", notifyReason(entityType, eventType, oldValue, newValue))
	}
	if err := a.db.FlagNeedsAttention(ctx, contextID, notifyReason(entityType, eventType, oldValue, newValue)); err != nil {
		a.logger.Warn("vcs notify: flag needs_attention failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
	return nil
}

// onReviewNotify records a new PR review request: dispatches code-review hooks for
// any linked work contexts, and records a review_requested event on the repo's
// synthetic "<repo> — reviews" context (review requests belong to no work context).
func (a *App) onReviewNotify(ctx context.Context, repoID, repoName string, prNumber int, prTitle, prAuthor string) {
	if ctxIDs, err := a.db.ContextIDsForRepo(ctx, repoID); err == nil {
		for _, cid := range ctxIDs {
			a.dispatchContextHooks(cid, "code_review")
		}
	} else {
		a.logger.Warn("review notify: contexts for repo failed", zap.String("repoID", repoID), zap.Error(err))
	}
	reviewCtx, err := a.reviewContextForRepo(repoID, repoName)
	if err != nil {
		a.logger.Warn("review notify: review context failed", zap.String("repoID", repoID), zap.Error(err))
		return
	}
	summary := fmt.Sprintf("Review requested: PR #%d %s", prNumber, prTitle)
	a.emitDomainEvent(reviewCtx, appstore.EventTypeReviewRequested, appstore.EventSourceGitHub,
		fmt.Sprintf("%s#%d", repoName, prNumber), prAuthor, summary)
}

// reviewContextForRepo gets or creates the synthetic "PR Review" context for a
// repo, used to house review-request events for PRs that belong to no existing
// work context. The context key is "<repoName> — reviews" and is idempotent:
// repeated calls for the same repo return the same context ID.
func (a *App) reviewContextForRepo(repoID, repoName string) (string, error) {
	key := repoName + " — reviews"
	if c, err := a.db.GetContextByKey(a.ctx, key); err == nil && c != nil {
		return c.ID, nil
	}

	orgID := ""
	if ids, err := a.db.ContextIDsForRepo(a.ctx, repoID); err == nil && len(ids) > 0 {
		if c, err := a.db.GetContext(a.ctx, ids[0]); err == nil && c != nil {
			orgID = c.OrgID
		}
	}
	if orgID == "" {
		orgs, err := a.db.ListOrganizations(a.ctx)
		if err != nil {
			return "", err
		}
		if len(orgs) == 0 {
			return "", errors.New("no organization available for review context")
		}
		orgID = orgs[0].ID
	}

	c, err := a.db.CreateContext(a.ctx, key, orgID)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// emitDomainEvent appends a deterministic domain event to the timeline. It is
// best-effort: a failed event write logs and returns without disrupting the
// caller's sync, so the timeline can never block a refresh.
func (a *App) emitDomainEvent(contextID, eventType, sourceType, sourceID, author, summary string) {
	if contextID == "" || eventType == "" {
		return
	}
	if _, err := a.db.AddEvent(a.ctx, appstore.Event{
		ContextID:  contextID,
		Type:       eventType,
		SourceType: sourceType,
		SourceID:   sourceID,
		Author:     author,
		Summary:    summary,
	}); err != nil {
		a.logger.Warn("emit domain event failed",
			zap.String("contextID", contextID), zap.String("type", eventType), zap.Error(err))
	}
}

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
		runtime.EventsEmit(a.ctx, "refresh:progress", map[string]interface{}{
			"current": current,
			"total":   total,
			"step":    step,
		})
	})
}

// CancelRefresh cancels the currently running RefreshContext call, if any.
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
	return a.cliRunner.LoadSlackContext(a.ctx, cliclient.LoadSlackContextOpts{
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
	return a.cliRunner.ListSlackThreads(a.ctx, contextID)
}

// --- Slack channels ---

// ListSlackWorkspaces returns the workspaces available via slack-cli.
func (a *App) ListSlackWorkspaces() ([]jiraservice.SlackWorkspaceResult, error) {
	return a.cliRunner.ListSlackWorkspaces(a.ctx)
}

// SearchSlackChannels searches Slack channels by name pattern via slack-cli.
// namePattern supports glob-style wildcards, e.g. "*deploy*".
func (a *App) SearchSlackChannels(workspace, namePattern string) ([]jiraservice.SlackChannelResult, error) {
	return a.cliRunner.SearchSlackChannels(a.ctx, workspace, namePattern)
}

// AddSlackChannels adds one or more Slack channels to a context.
func (a *App) AddSlackChannels(contextID, workspace string, channels []slackstore.SlackChannelInput) error {
	ids := make([]string, len(channels))
	names := make([]string, len(channels))
	startFrom := ""
	for i, ch := range channels {
		ids[i] = ch.ChannelID
		names[i] = ch.ChannelName
		if ch.StartFrom != "" {
			startFrom = ch.StartFrom
		}
	}
	return a.cliRunner.AddSlackChannels(a.ctx, cliclient.AddSlackChannelsOpts{
		ContextID:    contextID,
		Workspace:    workspace,
		ChannelIDs:   ids,
		ChannelNames: names,
		StartFrom:    startFrom,
	})
}

// ListSlackChannels returns all Slack channels configured for a context.
func (a *App) ListSlackChannels(contextID string) ([]slackstore.SlackChannel, error) {
	return a.cliRunner.ListSlackChannels(a.ctx, contextID)
}

// RemoveSlackChannel removes a Slack channel from a context by its record ID.
func (a *App) RemoveSlackChannel(id string) error {
	return a.cliRunner.RemoveSlackChannel(a.ctx, id)
}

// --- Slack DM configs ---

// ListSlackDMConfigs returns all DM load configurations for a context.
func (a *App) ListSlackDMConfigs(contextID string) ([]slackstore.SlackDMConfig, error) {
	return a.cliRunner.ListSlackDMConfigs(a.ctx, contextID)
}

// UpsertSlackDMConfig creates or updates a DM load configuration and immediately loads DM content.
// startFrom is a date string in YYYY-MM-DD format; pass an empty string for no date filter.
func (a *App) UpsertSlackDMConfig(contextID, workspace, startFrom string) (*slackstore.SlackDMConfig, error) {
	return a.cliRunner.UpsertSlackDMConfig(a.ctx, contextID, workspace, startFrom)
}

// DeleteSlackDMConfig removes a DM load configuration by its record ID.
func (a *App) DeleteSlackDMConfig(id string) error {
	return a.cliRunner.DeleteSlackDMConfig(a.ctx, id)
}

// ToggleSlackDMConfigActive flips the is_active flag for a Slack DM config.
func (a *App) ToggleSlackDMConfigActive(id string) (*slackstore.SlackDMConfig, error) {
	return a.cliRunner.ToggleSlackDMConfigActive(a.ctx, id)
}

// RemoveSlackDMEntry removes a single DM conversation entry from a DM config's JSON content.
func (a *App) RemoveSlackDMEntry(dmConfigID, entryID string) error {
	return a.cliRunner.RemoveSlackDMEntry(a.ctx, dmConfigID, entryID)
}

// PreviewSlackDMs calls slack-cli list-dms for the given workspace and optional startFrom date
// without persisting anything. Returns the formatted text content, or an empty string when
// no personal messages are found. Use this to preview DMs before adding them to a context.
func (a *App) PreviewSlackDMs(workspace, startFrom string) (string, error) {
	return a.cliRunner.PreviewSlackDMs(a.ctx, workspace, startFrom)
}

// --- Context texts ---

// ListContextTexts returns all plain-text entries attached to a context.
func (a *App) ListContextTexts(contextID string) ([]appstore.ContextText, error) {
	return a.cliRunner.ListContextTexts(a.ctx, contextID)
}

// UpsertContextText creates a new text entry (when id is empty) or updates an existing one.
func (a *App) UpsertContextText(id, contextID, name, content string) (*appstore.ContextText, error) {
	return a.cliRunner.UpsertContextText(a.ctx, id, contextID, name, content)
}

// DeleteContextText removes a single text entry by its internal ID.
func (a *App) DeleteContextText(id string) error {
	return a.cliRunner.DeleteContextText(a.ctx, id)
}

// DeleteJiraIssue removes a single Jira issue and all its comments from a context.
func (a *App) DeleteJiraIssue(id string) error {
	return a.cliRunner.DeleteJiraIssue(a.ctx, id)
}

// DeleteJiraComment removes a single Jira comment by its internal ID.
func (a *App) DeleteJiraComment(id string) error {
	return a.cliRunner.DeleteJiraComment(a.ctx, id)
}

// DeleteSlackThread removes a single Slack thread by its internal ID.
func (a *App) DeleteSlackThread(id string) error {
	return a.cliRunner.DeleteSlackThread(a.ctx, id)
}

// ToggleIssueActive flips the is_active flag for a Jira issue.
func (a *App) ToggleIssueActive(id string) (*jirastore.Issue, error) {
	return a.cliRunner.ToggleIssueActive(a.ctx, id)
}

// ToggleCommentActive flips the is_active flag for a Jira comment.
func (a *App) ToggleCommentActive(id string) (*jirastore.Comment, error) {
	return a.cliRunner.ToggleCommentActive(a.ctx, id)
}

// ToggleSlackChannelActive flips the is_active flag for a Slack channel.
func (a *App) ToggleSlackChannelActive(id string) (*slackstore.SlackChannel, error) {
	return a.cliRunner.ToggleSlackChannelActive(a.ctx, id)
}

// ToggleSlackThreadActive flips the is_active flag for a Slack thread.
func (a *App) ToggleSlackThreadActive(id string) (*slackstore.SlackThread, error) {
	return a.cliRunner.ToggleSlackThreadActive(a.ctx, id)
}

// ToggleContextTextActive flips the is_active flag for a text entry.
func (a *App) ToggleContextTextActive(id string) (*appstore.ContextText, error) {
	return a.cliRunner.ToggleContextTextActive(a.ctx, id)
}

// --- Uploaded files (document text extraction) ---

// PickAndExtractFiles opens a native file picker, extracts text from each selected document
// (PDF, DOCX, RTF, ODT, TXT) or image (PNG, JPG, JPEG, WEBP via OCR), persists the result
// and returns the created records. Files that cannot be processed are logged and skipped.
// Images with no readable text are reported in result.Errors rather than silently dropped.
func (a *App) PickAndExtractFiles(contextID string) (*UploadBatchResult, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select documents or images to attach",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Documents & images (*.pdf;*.docx;*.rtf;*.odt;*.txt;*.png;*.jpg;*.jpeg;*.webp)",
				Pattern:     "*.pdf;*.docx;*.rtf;*.odt;*.txt;*.png;*.jpg;*.jpeg;*.webp",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening file dialog: %w", err)
	}
	if len(paths) == 0 {
		return &UploadBatchResult{}, nil
	}

	result := &UploadBatchResult{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			a.logger.Warn("uploaded file: stat failed", zap.String("path", p), zap.Error(err))
			continue
		}

		text, fileType, err := extract.FromFile(p)
		if err != nil {
			if userMsg := userVisibleExtractionError(err, filepath.Base(p)); userMsg != "" {
				result.Errors = append(result.Errors, userMsg)
			} else {
				a.logger.Warn("uploaded file: extraction failed", zap.String("path", p), zap.Error(err))
			}
			continue
		}

		fileBytes, err := os.ReadFile(p)
		if err != nil {
			a.logger.Warn("uploaded file: reading bytes failed", zap.String("path", p), zap.Error(err))
			continue
		}

		originalName := filepath.Base(p)
		f, err := a.svc.InsertUploadedFile(a.ctx, contextID, originalName, fileType, text, fileBytes, info.Size())
		if err != nil {
			a.logger.Warn("uploaded file: insert failed", zap.String("path", p), zap.Error(err))
			continue
		}
		result.Files = append(result.Files, *f)
	}
	return result, nil
}

// PickAndExtractFilesPreview opens a native file picker, extracts text from each
// selected document or image, and stores the result in memory keyed by a temporary ID.
// No database write happens — call CommitPendingUploads after the context is created.
// Images with no readable text are reported in result.Errors rather than silently dropped.
func (a *App) PickAndExtractFilesPreview() (*UploadPreviewBatchResult, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select documents or images to attach",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Documents & images (*.pdf;*.docx;*.rtf;*.odt;*.txt;*.png;*.jpg;*.jpeg;*.webp)",
				Pattern:     "*.pdf;*.docx;*.rtf;*.odt;*.txt;*.png;*.jpg;*.jpeg;*.webp",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening file dialog: %w", err)
	}
	if len(paths) == 0 {
		return &UploadPreviewBatchResult{}, nil
	}

	a.pendingMu.Lock()
	if a.pendingUploads == nil {
		a.pendingUploads = make(map[string]*PendingUpload)
	}
	a.pendingMu.Unlock()

	result := &UploadPreviewBatchResult{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			a.logger.Warn("preview file: stat failed", zap.String("path", p), zap.Error(err))
			continue
		}

		text, fileType, err := extract.FromFile(p)
		if err != nil {
			if userMsg := userVisibleExtractionError(err, filepath.Base(p)); userMsg != "" {
				result.Errors = append(result.Errors, userMsg)
			} else {
				a.logger.Warn("preview file: extraction failed", zap.String("path", p), zap.Error(err))
			}
			continue
		}

		fileBytes, err := os.ReadFile(p)
		if err != nil {
			a.logger.Warn("preview file: reading bytes failed", zap.String("path", p), zap.Error(err))
			continue
		}

		tempID := uuid.New().String()

		a.pendingMu.Lock()
		a.pendingUploads[tempID] = &PendingUpload{
			OriginalName:  filepath.Base(p),
			FileType:      fileType,
			ExtractedText: text,
			FileContent:   fileBytes,
			FileSize:      info.Size(),
		}
		a.pendingMu.Unlock()

		result.Previews = append(result.Previews, PendingUploadPreview{
			TempID:        tempID,
			OriginalName:  filepath.Base(p),
			FileType:      fileType,
			ExtractedText: text,
			FileSize:      info.Size(),
		})
	}
	return result, nil
}

// CommitPendingUploads persists previously picked files (from PickAndExtractFilesPreview)
// into the database for the given context ID, then clears them from memory.
func (a *App) CommitPendingUploads(contextID string, tempIDs []string) ([]appstore.ContextUploadedFile, error) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()

	var results []appstore.ContextUploadedFile
	for _, tid := range tempIDs {
		pending, ok := a.pendingUploads[tid]
		if !ok {
			a.logger.Warn("commit pending upload: temp ID not found", zap.String("tempID", tid))
			continue
		}
		f, err := a.svc.InsertUploadedFile(a.ctx, contextID, pending.OriginalName, pending.FileType, pending.ExtractedText, pending.FileContent, pending.FileSize)
		if err != nil {
			a.logger.Warn("commit pending upload: insert failed", zap.String("tempID", tid), zap.Error(err))
			continue
		}
		results = append(results, *f)
		delete(a.pendingUploads, tid)
	}
	return results, nil
}

// DiscardPendingUpload removes a single pending upload from memory (user removed it before saving).
func (a *App) DiscardPendingUpload(tempID string) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	delete(a.pendingUploads, tempID)
}

// ListUploadedFiles returns all uploaded files for a context (active and inactive).
func (a *App) ListUploadedFiles(contextID string) ([]appstore.ContextUploadedFile, error) {
	return a.cliRunner.ListUploadedFiles(a.ctx, contextID)
}

// DeleteUploadedFile removes a single uploaded file by its internal ID.
func (a *App) DeleteUploadedFile(id string) error {
	return a.cliRunner.DeleteUploadedFile(a.ctx, id)
}

// ToggleUploadedFileActive flips the is_active flag for an uploaded file.
func (a *App) ToggleUploadedFileActive(id string) (*appstore.ContextUploadedFile, error) {
	return a.cliRunner.ToggleUploadedFileActive(a.ctx, id)
}

// SearchJQL runs an arbitrary JQL query and returns matching issues for preview / validation.
// It does not require a saved context — results are returned directly to the caller.
func (a *App) SearchJQL(jql string) ([]jira.SearchResult, error) {
	return a.cliRunner.SearchJQL(a.ctx, jql)
}

// ReloadAllContexts fetches fresh data from every context's linked Jira query and Slack threads.
// Returns one result per context; per-context errors are embedded in the result, not returned as
// a top-level error, so the caller always receives the full picture.
// Progress events ("reload:progress") are emitted via Wails runtime before each provider step.
func (a *App) ReloadAllContexts() ([]jiraservice.ReloadAllResult, error) {
	return a.svc.ReloadAllContexts(a.ctx, func(current, total int, contextKey, step string) {
		runtime.EventsEmit(a.ctx, "reload:progress", map[string]interface{}{
			"current":    current,
			"total":      total,
			"contextKey": contextKey,
			"step":       step,
		})
	})
}

// --- AI settings ---

// AISettingsResponse is the shape returned to the frontend; raw API keys are never included.
type AISettingsResponse struct {
	Provider        string `json:"provider"`
	HasKey          bool   `json:"hasKey"`
	HasOpenAIKey    bool   `json:"hasOpenAIKey"`
	HasAnthropicKey bool   `json:"hasAnthropicKey"`
	Model           string `json:"model"`
	BaseURL         string `json:"baseUrl"`
	Source          string `json:"source"`
	UpdatedAt       string `json:"updatedAt"`
}

// GetAISettings returns the current AI configuration metadata (without the raw keys).
func (a *App) GetAISettings() (*AISettingsResponse, error) {
	result, err := a.cliRunner.GetAISettings(a.ctx)
	if err != nil {
		return nil, err
	}
	return &AISettingsResponse{
		Provider:        result.Provider,
		HasKey:          result.HasKey,
		HasOpenAIKey:    result.HasOpenAIKey,
		HasAnthropicKey: result.HasAnthropicKey,
		Model:           result.Model,
		BaseURL:         result.BaseURL,
		Source:          result.Source,
		UpdatedAt:       result.UpdatedAt,
	}, nil
}

// SetAISettings upserts the AI configuration for the given provider.
// Empty key strings preserve existing stored keys.
func (a *App) SetAISettings(provider, openaiKey, anthropicKey, model, baseURL string) error {
	_, err := a.cliRunner.SetAISettings(a.ctx, cliclient.SetAISettingsOpts{
		Provider:     provider,
		APIKey:       openaiKey,
		AnthropicKey: anthropicKey,
		Model:        model,
		BaseURL:      baseURL,
	})
	return err
}

// --- Quick Text Attach ---

// clipboardCopyWait is how long to wait for Cmd+C to reach the clipboard
// when using the CGEvent fallback path.
const clipboardCopyWait = 300 * time.Millisecond

// maxCapturedTextBytes is the maximum size of text forwarded to the frontend.
// Larger payloads are truncated to prevent UI freezes and excessive AI token use.
const maxCapturedTextBytes = 100 * 1024 // 100 KB

// a11yPollInterval is how often the accessibility poller checks for permission.
const a11yPollInterval = 2 * time.Second

// onHotkey is called by the HotkeyService when Cmd+Shift+C is pressed.
// Strategy:
//  1. Try AXUIElement — reads selected text directly without touching the clipboard.
//  2. Fall back to CGEvent Cmd+C for apps that don't expose AX (Mail, Slack, etc.).
//     Clipboard is read via NSPasteboard in CGo — no subprocess fork (which can
//     corrupt Carbon event loops) and no Wails main-thread dispatch (which can
//     steal focus from the target app before the simulated Cmd+C fires).
func (a *App) onHotkey() {
	// Drop concurrent invocations (rapid double-press).
	// The hotkeyInFlight guard serialises all access below, including the
	// separate pickerMu check-and-set, so the two-lock pattern is safe.
	if !a.hotkeyInFlight.CompareAndSwap(false, true) {
		return
	}
	defer a.hotkeyInFlight.Store(false)

	// Emit accessibility:needed on the first hotkey invocation when permission
	// is absent — deferred from domReady to avoid false-positives on freshly
	// rebuilt binaries whose trust has not yet been cached by the OS.
	if !hotkey.CheckAccessibility() {
		runtime.EventsEmit(a.ctx, "accessibility:needed")
		return
	}

	// Drop if picker is already waiting for a response.
	a.pickerMu.Lock()
	alreadyPending := a.pendingText != ""
	pendingLen := len(a.pendingText)
	a.pickerMu.Unlock()
	if alreadyPending {
		a.logger.Info("hotkey drop: picker already pending", zap.Int("pendingTextLen", pendingLen))
		return
	}

	// --- Primary path: AX API, no clipboard side-effects ---
	text := hotkey.GetSelectedText()

	if strings.TrimSpace(text) == "" {
		// --- Fallback: simulate Cmd+C and detect clipboard change ---
		before := hotkey.ReadClipboard()
		hotkey.SimulateCmdC()
		time.Sleep(clipboardCopyWait)
		after := hotkey.ReadClipboard()
		if after == before || strings.TrimSpace(after) == "" {
			a.logger.Info("hotkey: no text selected in source app")
			runtime.EventsEmit(a.ctx, "hotkey:no-selection")
			return
		}
		text = after
	}

	// Cap by byte length, then strip any trailing UTF-8 continuation bytes so we
	// never split a multi-byte rune. This correctly handles all of ASCII, CJK, emoji,
	// etc., and avoids the /4 over-truncation that would cap 100 KB of ASCII to ~25 KB.
	if len(text) > maxCapturedTextBytes {
		text = text[:maxCapturedTextBytes]
		// Continuation bytes are 0x80–0xBF (bit pattern 10xxxxxx). Walk backwards
		// until we land on an ASCII byte or a multi-byte lead byte.
		for len(text) > 0 && text[len(text)-1]&0xC0 == 0x80 {
			text = text[:len(text)-1]
		}
	}

	a.pickerMu.Lock()
	a.pendingText = text
	a.pickerMu.Unlock()

	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.EventsEmit(a.ctx, "hotkey:show-picker", text)
}

// GetPendingText returns the last text captured by the hotkey. Useful as a
// fallback if the frontend misses the event (e.g. during page navigation).
func (a *App) GetPendingText() string {
	a.pickerMu.Lock()
	defer a.pickerMu.Unlock()
	return a.pendingText
}

// QuickAttachText saves content to the given context as a new ContextText entry.
// The name is derived from the first 80 rune characters of the content.
// Clears always-on-top and the pending text on success.
func (a *App) QuickAttachText(contextID, content string) (*appstore.ContextText, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID is required")
	}
	// Validate UUID format before passing to the DB layer.
	if _, err := uuid.Parse(contextID); err != nil {
		return nil, fmt.Errorf("invalid contextID format")
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	// Hard IPC-boundary cap — prevents oversized payloads that bypass the
	// hotkey-side cap (e.g. via a direct frontend call).
	const maxContentBytes = 1024 * 1024 // 1 MB
	if len(content) > maxContentBytes {
		return nil, fmt.Errorf("content too large (max 1 MB)")
	}

	// Always clear always-on-top, even on error, so the window is not stuck on top.
	defer runtime.WindowSetAlwaysOnTop(a.ctx, false)

	name := content
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80]) + "…"
	}
	result, err := a.svc.UpsertContextText(a.ctx, "", contextID, name, content)
	if err != nil {
		// Log the full error server-side; return a generic message so internals
		// (DB connection strings, file paths, SQL details) are not exposed via IPC.
		a.logger.Error("quick attach: save failed", zap.Error(err), zap.String("contextID", contextID))
		return nil, fmt.Errorf("failed to save text")
	}
	a.pickerMu.Lock()
	a.pendingText = ""
	a.pickerMu.Unlock()
	return result, nil
}

// CheckAccessibility returns true when this process has the Accessibility
// permission needed to simulate keyboard input via System Events.
func (a *App) CheckAccessibility() bool {
	return hotkey.CheckAccessibility()
}

// OpenAccessibilityPreferences opens the macOS Privacy & Security → Accessibility
// pane so the user can grant permission for the Cmd+Shift+C hotkey feature.
// It also starts a background poller (at most one at a time) that emits
// "accessibility:granted" once the permission is detected so the frontend
// banner can auto-dismiss.
func (a *App) OpenAccessibilityPreferences() {
	if err := exec.Command("open",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Run(); err != nil {
		a.logger.Warn("failed to open Accessibility preferences pane", zap.Error(err))
	}
	// Only one poller at a time — guard against multiple button clicks.
	if !a.a11yPolling.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.a11yPolling.Store(false)
		for i := 0; i < 60; i++ { // poll for up to 2 minutes
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(a11yPollInterval):
			}
			if hotkey.CheckAccessibility() {
				// Guard against emitting after app shutdown.
				select {
				case <-a.ctx.Done():
					return
				default:
				}
				runtime.EventsEmit(a.ctx, "accessibility:granted")
				return
			}
		}
	}()
}

// DismissPicker clears the always-on-top state and pending text without saving.
// Called when the user cancels or completes the quick-attach overlay.
func (a *App) DismissPicker() {
	a.pickerMu.Lock()
	a.pendingText = ""
	a.pickerMu.Unlock()
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	// Return focus to the source app so the user can immediately select more
	// text and use Cmd+Shift+C again without having to manually switch apps.
	hotkey.DeactivateApp()
}

// --- Users & Author Resolution ---

// ListOrgUsers returns all users for an org (empty orgID = all orgs).
func (a *App) ListOrgUsers(orgID string) ([]appstore.User, error) {
	return a.db.ListUsers(a.ctx, orgID)
}

// CreateOrgUser creates a new org-level user.
func (a *App) CreateOrgUser(displayName, email, orgID string) (*appstore.User, error) {
	return a.db.CreateUser(a.ctx, appstore.CreateUserArgs{
		OrgID: orgID, DisplayName: displayName, Email: email,
	})
}

// GetUnresolvedAuthors returns unresolved (source, rawAuthor) pairs for a context.
func (a *App) GetUnresolvedAuthors(contextID string) ([]appstore.UnresolvedAuthor, error) {
	return a.db.GetUnresolvedAuthors(a.ctx, contextID)
}

// AssignAuthorBatch records author mappings and back-fills author_id on messages.
func (a *App) AssignAuthorBatch(assignments []appstore.AuthorAssignment) error {
	return a.db.AssignAuthorBatch(a.ctx, assignments)
}

// CreateUserAndAssign creates a new org user and maps (sourceID, rawAuthor) → that user.
func (a *App) CreateUserAndAssign(displayName, email, orgID, sourceID, rawAuthor string) (*appstore.User, error) {
	return a.db.CreateUserAndAssign(
		a.ctx,
		appstore.CreateUserArgs{OrgID: orgID, DisplayName: displayName, Email: email},
		sourceID, rawAuthor,
	)
}

// ListMessages returns all messages for a context ordered by created_at ASC.
func (a *App) ListMessages(contextID string) ([]appstore.Message, error) {
	return a.db.ListMessagesSince(a.ctx, contextID, time.Time{})
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

// AppLogEntry represents a single structured log entry from the database.
type AppLogEntry struct {
	ID        int64   `json:"id"`
	Level     string  `json:"level"`
	Ts        float64 `json:"ts"`
	Caller    string  `json:"caller"`
	Msg       string  `json:"msg"`
	Error     string  `json:"error,omitempty"`
	Fields    string  `json:"fields,omitempty"`
	CreatedAt string  `json:"createdAt"`
}

// GetAppLogs returns the last `limit` info-and-above entries from the database.
func (a *App) GetAppLogs(limit int) ([]AppLogEntry, error) {
	rows, err := a.appLogRepo.List(a.ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]AppLogEntry, len(rows))
	for i, r := range rows {
		result[i] = AppLogEntry{
			ID: r.ID, Level: r.Level, Ts: r.Ts,
			Caller: r.Caller, Msg: r.Msg, Error: r.Error, Fields: r.Fields, CreatedAt: r.CreatedAt,
		}
	}
	return result, nil
}

// ClearAppLogs deletes all app log entries from the database.
func (a *App) ClearAppLogs() error {
	return a.appLogRepo.Clear(a.ctx)
}

// GetMyBoard returns aggregated personal board data: blockers, assignments for personName,
// and open PRs authored by personName.
// personName is matched against person_assignments.name and vcs_pull_requests.author.
// Pass empty string to see all assignments.
func (a *App) GetMyBoard(personName string) (*appstore.MyBoardData, error) {
	blockers, err := a.db.ListBoardBlockers(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("board blockers: %w", err)
	}

	assignments, err := a.db.ListBoardAssignments(a.ctx, personName)
	if err != nil {
		return nil, fmt.Errorf("board assignments: %w", err)
	}

	prs, err := a.db.ListBoardPRsByAuthor(a.ctx, personName)
	if err != nil {
		return nil, fmt.Errorf("board prs: %w", err)
	}

	jiraIssues, err := a.db.ListBoardJiraAssignments(a.ctx, "priority")
	if err != nil {
		return nil, fmt.Errorf("board jira assignments: %w", err)
	}

	recent, err := a.db.ListBoardRecentEvents(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("board recent events: %w", err)
	}

	return &appstore.MyBoardData{
		Blockers:             blockers,
		MyAssignments:        assignments,
		MyOpenPRs:            prs,
		MyAssignedJiraIssues: jiraIssues,
		RecentActivity:       recent,
	}, nil
}

func logFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".vibecodeway")
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "app.log")
}
