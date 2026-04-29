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

	cliclient "github.com/artschekoff/context-consolidator/internal/cli"
	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/extract"
	"github.com/artschekoff/context-consolidator/internal/hotkey"
	"github.com/google/uuid"
	"github.com/artschekoff/context-consolidator/internal/jira"
	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	openaiclient "github.com/artschekoff/context-consolidator/internal/openai"
	"github.com/artschekoff/context-consolidator/internal/slack"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	githubrunner "github.com/artschekoff/context-consolidator/internal/vcs/github"
	vcsservice "github.com/artschekoff/context-consolidator/internal/vcs/service"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
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
	Errors []string                        `json:"errors"`
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
	contextLog    *contextlog.Service
	openAI        *openaiclient.Client
	db            *appstore.Store
	logger        *zap.Logger
	slackRunner   *slack.Runner
	cliRunner     *cliclient.Runner
	vcsStore      *vcsstore.Store
	vcsSvc        *vcsservice.VCSProvider
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
	a.cliRunner = cliclient.NewRunner(logger, cliclient.WithPassphraseFunc(slackRunner.Passphrase))

	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())

	jiraProvider := jiraservice.NewJiraProvider(jiraRepo, jiraClient, logger)
	slackProvider := jiraservice.NewSlackProvider(slackRepo, slackRunner, logger)
	textProvider := jiraservice.NewTextProvider(s, logger)

	ghRunner := githubrunner.NewRunner(logger)
	a.vcsStore = vcsstore.NewStore(s.DB())
	a.vcsSvc = vcsservice.NewVCSProvider(a.vcsStore, logger, ghRunner)
	a.vcsSvc.WithNotifyFunc(func(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error {
		return s.CreateNotification(ctx, contextID, entityType, entityID, eventType, oldValue, newValue)
	})
	vcsProvider := a.vcsSvc

	a.svc = jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider, textProvider, vcsProvider)
	a.svc.SetVCSStore(a.vcsStore)

	openAI := openaiclient.NewClient(s, slackRunner.Passphrase, logger)
	a.openAI = openAI
	contextLogRepo := contextlog.NewRepo(s.DB())
	a.contextLog = contextlog.NewService(contextLogRepo, openAI.AsContextLogClient(), logger)
	a.svc.SetContextLog(a.contextLog)

	a.hotkeyService = hotkey.New(a.onHotkey)
	if err := a.hotkeyService.Start(); err != nil {
		logger.Warn("failed to register global hotkey Cmd+Shift+C", zap.Error(err))
	} else {
		logger.Info("global hotkey Cmd+Shift+C registered")
	}

	a.startBackgroundRefresh()

	logger.Info("context-consolidator desktop started")
}

// domReady is called by Wails once the frontend DOM is loaded and event listeners
// are active.
func (a *App) domReady(_ context.Context) {}

// shutdown is called by Wails when the app is closing.
func (a *App) shutdown(_ context.Context) {
	a.stopBackgroundRefresh()
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

// SetSlackPassphrase stores the SLACK_MCP_PASSPHRASE value that will be injected
// as an environment variable into every subsequent slack-cli invocation.
func (a *App) SetSlackPassphrase(passphrase string) error {
	return a.slackRunner.SetPassphrase(passphrase)
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

// SearchContextIDs returns IDs of contexts matching the query across key, Jira summaries,
// Slack channel names, VCS branches, text file names, and uploaded file names.
func (a *App) SearchContextIDs(query string) ([]string, error) {
	return a.db.SearchContextIDs(a.ctx, query)
}

// ToggleContextStatus flips a context between "active" and "archived".
func (a *App) ToggleContextStatus(id string) (*appstore.Context, error) {
	return a.cliRunner.ToggleContextStatus(a.ctx, id)
}

// SetContextsOrder persists the display order of contexts.
// ids must contain the full ordered list of context IDs from top to bottom.
func (a *App) SetContextsOrder(ids []string) error {
	return a.db.SetContextsOrder(a.ctx, ids)
}

// CreateContext creates a new context.
func (a *App) CreateContext(key string) (*appstore.Context, error) {
	return a.cliRunner.CreateContext(a.ctx, key)
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
	// Auto-add GitHub repos detected in issue descriptions (best-effort)
	repos := a.svc.ExtractGitHubReposFromIssues(a.ctx, contextID, jiraKeys)
	for _, ownerRepo := range repos {
		if _, addErr := a.AddVCSBranch(contextID, "github", ownerRepo, "", ""); addErr != nil {
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

// --- Context logs ---

// ListContextLogs returns all change-log entries for a context, newest first.
func (a *App) ListContextLogs(contextID string) ([]contextlog.ContextLog, error) {
	return a.cliRunner.ListContextLogs(a.ctx, contextID)
}

// CreateContextLog adds a manual (user-authored) log entry and refreshes the consolidated log.
func (a *App) CreateContextLog(contextID, summaryMd string) (*contextlog.ContextLog, error) {
	return a.cliRunner.CreateContextLog(a.ctx, contextID, summaryMd)
}

// UpdateContextLog edits the summary of an existing log entry and refreshes the consolidated log.
func (a *App) UpdateContextLog(id, summaryMd string) (*contextlog.ContextLog, error) {
	return a.cliRunner.UpdateContextLog(a.ctx, id, summaryMd)
}

// DeleteContextLog removes a single log entry and refreshes the consolidated log.
func (a *App) DeleteContextLog(id string) error {
	return a.cliRunner.DeleteContextLog(a.ctx, id)
}

// --- Consolidated log ---

// GetConsolidatedContextLog returns the consolidated log for a context (may be nil).
func (a *App) GetConsolidatedContextLog(contextID string) (*contextlog.ConsolidatedLog, error) {
	return a.cliRunner.GetConsolidatedContextLog(a.ctx, contextID)
}

// UpdateConsolidatedContextLog overwrites the consolidated log with user-provided markdown.
func (a *App) UpdateConsolidatedContextLog(contextID, contentMd string) (*contextlog.ConsolidatedLog, error) {
	return a.cliRunner.UpdateConsolidatedContextLog(a.ctx, contextID, contentMd)
}

// InitializeConsolidatedLog performs the AI-driven bootstrap of a fresh consolidated log.
func (a *App) InitializeConsolidatedLog(contextID string) (*contextlog.ConsolidatedLog, error) {
	return a.cliRunner.InitializeConsolidatedLog(a.ctx, contextID)
}

// AnalyzeContextChanges triggers an on-demand change-log analysis.
func (a *App) AnalyzeContextChanges(contextID string) (*contextlog.ContextLog, error) {
	return a.cliRunner.AnalyzeContextChanges(a.ctx, contextID)
}

// --- Chat ---

// ListContextChat returns the full chat transcript for a context (oldest first).
func (a *App) ListContextChat(contextID string) ([]contextlog.ChatMessage, error) {
	return a.cliRunner.ListContextChat(a.ctx, contextID)
}

// ChatWithContext appends a user message and returns the AI assistant reply.
func (a *App) ChatWithContext(contextID, userMessage string) (*contextlog.ChatMessage, error) {
	return a.cliRunner.ChatWithContext(a.ctx, contextID, userMessage)
}

// ApplyChatSuggestion applies an assistant-proposed consolidated rewrite to the consolidated log.
func (a *App) ApplyChatSuggestion(messageID string) (*contextlog.ConsolidatedLog, error) {
	return a.cliRunner.ApplyChatSuggestion(a.ctx, messageID)
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
	result, err := a.cliRunner.GetAISettings(a.ctx)
	if err != nil {
		return nil, err
	}
	return &AISettingsResponse{
		HasKey:    result.HasKey,
		Model:     result.Model,
		BaseURL:   result.BaseURL,
		Source:    result.Source,
		UpdatedAt: result.UpdatedAt,
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
	_, err := a.cliRunner.SetAISettings(a.ctx, cliclient.SetAISettingsOpts{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
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
	a.pickerMu.Unlock()
	if alreadyPending {
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
// Called when the user cancels the quick-attach overlay.
func (a *App) DismissPicker() {
	a.pickerMu.Lock()
	a.pendingText = ""
	a.pickerMu.Unlock()
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
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
