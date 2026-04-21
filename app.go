package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/extract"
	"github.com/artschekoff/context-consolidator/internal/jira"
	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	openaiclient "github.com/artschekoff/context-consolidator/internal/openai"
	"github.com/artschekoff/context-consolidator/internal/slack"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
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

// App is the Wails application struct. All exported methods are bound to the frontend.
type App struct {
	ctx           context.Context
	svc           *jiraservice.ContextService
	contextLog    *contextlog.Service
	openAI        *openaiclient.Client
	db            *appstore.Store
	logger        *zap.Logger
	slackRunner   *slack.Runner
	cancelRefresh context.CancelFunc
	cancelMu      sync.Mutex

	pendingMu      sync.Mutex
	pendingUploads map[string]*PendingUpload
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
	textProvider := jiraservice.NewTextProvider(s, logger)

	a.svc = jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider, textProvider)

	openAI := openaiclient.NewClient(s, slackRunner.Passphrase, logger)
	a.openAI = openAI
	contextLogRepo := contextlog.NewRepo(s.DB())
	a.contextLog = contextlog.NewService(contextLogRepo, openAI.AsContextLogClient(), logger)
	a.svc.SetContextLog(a.contextLog)

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
func (a *App) SetSlackPassphrase(passphrase string) error {
	return a.slackRunner.SetPassphrase(passphrase)
}

// --- Context CRUD ---

// ListContexts returns active contexts only.
func (a *App) ListContexts() ([]appstore.Context, error) {
	return a.svc.ListContexts(a.ctx)
}

// ListAllContexts returns all contexts including archived ones.
func (a *App) ListAllContexts() ([]appstore.Context, error) {
	return a.svc.ListAllContexts(a.ctx)
}

// ToggleContextStatus flips a context between "active" and "archived".
func (a *App) ToggleContextStatus(id string) (*appstore.Context, error) {
	return a.svc.ToggleContextStatus(a.ctx, id)
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

// --- Slack DM configs ---

// ListSlackDMConfigs returns all DM load configurations for a context.
func (a *App) ListSlackDMConfigs(contextID string) ([]slackstore.SlackDMConfig, error) {
	return a.svc.ListSlackDMConfigs(a.ctx, contextID)
}

// UpsertSlackDMConfig creates or updates a DM load configuration and immediately loads DM content.
// startFrom is a date string in YYYY-MM-DD format; pass an empty string for no date filter.
func (a *App) UpsertSlackDMConfig(contextID, workspace, startFrom string) (*slackstore.SlackDMConfig, error) {
	return a.svc.UpsertSlackDMConfig(a.ctx, contextID, workspace, startFrom)
}

// DeleteSlackDMConfig removes a DM load configuration by its record ID.
func (a *App) DeleteSlackDMConfig(id string) error {
	return a.svc.DeleteSlackDMConfig(a.ctx, id)
}

// ToggleSlackDMConfigActive flips the is_active flag for a Slack DM config.
func (a *App) ToggleSlackDMConfigActive(id string) (*slackstore.SlackDMConfig, error) {
	return a.svc.ToggleSlackDMConfigActive(a.ctx, id)
}

// RemoveSlackDMEntry removes a single DM conversation entry from a DM config's JSON content.
func (a *App) RemoveSlackDMEntry(dmConfigID, entryID string) error {
	return a.svc.RemoveSlackDMEntry(a.ctx, dmConfigID, entryID)
}

// PreviewSlackDMs calls slack-cli list-dms for the given workspace and optional startFrom date
// without persisting anything. Returns the formatted text content, or an empty string when
// no personal messages are found. Use this to preview DMs before adding them to a context.
func (a *App) PreviewSlackDMs(workspace, startFrom string) (string, error) {
	return a.svc.PreviewSlackDMs(a.ctx, workspace, startFrom)
}

// --- Context texts ---

// ListContextTexts returns all plain-text entries attached to a context.
func (a *App) ListContextTexts(contextID string) ([]appstore.ContextText, error) {
	return a.svc.ListContextTexts(a.ctx, contextID)
}

// UpsertContextText creates a new text entry (when id is empty) or updates an existing one.
func (a *App) UpsertContextText(id, contextID, name, content string) (*appstore.ContextText, error) {
	return a.svc.UpsertContextText(a.ctx, id, contextID, name, content)
}

// DeleteContextText removes a single text entry by its internal ID.
func (a *App) DeleteContextText(id string) error {
	return a.svc.DeleteContextText(a.ctx, id)
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

// ToggleIssueActive flips the is_active flag for a Jira issue.
func (a *App) ToggleIssueActive(id string) (*jirastore.Issue, error) {
	return a.svc.ToggleIssueActive(a.ctx, id)
}

// ToggleCommentActive flips the is_active flag for a Jira comment.
func (a *App) ToggleCommentActive(id string) (*jirastore.Comment, error) {
	return a.svc.ToggleCommentActive(a.ctx, id)
}

// ToggleSlackChannelActive flips the is_active flag for a Slack channel.
func (a *App) ToggleSlackChannelActive(id string) (*slackstore.SlackChannel, error) {
	return a.svc.ToggleSlackChannelActive(a.ctx, id)
}

// ToggleSlackThreadActive flips the is_active flag for a Slack thread.
func (a *App) ToggleSlackThreadActive(id string) (*slackstore.SlackThread, error) {
	return a.svc.ToggleSlackThreadActive(a.ctx, id)
}

// ToggleContextTextActive flips the is_active flag for a text entry.
func (a *App) ToggleContextTextActive(id string) (*appstore.ContextText, error) {
	return a.svc.ToggleContextTextActive(a.ctx, id)
}

// --- Uploaded files (document text extraction) ---

// PickAndExtractFiles opens a native file picker, extracts text from each selected document
// (PDF, DOCX, RTF, ODT, TXT), persists the result and returns the created records.
// Files that cannot be processed are logged and skipped rather than causing the call to fail.
func (a *App) PickAndExtractFiles(contextID string) ([]appstore.ContextUploadedFile, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select documents to attach",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Supported documents (*.pdf;*.docx;*.rtf;*.odt;*.txt)",
				Pattern:     "*.pdf;*.docx;*.rtf;*.odt;*.txt",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening file dialog: %w", err)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	var results []appstore.ContextUploadedFile
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			a.logger.Warn("uploaded file: stat failed", zap.String("path", p), zap.Error(err))
			continue
		}

		text, fileType, err := extract.FromFile(p)
		if err != nil {
			a.logger.Warn("uploaded file: extraction failed", zap.String("path", p), zap.Error(err))
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
		results = append(results, *f)
	}
	return results, nil
}

// PickAndExtractFilesPreview opens a native file picker, extracts text from each
// selected document, and stores the result in memory keyed by a temporary ID.
// No database write happens — call CommitPendingUploads after the context is created.
func (a *App) PickAndExtractFilesPreview() ([]PendingUploadPreview, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select documents to attach",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Supported documents (*.pdf;*.docx;*.rtf;*.odt;*.txt)",
				Pattern:     "*.pdf;*.docx;*.rtf;*.odt;*.txt",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening file dialog: %w", err)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	a.pendingMu.Lock()
	if a.pendingUploads == nil {
		a.pendingUploads = make(map[string]*PendingUpload)
	}
	a.pendingMu.Unlock()

	var previews []PendingUploadPreview
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			a.logger.Warn("preview file: stat failed", zap.String("path", p), zap.Error(err))
			continue
		}

		text, fileType, err := extract.FromFile(p)
		if err != nil {
			a.logger.Warn("preview file: extraction failed", zap.String("path", p), zap.Error(err))
			continue
		}

		fileBytes, err := os.ReadFile(p)
		if err != nil {
			a.logger.Warn("preview file: reading bytes failed", zap.String("path", p), zap.Error(err))
			continue
		}

		a.pendingMu.Lock()
		tempID := fmt.Sprintf("pending-%d-%s", len(a.pendingUploads), filepath.Base(p))
		a.pendingMu.Unlock()

		a.pendingMu.Lock()
		a.pendingUploads[tempID] = &PendingUpload{
			OriginalName:  filepath.Base(p),
			FileType:      fileType,
			ExtractedText: text,
			FileContent:   fileBytes,
			FileSize:      info.Size(),
		}
		a.pendingMu.Unlock()

		previews = append(previews, PendingUploadPreview{
			TempID:        tempID,
			OriginalName:  filepath.Base(p),
			FileType:      fileType,
			ExtractedText: text,
			FileSize:      info.Size(),
		})
	}
	return previews, nil
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
	return a.svc.ListUploadedFiles(a.ctx, contextID)
}

// DeleteUploadedFile removes a single uploaded file by its internal ID.
func (a *App) DeleteUploadedFile(id string) error {
	return a.svc.DeleteUploadedFile(a.ctx, id)
}

// ToggleUploadedFileActive flips the is_active flag for an uploaded file.
func (a *App) ToggleUploadedFileActive(id string) (*appstore.ContextUploadedFile, error) {
	return a.svc.ToggleUploadedFileActive(a.ctx, id)
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

// --- Context logs ---

// ListContextLogs returns all change-log entries for a context, newest first.
func (a *App) ListContextLogs(contextID string) ([]contextlog.ContextLog, error) {
	return a.contextLog.ListLogs(a.ctx, contextID)
}

// CreateContextLog adds a manual (user-authored) log entry and refreshes the consolidated log.
func (a *App) CreateContextLog(contextID, summaryMd string) (*contextlog.ContextLog, error) {
	return a.contextLog.CreateManualLog(a.ctx, contextID, summaryMd)
}

// UpdateContextLog edits the summary of an existing log entry and refreshes the consolidated log.
func (a *App) UpdateContextLog(id, summaryMd string) (*contextlog.ContextLog, error) {
	return a.contextLog.UpdateLog(a.ctx, id, summaryMd)
}

// DeleteContextLog removes a single log entry and refreshes the consolidated log.
func (a *App) DeleteContextLog(id string) error {
	return a.contextLog.DeleteLog(a.ctx, id)
}

// --- Consolidated log ---

// GetConsolidatedContextLog returns the consolidated log for a context (may be nil).
func (a *App) GetConsolidatedContextLog(contextID string) (*contextlog.ConsolidatedLog, error) {
	return a.contextLog.GetConsolidated(a.ctx, contextID)
}

// UpdateConsolidatedContextLog overwrites the consolidated log with user-provided markdown.
func (a *App) UpdateConsolidatedContextLog(contextID, contentMd string) (*contextlog.ConsolidatedLog, error) {
	return a.contextLog.UpdateConsolidatedManual(a.ctx, contextID, contentMd)
}

// InitializeConsolidatedLog performs the AI-driven bootstrap of a fresh consolidated log.
// Uses the current snapshot of all context objects as the source of truth.
func (a *App) InitializeConsolidatedLog(contextID string) (*contextlog.ConsolidatedLog, error) {
	snap, err := a.contextLog.Repo().SnapshotFull(a.ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("snapshotting context: %w", err)
	}
	return a.contextLog.Initialize(a.ctx, contextID, snap)
}

// AnalyzeContextChanges triggers an on-demand change-log analysis using the current snapshot
// as "after" and an empty snapshot as "before" (i.e. treats everything as new).
func (a *App) AnalyzeContextChanges(contextID string) (*contextlog.ContextLog, error) {
	snap, err := a.contextLog.Repo().SnapshotFull(a.ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("snapshotting context: %w", err)
	}
	return a.contextLog.AnalyzeNow(a.ctx, contextID, snap)
}

// --- Chat ---

// ListContextChat returns the full chat transcript for a context (oldest first).
func (a *App) ListContextChat(contextID string) ([]contextlog.ChatMessage, error) {
	return a.contextLog.ListChat(a.ctx, contextID)
}

// ChatWithContext appends a user message and returns the AI assistant reply.
func (a *App) ChatWithContext(contextID, userMessage string) (*contextlog.ChatMessage, error) {
	return a.contextLog.ChatSend(a.ctx, contextID, userMessage)
}

// ApplyChatSuggestion applies an assistant-proposed consolidated rewrite to the consolidated log.
func (a *App) ApplyChatSuggestion(messageID string) (*contextlog.ConsolidatedLog, error) {
	return a.contextLog.ApplyChatSuggestion(a.ctx, messageID)
}

// --- AI settings ---

// AISettingsResponse is the shape returned to the frontend; the raw API key is never included.
type AISettingsResponse struct {
	HasKey    bool   `json:"hasKey"`
	Model     string `json:"model"`
	BaseURL   string `json:"baseUrl"`
	Source    string `json:"source"`
	UpdatedAt string `json:"updatedAt"`
}

// GetAISettings returns the current AI configuration metadata (without the raw key).
func (a *App) GetAISettings() (*AISettingsResponse, error) {
	settings, src, err := a.openAI.DescribeSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	return &AISettingsResponse{
		HasKey:    settings.HasKey,
		Model:     settings.Model,
		BaseURL:   settings.BaseURL,
		Source:    string(src),
		UpdatedAt: settings.UpdatedAt,
	}, nil
}

// SetAISettings upserts the AI configuration. When apiKey is non-empty it is encrypted
// with the current Slack passphrase; passing an empty apiKey preserves the existing key
// and only updates model / baseURL.
func (a *App) SetAISettings(apiKey, model, baseURL string) error {
	passphrase := a.slackRunner.Passphrase()
	if apiKey != "" && passphrase == "" {
		return fmt.Errorf("set Slack passphrase first (used to encrypt the API key)")
	}
	return a.db.SetAISettings(a.ctx, apiKey, model, baseURL, passphrase)
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
