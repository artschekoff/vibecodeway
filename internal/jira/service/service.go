// Package service contains business logic shared by both the Wails GUI and MCP server.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/jira"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/openquestions"
	"github.com/artschekoff/context-consolidator/internal/provider"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/artschekoff/context-consolidator/internal/workflow"
	"go.uber.org/zap"
)

// commentLister is the subset of the Jira client used by syncIssueComments.
type commentLister interface {
	ListComments(ctx context.Context, issueKey string) ([]jira.Comment, error)
}

// JiraAPI is the Jira client interface used by ContextService and JiraProvider.
type JiraAPI interface {
	commentLister
	ViewIssue(ctx context.Context, key string) (*jira.IssueDetail, error)
	Search(ctx context.Context, jql string) ([]jira.SearchResult, error)
	AuthStatus(ctx context.Context) (string, error)
	GetInstanceURL(ctx context.Context) string
}

const (
	// providerJira and providerSlack are the canonical provider name strings.
	providerJira  = "jira"
	providerSlack = "slack"
)

// jiraLinkToRelationType maps a Jira issue link type name and direction to a context relation type.
func jiraLinkToRelationType(typeName, direction string) string {
	name := strings.ToLower(typeName)
	switch {
	case strings.Contains(name, "block"):
		if direction == "outward" {
			return "blocks"
		}
		return "is-blocked-by"
	case strings.Contains(name, "duplic"):
		if direction == "outward" {
			return "duplicates"
		}
		return "is-duplicated-by"
	default:
		return "relates-to"
	}
}

// slackDMEntry is one DM conversation returned by slack-cli list-dms --with-messages.
// The CLI may return the message as "lastMessage" (current), "message" (singular), or
// "messages" (array). All three are decoded for forward/backward compatibility.
type slackDMEntry struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	UserID      string                    `json:"userId"`
	UserName    string                    `json:"userName"`
	IsIM        bool                      `json:"isIm"`
	Messages    []slackstore.SlackMessage `json:"messages"`
	Message     *slackstore.SlackMessage  `json:"message"`
	LastMessage *slackstore.SlackMessage  `json:"lastMessage"`
}

// allMessages returns all messages for this DM entry across all possible field shapes.
func (e slackDMEntry) allMessages() []slackstore.SlackMessage {
	if len(e.Messages) > 0 {
		return e.Messages
	}
	if e.LastMessage != nil {
		return []slackstore.SlackMessage{*e.LastMessage}
	}
	if e.Message != nil {
		return []slackstore.SlackMessage{*e.Message}
	}
	return nil
}

// SlackChannelResult is one channel returned by SearchSlackChannels.
// The JSON shape matches what slack-cli search-channels emits:
// [{id, name, messages:[{user, text, timestamp, rawTs}]}]
// Content is derived by formatting Messages for the search-result preview in ContextForm;
// it is NOT stored to the DB — AddSlackChannels uses Messages directly.
type SlackChannelResult struct {
	ID       string                    `json:"id"`
	Name     string                    `json:"name"`
	Messages []slackstore.SlackMessage `json:"messages"`
	Content  string                    `json:"content,omitempty"` // formatted preview; not in CLI output but sent to frontend
}

// SlackWorkspaceResult is one workspace returned by ListSlackWorkspaces.
// The JSON shape matches what slack-cli list-workspaces emits.
type SlackWorkspaceResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SlackRunner is the interface for executing slack-cli subcommands.
// Implemented by slack.Runner in production and by test doubles in tests.
type SlackRunner interface {
	Run(ctx context.Context, subCmdPath []string, flags []string) (string, error)
}

// LoadSlackContextArgs holds all parameters for LoadSlackContext.
type LoadSlackContextArgs struct {
	ContextID   string
	Workspace   string
	ChannelID   string
	ThreadTS    string
	ChannelName string
	Permalink   string
	SearchQuery string
}

// SearchDiff contains the result of running a context's saved JQL and
// comparing it against the issues already stored for that context.
type SearchDiff struct {
	ContextID string              `json:"contextId"`
	New       []jira.SearchResult `json:"new"`
	Existing  []jira.SearchResult `json:"existing"`
}

// RefreshResult summarises what was updated during a RefreshContext run.
type RefreshResult struct {
	IssuesRefreshed  int `json:"issuesRefreshed"`
	NewComments      int `json:"newComments"`
	ThreadsRefreshed int `json:"threadsRefreshed"`
	DMsRefreshed     int `json:"dmsRefreshed"`
	PRsRefreshed     int `json:"prsRefreshed"`
	NewPRComments    int `json:"newPRComments"`
	NewNotifications int `json:"newNotifications"`
}

// ReloadAllResult summarises what was fetched for one context during a ReloadAllContexts run.
type ReloadAllResult struct {
	ContextID        string `json:"contextId"`
	ContextKey       string `json:"contextKey"`
	NewIssues        int    `json:"newIssues"`
	NewComments      int    `json:"newComments"`
	ThreadsRefreshed int    `json:"threadsRefreshed"`
	DMsRefreshed     int    `json:"dmsRefreshed"`
	PRsRefreshed     int    `json:"prsRefreshed"`
	NewPRComments    int    `json:"newPRComments"`
	NewNotifications int    `json:"newNotifications"`
	Error            string `json:"error,omitempty"`
}

// RefreshProgressFunc is called before each provider refresh step inside RefreshContext.
// current is 1-based; total is the number of providers.
type RefreshProgressFunc func(current, total int, step string)

// ReloadProgressFunc is called before each provider step inside ReloadAllContexts.
// current is 1-based, total is the number of active contexts being reloaded.
// contextKey is the human-readable key for the context, step is the provider name
// (e.g. "jira", "slack", "vcs") so the caller can display fine-grained progress.
type ReloadProgressFunc func(current, total int, contextKey, step string)

// ContextDetail is a Context with its provider configs, Jira issues, Slack threads, texts, and VCS branches/PRs.
type ContextDetail struct {
	Context        appstore.Context               `json:"context"`
	Jira           *jirastore.JiraProvider        `json:"jira,omitempty"`
	Slack          *slackstore.SlackProvider      `json:"slack,omitempty"`
	Issues         []jirastore.Issue              `json:"issues"`
	SlackThreads   []slackstore.SlackThread       `json:"slackThreads,omitempty"`
	SlackChannels  []slackstore.SlackChannel      `json:"slackChannels,omitempty"`
	SlackDMConfigs []slackstore.SlackDMConfig     `json:"slackDMConfigs,omitempty"`
	ContextTexts   []appstore.ContextText         `json:"contextTexts,omitempty"`
	UploadedFiles  []appstore.ContextUploadedFile `json:"uploadedFiles,omitempty"`
	VCSBranches    []vcsstore.ContextVCSBranch    `json:"vcsBranches,omitempty"`
	PullRequests   []vcsstore.PullRequest         `json:"pullRequests,omitempty"`
	Relations      []appstore.ContextRelation     `json:"relations,omitempty"`
}

// ContextService provides all domain operations for managing contexts.
type ContextService struct {
	store         *appstore.Store
	jiraRepo      *jirastore.Repo
	slackRepo     *slackstore.Repo
	vcsStore      *vcsstore.Store // optional; nil disables VCS data in GetContextDetail
	jira          JiraAPI
	slack         SlackRunner
	logger        *zap.Logger
	providers     []provider.Provider
	contextLog    *contextlog.Service
	openQuestions OpenQuestionsRunner
	nextStepAI    workflow.NextStepAIClient // may be nil
}

// SetContextLog attaches a contextlog.Service for refresh/reload change recording.
// Passing nil disables logging hooks. Safe to call after construction.
func (svc *ContextService) SetContextLog(cl *contextlog.Service) {
	svc.contextLog = cl
}

// ContextLog returns the currently attached contextlog.Service, or nil if none.
func (svc *ContextService) ContextLog() *contextlog.Service {
	return svc.contextLog
}

// OpenQuestionsRunner is the interface used by ContextService to run the open-questions agent.
type OpenQuestionsRunner interface {
	AnalyzeContext(ctx context.Context, contextID, contentMD string, repos []openquestions.RepoInfo) error
	ReevaluateOpenQuestions(ctx context.Context, contextID, contentMD string) error
}

// SetOpenQuestionsService attaches the open-questions agent. Passing nil disables it.
func (svc *ContextService) SetOpenQuestionsService(oqs OpenQuestionsRunner) { svc.openQuestions = oqs }

// SetNextStepAI attaches the AI client used for next-step generation.
// Passing nil falls back to template-based generation.
func (svc *ContextService) SetNextStepAI(ai workflow.NextStepAIClient) { svc.nextStepAI = ai }

// SetAgentProposal persists an AI-agent-provided workflow proposal for contextID.
// status should be one of the valid workflow statuses (validated by the caller).
// comment and nextStep are optional; pass empty string to omit.
// This is the service-layer entry point for MCP context_set_status calls.
func (svc *ContextService) SetAgentProposal(ctx context.Context, contextID, status, comment, nextStep string) (*appstore.WorkflowProposal, error) {
	return svc.store.SetAIProposal(ctx, contextID, status, comment, nextStep, "")
}

// IsValidWorkflowStatus reports whether slug is a registered workflow status in the DB.
func (svc *ContextService) IsValidWorkflowStatus(ctx context.Context, slug string) (bool, error) {
	return svc.store.IsValidWorkflowStatus(ctx, slug)
}

// NewContextService creates a new ContextService.
// Pass provider implementations as variadic arguments to enable cross-cutting operations
// (Refresh, Reload, FormatContext). Each provider must have a unique Name().
// slack may be nil if Slack support is not needed.
func NewContextService(s *appstore.Store, j JiraAPI, jiraRepo *jirastore.Repo, slackRepo *slackstore.Repo, slack SlackRunner, logger *zap.Logger, providers ...provider.Provider) *ContextService {
	return &ContextService{
		store:     s,
		jiraRepo:  jiraRepo,
		slackRepo: slackRepo,
		jira:      j,
		slack:     slack,
		logger:    logger,
		providers: providers,
	}
}

// SetVCSStore attaches a vcsstore.Store so that GetContextDetail includes VCS data.
// Pass nil to disable VCS data (e.g. when running without VCS support).
func (svc *ContextService) SetVCSStore(vs *vcsstore.Store) {
	svc.vcsStore = vs
}

// AuthStatus returns the current acli authentication status.
func (svc *ContextService) AuthStatus(ctx context.Context) (string, error) {
	return svc.jira.AuthStatus(ctx)
}

// GetJiraInstanceURL returns the Jira instance base URL from acli auth status.
// Returns empty string when not authenticated or URL cannot be determined.
func (svc *ContextService) GetJiraInstanceURL(ctx context.Context) string {
	return svc.jira.GetInstanceURL(ctx)
}

// ListContexts returns active contexts only.
func (svc *ContextService) ListContexts(ctx context.Context) ([]appstore.Context, error) {
	return svc.store.ListContexts(ctx, false)
}

// ListAllContexts returns all contexts including archived ones.
func (svc *ContextService) ListAllContexts(ctx context.Context) ([]appstore.Context, error) {
	return svc.store.ListContexts(ctx, true)
}

// ToggleContextStatus flips the status between "active" and "archived".
func (svc *ContextService) ToggleContextStatus(ctx context.Context, id string) (*appstore.Context, error) {
	return svc.store.ToggleContextStatus(ctx, id)
}

// CreateContext creates a new context with the given key inside the specified organization.
func (svc *ContextService) CreateContext(ctx context.Context, key, orgID string) (*appstore.Context, error) {
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	return svc.store.CreateContext(ctx, key, orgID)
}

// UpdateContext updates mutable fields of a context.
func (svc *ContextService) UpdateContext(ctx context.Context, id, key, status string) (*appstore.Context, error) {
	return svc.store.UpdateContext(ctx, id, key, status)
}

// DeleteContext removes a context and all its data.
func (svc *ContextService) DeleteContext(ctx context.Context, id string) error {
	return svc.store.DeleteContext(ctx, id)
}

// SetJiraConfig creates or updates the Jira provider config for a context.
func (svc *ContextService) SetJiraConfig(ctx context.Context, contextID, searchQuery string) (*jirastore.JiraProvider, error) {
	if searchQuery == "" {
		return nil, fmt.Errorf("jira search query is required")
	}
	return svc.jiraRepo.UpsertJiraProvider(ctx, contextID, searchQuery)
}

// SetSlackConfig creates or updates the Slack provider config for a context.
func (svc *ContextService) SetSlackConfig(ctx context.Context, contextID, workspace, searchQuery string) (*slackstore.SlackProvider, error) {
	if workspace == "" {
		return nil, fmt.Errorf("slack workspace is required")
	}
	return svc.slackRepo.UpsertSlackProvider(ctx, contextID, workspace, searchQuery)
}

// GetContextDetail returns a context with provider configs, Jira issues, and Slack threads.
func (svc *ContextService) GetContextDetail(ctx context.Context, id string) (*ContextDetail, error) {
	c, err := svc.store.GetContext(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading context: %w", err)
	}
	detail := &ContextDetail{Context: *c}

	// Phase 1: fire all independent queries concurrently.
	var (
		mu   sync.Mutex
		errs []error

		jp        *jirastore.JiraProvider
		sp        *slackstore.SlackProvider
		issues    []jirastore.Issue
		threads   []slackstore.SlackThread
		channels  []slackstore.SlackChannel
		dmConfigs []slackstore.SlackDMConfig
		ctxTexts  []appstore.ContextText
		uploaded  []appstore.ContextUploadedFile
		branches  []vcsstore.ContextVCSBranch
		prs       []vcsstore.PullRequest
		relations []appstore.ContextRelation
	)

	addErr := func(e error) {
		mu.Lock()
		errs = append(errs, e)
		mu.Unlock()
	}

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	run(func() {
		v, err := svc.jiraRepo.GetJiraProvider(ctx, id)
		if err != nil && !errors.Is(err, jirastore.ErrNotFound) {
			addErr(fmt.Errorf("loading jira provider: %w", err))
			return
		}
		mu.Lock(); jp = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.slackRepo.GetSlackProvider(ctx, id)
		if err != nil && !errors.Is(err, slackstore.ErrNotFound) {
			addErr(fmt.Errorf("loading slack provider: %w", err))
			return
		}
		mu.Lock(); sp = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.jiraRepo.GetIssuesByContext(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading issues: %w", err)); return }
		mu.Lock(); issues = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.slackRepo.ListSlackThreads(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading slack threads: %w", err)); return }
		mu.Lock(); threads = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.slackRepo.ListSlackChannels(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading slack channels: %w", err)); return }
		mu.Lock(); channels = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.slackRepo.ListSlackDMConfigs(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading slack dm configs: %w", err)); return }
		mu.Lock(); dmConfigs = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.store.ListContextTexts(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading context texts: %w", err)); return }
		mu.Lock(); ctxTexts = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.store.ListUploadedFiles(ctx, id, false)
		if err != nil { addErr(fmt.Errorf("loading uploaded files: %w", err)); return }
		mu.Lock(); uploaded = v; mu.Unlock()
	})
	run(func() {
		v, err := svc.store.ListContextRelations(ctx, id)
		if err != nil { addErr(fmt.Errorf("loading relations: %w", err)); return }
		mu.Lock(); relations = v; mu.Unlock()
	})
	if svc.vcsStore != nil {
		run(func() {
			v, err := svc.vcsStore.Branches.ListBranches(ctx, id)
			if err != nil {
				svc.logger.Warn("loading vcs branches failed", zap.String("contextID", id), zap.Error(err))
				return
			}
			mu.Lock(); branches = v; mu.Unlock()
		})
		run(func() {
			v, err := svc.vcsStore.PullRequests.ListPRsForContext(ctx, id)
			if err != nil {
				svc.logger.Warn("loading vcs prs failed", zap.String("contextID", id), zap.Error(err))
				return
			}
			mu.Lock(); prs = v; mu.Unlock()
		})
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, errs[0]
	}

	// Phase 2: fetch comments now that issue IDs and PR IDs are known.
	var (
		commentsByIssue map[string][]jirastore.Comment
		commentsByPR    map[string][]vcsstore.PRComment
	)
	var wg2 sync.WaitGroup
	if len(issues) > 0 {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			issueIDs := make([]string, len(issues))
			for i, iss := range issues { issueIDs[i] = iss.ID }
			v, err := svc.jiraRepo.GetCommentsByIssues(ctx, issueIDs, false)
			if err != nil { addErr(fmt.Errorf("loading comments: %w", err)); return }
			mu.Lock(); commentsByIssue = v; mu.Unlock()
		}()
	}
	if svc.vcsStore != nil && len(prs) > 0 {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			prIDs := make([]string, len(prs))
			for i, pr := range prs { prIDs[i] = pr.ID }
			v, err := svc.vcsStore.PullRequests.ListCommentsForPRs(ctx, prIDs)
			if err != nil {
				svc.logger.Warn("loading pr comments failed", zap.Error(err))
				return
			}
			mu.Lock(); commentsByPR = v; mu.Unlock()
		}()
	}
	wg2.Wait()

	// Phase 2 error handling is asymmetric by design: Jira comment failures are
	// fatal (matching original behavior) while VCS PR comment failures are
	// non-fatal warnings (VCS data is supplementary).
	if len(errs) > 0 {
		return nil, errs[0]
	}

	// Assemble detail (single goroutine from here).
	for i, iss := range issues {
		issues[i].Comments = commentsByIssue[iss.ID]
	}
	if svc.vcsStore != nil {
		for i, pr := range prs {
			prs[i].Comments = vcsstore.BuildCommentTree(commentsByPR[pr.ID])
		}
	}

	detail.Jira = jp
	detail.Slack = sp
	detail.Issues = issues
	detail.SlackThreads = threads
	detail.SlackChannels = channels
	detail.SlackDMConfigs = dmConfigs
	detail.ContextTexts = ctxTexts
	detail.UploadedFiles = uploaded
	detail.VCSBranches = branches
	detail.PullRequests = prs
	detail.Relations = relations

	return detail, nil
}

// GetContextDetailByKey is like GetContextDetail but looks up by user-assigned key.
func (svc *ContextService) GetContextDetailByKey(ctx context.Context, key string) (*ContextDetail, error) {
	c, err := svc.store.GetContextByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("loading context by key: %w", err)
	}
	return svc.GetContextDetail(ctx, c.ID)
}

// SearchNewIssues runs the context's saved JQL and returns a diff of new vs existing issues.
func (svc *ContextService) SearchNewIssues(ctx context.Context, contextID string) (*SearchDiff, error) {
	jp, err := svc.jiraRepo.GetJiraProvider(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("loading jira config: %w", err)
	}

	results, err := svc.jira.Search(ctx, jp.SearchQuery)
	if err != nil {
		return nil, fmt.Errorf("searching jira: %w", err)
	}

	existingKeys, err := svc.jiraRepo.GetIssueJiraKeys(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("loading existing keys: %w", err)
	}

	diff := &SearchDiff{ContextID: contextID}
	for _, r := range results {
		if existingKeys[r.Key] {
			diff.Existing = append(diff.Existing, r)
		} else {
			diff.New = append(diff.New, r)
		}
	}
	return diff, nil
}

// AddIssuesToContext fetches full details for the given Jira keys and upserts them into the context.
// For each primary issue it also adds: subtasks from the API subtasks field, direct child issues
// found via JQL "parent = KEY" (handles next-gen Jira projects), and all linked issues.
// Cross-context relations are still auto-created for linked keys that exist in other contexts.
// Returns the count of issues added.
func (svc *ContextService) AddIssuesToContext(ctx context.Context, contextID string, jiraKeys []string) (int, error) {
	if len(jiraKeys) == 0 {
		return 0, nil
	}
	added := 0
	seen := make(map[string]bool, len(jiraKeys))
	for _, key := range jiraKeys {
		seen[key] = true
	}

	type pendingLink struct {
		typeName  string
		direction string
		linkedKey string
	}
	var collectedLinks []pendingLink
	var refKeys []string // subtasks + linked issues + JQL children

	// upsertSimple fetches full details, persists the issue, and returns the detail for ref collection.
	// Returns nil if fetch or persist fails.
	upsertSimple := func(key string) *jira.IssueDetail {
		detail, err := svc.jira.ViewIssue(ctx, key)
		if err != nil {
			svc.logger.Warn("failed to view issue", zap.String("key", key), zap.Error(err))
			return nil
		}
		if _, err = svc.jiraRepo.UpsertIssue(ctx, contextID,
			detail.Key, detail.Summary, detail.Status, detail.Assignee,
			detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
		); err != nil {
			svc.logger.Warn("failed to upsert issue", zap.String("key", key), zap.Error(err))
			return nil
		}
		return detail
	}

	// Primary pass: upsert each requested issue and collect all refs.
	for _, key := range jiraKeys {
		detail := upsertSimple(key)
		if detail == nil {
			continue
		}
		added++
		for _, sub := range detail.Subtasks {
			if !seen[sub.Key] {
				seen[sub.Key] = true
				refKeys = append(refKeys, sub.Key)
			}
		}
		for _, link := range detail.Links {
			if !seen[link.LinkedKey] {
				seen[link.LinkedKey] = true
				refKeys = append(refKeys, link.LinkedKey)
			}
			collectedLinks = append(collectedLinks, pendingLink{
				typeName:  link.TypeName,
				direction: link.Direction,
				linkedKey: link.LinkedKey,
			})
		}
	}

	// Fetch child issues via JQL for next-gen Jira projects where child issues
	// are not included in the subtasks API field.
	for _, key := range jiraKeys {
		children, err := svc.jira.Search(ctx, fmt.Sprintf("parent = %q", key))
		if err != nil {
			svc.logger.Warn("child search failed", zap.String("key", key), zap.Error(err))
			continue
		}
		for _, child := range children {
			if !seen[child.Key] {
				seen[child.Key] = true
				refKeys = append(refKeys, child.Key)
			}
		}
	}

	// Ref pass: upsert all referenced issues (subtasks, linked, children).
	for _, key := range refKeys {
		if upsertSimple(key) != nil {
			added++
		}
	}

	// Auto-link related contexts via Jira issue links (existing behavior).
	if len(collectedLinks) > 0 {
		seenLinkedKeys := make(map[string]bool)
		linkedKeys := make([]string, 0, len(collectedLinks))
		for _, l := range collectedLinks {
			if !seenLinkedKeys[l.linkedKey] {
				seenLinkedKeys[l.linkedKey] = true
				linkedKeys = append(linkedKeys, l.linkedKey)
			}
		}
		keyToCtxIDs, err := svc.jiraRepo.FindContextsByJiraKeys(ctx, linkedKeys)
		if err != nil {
			svc.logger.Warn("auto-link: finding contexts by jira keys failed", zap.Error(err))
		} else {
			deduped := make(map[string]bool)
			for _, l := range collectedLinks {
				targetCtxIDs := keyToCtxIDs[l.linkedKey]
				for _, targetCtxID := range targetCtxIDs {
					if targetCtxID == contextID {
						continue
					}
					relType := jiraLinkToRelationType(l.typeName, l.direction)
					dedupeKey := targetCtxID + "|" + relType
					if deduped[dedupeKey] {
						continue
					}
					deduped[dedupeKey] = true
					if _, err := svc.store.AddContextRelation(ctx, contextID, targetCtxID, relType); err != nil {
						if strings.Contains(err.Error(), "UNIQUE constraint failed") {
							continue
						}
						svc.logger.Warn("auto-link: failed to create relation",
							zap.String("from", contextID), zap.String("to", targetCtxID),
							zap.String("type", relType), zap.Error(err))
					}
				}
			}
		}
	}

	return added, svc.store.TouchContext(ctx, contextID)
}

// ExtractGitHubReposFromIssues reads stored issue descriptions for the given Jira keys
// and returns unique "owner/repo" pairs found in GitHub URLs within those descriptions.
// This is a best-effort operation: it reads what's already in the DB after add.
func (svc *ContextService) ExtractGitHubReposFromIssues(ctx context.Context, contextID string, keys []string) []string {
	issues, err := svc.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		svc.logger.Warn("extractGitHubRepos: failed to load issues", zap.Error(err))
		return nil
	}
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}
	seen := make(map[string]bool)
	var repos []string
	for _, issue := range issues {
		if !keySet[issue.JiraKey] {
			continue
		}
		for _, ownerRepo := range jira.ExtractGitHubRepos(issue.Description) {
			if !seen[ownerRepo] {
				seen[ownerRepo] = true
				repos = append(repos, ownerRepo)
			}
		}
	}
	return repos
}

// SyncAllComments fetches and deduplicates comments for every issue in a context.
// Returns the total number of new comments inserted.
func (svc *ContextService) SyncAllComments(ctx context.Context, contextID string) (int, error) {
	issues, err := svc.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		return 0, fmt.Errorf("loading issues: %w", err)
	}
	return syncIssueComments(ctx, svc.jira, svc.jiraRepo, issues, svc.logger)
}

// syncIssueComments fetches and deduplicates comments for the given issues.
// Errors per-issue are logged and skipped; returns the count of newly inserted comments.
// Comments are fetched concurrently using a pool of up to 5 workers.
func syncIssueComments(ctx context.Context, jiraClient commentLister, jiraRepo *jirastore.Repo, issues []jirastore.Issue, logger *zap.Logger) (int, error) {
	if len(issues) == 0 || jiraRepo == nil {
		return 0, nil
	}

	workers := min(5, len(issues))

	jobs := make(chan jirastore.Issue, len(issues))
	for _, iss := range issues {
		jobs <- iss
	}
	close(jobs)

	var total atomic.Int64

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for issue := range jobs {
				comments, err := jiraClient.ListComments(ctx, issue.JiraKey)
				if err != nil {
					logger.Warn("sync comments: list failed", zap.String("key", issue.JiraKey), zap.Error(err))
					continue
				}
				for _, c := range comments {
					inserted, err := jiraRepo.UpsertComment(ctx, issue.ID, c.ID, c.Author, c.Body, c.Created)
					if err != nil {
						logger.Warn("sync comments: upsert failed",
							zap.String("issue", issue.JiraKey), zap.String("jiraID", c.ID), zap.Error(err))
						continue
					}
					if inserted {
						total.Add(1)
					}
				}
			}
		}()
	}
	wg.Wait()
	return int(total.Load()), nil
}

// LoadSlackContext calls slack-cli load-context, stores the markdown output for the given thread.
func (svc *ContextService) LoadSlackContext(ctx context.Context, args LoadSlackContextArgs) error {
	if _, err := svc.store.GetContext(ctx, args.ContextID); err != nil {
		return fmt.Errorf("loading context: %w", err)
	}

	flags := []string{args.Workspace, args.ChannelID, args.ThreadTS}
	if args.ChannelName != "" {
		flags = append(flags, "--channel-name", args.ChannelName)
	}
	if args.Permalink != "" {
		flags = append(flags, "--permalink", args.Permalink)
	}
	if args.SearchQuery != "" {
		flags = append(flags, "--search-query", args.SearchQuery)
	}

	content, err := svc.slack.Run(ctx, []string{"load-context"}, flags)
	if err != nil {
		return fmt.Errorf("loading slack context: %w", err)
	}

	_, err = svc.slackRepo.UpsertSlackThread(ctx, args.ContextID, slackstore.UpsertSlackThreadArgs{
		Workspace:   args.Workspace,
		ChannelID:   args.ChannelID,
		ThreadTS:    args.ThreadTS,
		ChannelName: args.ChannelName,
		Permalink:   args.Permalink,
		SearchQuery: args.SearchQuery,
		Content:     content,
	})
	if err != nil {
		return fmt.Errorf("storing slack thread: %w", err)
	}
	return svc.store.TouchContext(ctx, args.ContextID)
}

// ListSlackThreads returns all stored Slack threads for a context.
func (svc *ContextService) ListSlackThreads(ctx context.Context, contextID string) ([]slackstore.SlackThread, error) {
	return svc.slackRepo.ListSlackThreads(ctx, contextID, false)
}

// SearchSlackChannels calls slack-cli search-channels with a glob name pattern and returns
// the matching channels. namePattern may use glob-style wildcards (e.g. "*deploy*").
func (svc *ContextService) SearchSlackChannels(ctx context.Context, workspace, namePattern string) ([]SlackChannelResult, error) {
	if workspace == "" {
		return nil, fmt.Errorf("slack workspace is required")
	}
	if namePattern == "" {
		return nil, fmt.Errorf("channel name pattern is required")
	}

	output, err := svc.slack.Run(ctx, []string{"search-channels"}, []string{workspace, namePattern})
	if err != nil {
		return nil, fmt.Errorf("searching slack channels: %w", err)
	}

	var results []SlackChannelResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("parsing slack channel results: %w", err)
	}
	// Populate Content from Messages so callers (add-channel flow) can store it immediately.
	for i := range results {
		if len(results[i].Messages) > 0 {
			results[i].Content = formatMessages(results[i].Name, results[i].Messages)
		}
	}
	return results, nil
}

// AddSlackChannels upserts multiple Slack channels into a context.
// If a channel carries Messages (pre-loaded by slack-cli search-channels), each message is
// stored immediately as its own slack_thread record so the user sees content without a Refresh.
func (svc *ContextService) AddSlackChannels(ctx context.Context, contextID, workspace string, channels []slackstore.SlackChannelInput) error {
	if workspace == "" {
		return fmt.Errorf("slack workspace is required")
	}
	if len(channels) == 0 {
		return fmt.Errorf("at least one channel is required")
	}
	if _, err := svc.store.GetContext(ctx, contextID); err != nil {
		return fmt.Errorf("loading context: %w", err)
	}
	for _, ch := range channels {
		slackCh, err := svc.slackRepo.UpsertSlackChannel(ctx, contextID, workspace, ch.ChannelID, ch.ChannelName, ch.StartFrom)
		if err != nil {
			return fmt.Errorf("upserting channel %q: %w", ch.ChannelName, err)
		}
		// Store each message (and its replies) as its own slack_thread record so users can manage
		// individual messages (delete, enable/disable) without a separate Refresh.
		// Iterate in reverse so oldest messages get the earliest created_at timestamps.
		for i := len(ch.Messages) - 1; i >= 0; i-- {
			msg := ch.Messages[i]
			if msg.RawTS == "" {
				continue
			}
			if _, err := svc.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
				SlackChannelID: slackCh.ID,
				Workspace:      workspace,
				ChannelID:      ch.ChannelID,
				ThreadTS:       msg.RawTS,
				ChannelName:    ch.ChannelName,
				Content:        formatSingleMessage(msg),
			}); err != nil {
				svc.logger.Warn("add channels: upsert message thread failed",
					zap.String("channel", ch.ChannelName), zap.String("ts", msg.RawTS), zap.Error(err))
			}
			for _, reply := range msg.Replies {
				if reply.RawTS == "" {
					continue
				}
				if _, err := svc.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
					SlackChannelID: slackCh.ID,
					Workspace:      workspace,
					ChannelID:      ch.ChannelID,
					ThreadTS:       reply.RawTS,
					ParentThreadTS: msg.RawTS,
					ChannelName:    ch.ChannelName,
					Content:        formatSingleMessage(reply),
				}); err != nil {
					svc.logger.Warn("add channels: upsert reply thread failed",
						zap.String("channel", ch.ChannelName), zap.String("ts", reply.RawTS), zap.Error(err))
				}
			}
		}
	}
	return svc.store.TouchContext(ctx, contextID)
}

// ListSlackChannels returns all Slack channels configured for a context.
func (svc *ContextService) ListSlackChannels(ctx context.Context, contextID string) ([]slackstore.SlackChannel, error) {
	return svc.slackRepo.ListSlackChannels(ctx, contextID, false)
}

// RemoveSlackChannel removes a Slack channel from a context by its record ID.
// All slack_threads belonging to the same channel are also removed.
func (svc *ContextService) RemoveSlackChannel(ctx context.Context, id string) error {
	return svc.slackRepo.RemoveSlackChannel(ctx, id)
}

// DeleteJiraIssue removes a single Jira issue by its internal ID.
// All comments for the issue are cascade-deleted via the FK constraint.
func (svc *ContextService) DeleteJiraIssue(ctx context.Context, id string) error {
	return svc.jiraRepo.DeleteIssue(ctx, id)
}

// DeleteJiraComment removes a single Jira comment by its internal ID.
func (svc *ContextService) DeleteJiraComment(ctx context.Context, id string) error {
	return svc.jiraRepo.DeleteComment(ctx, id)
}

// DeleteSlackThread removes a single Slack thread by its internal ID.
func (svc *ContextService) DeleteSlackThread(ctx context.Context, id string) error {
	return svc.slackRepo.DeleteSlackThread(ctx, id)
}

// ListContextTexts returns all text entries attached to a context.
func (svc *ContextService) ListContextTexts(ctx context.Context, contextID string) ([]appstore.ContextText, error) {
	return svc.store.ListContextTexts(ctx, contextID, false)
}

// UpsertContextText creates a new text entry (when id is empty) or updates an existing one.
func (svc *ContextService) UpsertContextText(ctx context.Context, id, contextID, name, content string) (*appstore.ContextText, error) {
	return svc.store.UpsertContextText(ctx, id, contextID, name, content)
}

// DeleteContextText removes a single text entry by its internal ID.
func (svc *ContextService) DeleteContextText(ctx context.Context, id string) error {
	return svc.store.DeleteContextText(ctx, id)
}

// ToggleIssueActive flips the is_active flag for a Jira issue.
func (svc *ContextService) ToggleIssueActive(ctx context.Context, id string) (*jirastore.Issue, error) {
	return svc.jiraRepo.ToggleIssueActive(ctx, id)
}

// ToggleCommentActive flips the is_active flag for a Jira comment.
func (svc *ContextService) ToggleCommentActive(ctx context.Context, id string) (*jirastore.Comment, error) {
	return svc.jiraRepo.ToggleCommentActive(ctx, id)
}

// ToggleSlackChannelActive flips the is_active flag for a Slack channel.
func (svc *ContextService) ToggleSlackChannelActive(ctx context.Context, id string) (*slackstore.SlackChannel, error) {
	return svc.slackRepo.ToggleSlackChannelActive(ctx, id)
}

// ToggleSlackThreadActive flips the is_active flag for a Slack thread.
func (svc *ContextService) ToggleSlackThreadActive(ctx context.Context, id string) (*slackstore.SlackThread, error) {
	return svc.slackRepo.ToggleSlackThreadActive(ctx, id)
}

// ToggleContextTextActive flips the is_active flag for a text entry.
func (svc *ContextService) ToggleContextTextActive(ctx context.Context, id string) (*appstore.ContextText, error) {
	return svc.store.ToggleContextTextActive(ctx, id)
}

// RefreshContext re-fetches the latest data for every item already stored in the context
// by delegating to each registered provider. It does NOT execute JQL or add new items.
// An optional onProgress callback is called before each provider with a 1-based index.
func (svc *ContextService) RefreshContext(ctx context.Context, contextID string, onProgress ...RefreshProgressFunc) (*RefreshResult, error) {
	result := &RefreshResult{}

	var notify RefreshProgressFunc
	if len(onProgress) > 0 && onProgress[0] != nil {
		notify = onProgress[0]
	}

	before := svc.snapshotBefore(ctx, contextID)

	for i, p := range svc.providers {
		if notify != nil {
			notify(i+1, len(svc.providers), p.Name())
		}
		stats, err := p.Refresh(ctx, contextID)
		if err != nil {
			svc.logger.Warn("refresh: provider failed",
				zap.String("provider", p.Name()), zap.String("contextID", contextID), zap.Error(err))
			continue
		}
		// Map each provider's stats to the named result fields for frontend compatibility.
		switch p.Name() {
		case providerJira:
			result.IssuesRefreshed += stats.ItemsRefreshed
			result.NewComments += stats.NewItems
		case providerSlack:
			result.ThreadsRefreshed += stats.ItemsRefreshed
			result.DMsRefreshed += stats.NewItems
		case "vcs":
			result.PRsRefreshed += stats.ItemsRefreshed
			result.NewPRComments += stats.NewItems
			result.NewNotifications += stats.NewNotifications
		}
	}

	isFirst, _ := svc.store.IsFirstRefresh(ctx, contextID)

	svc.recordContextChange(ctx, contextID, contextlog.TriggerRefresh, before)
	svc.runPostRefreshWorkflow(ctx, contextID, isFirst)

	if err := svc.store.TouchContext(ctx, contextID); err != nil {
		svc.logger.Warn("refresh: touch context failed", zap.String("contextID", contextID), zap.Error(err))
	}

	return result, nil
}

// snapshotBefore takes a snapshot for change-log diffing, or returns nil if logging
// is disabled or the snapshot fails (in which case RecordChange is skipped).
func (svc *ContextService) snapshotBefore(ctx context.Context, contextID string) contextlog.Snapshot {
	if svc.contextLog == nil {
		return nil
	}
	snap, err := svc.contextLog.Repo().SnapshotFull(ctx, contextID)
	if err != nil {
		svc.logger.Warn("contextlog: snapshot before failed",
			zap.String("contextID", contextID), zap.Error(err))
		return nil
	}
	return snap
}

// recordContextChange takes an "after" snapshot and records the diff through contextlog.
// A nil before (typically from snapshotBefore failure or disabled logging) is skipped.
func (svc *ContextService) recordContextChange(ctx context.Context, contextID, trigger string, before contextlog.Snapshot) {
	if svc.contextLog == nil || before == nil {
		return
	}
	after, err := svc.contextLog.Repo().SnapshotFull(ctx, contextID)
	if err != nil {
		svc.logger.Warn("contextlog: snapshot after failed",
			zap.String("contextID", contextID), zap.Error(err))
		return
	}
	if _, err := svc.contextLog.RecordChange(ctx, contextID, trigger, before, after); err != nil {
		svc.logger.Warn("contextlog: record change failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
}

// FormatContext collects all provider artifacts for contextID, sorts them globally
// newest-first, and writes them to w.
// filter limits output to a specific provider name (e.g. "jira", "slack"); empty = all providers.
// Only items on or after startAt are included; pass a zero time for no filter.
func (svc *ContextService) FormatContext(ctx context.Context, w io.Writer, contextID string, filter string, startAt time.Time, activeOnly bool) error {
	var all []provider.Item
	for _, p := range svc.providers {
		if filter != "" && filter != p.Name() {
			continue
		}
		items, err := p.CollectItems(ctx, contextID, startAt, activeOnly)
		if err != nil {
			svc.logger.Warn("format context: provider failed",
				zap.String("provider", p.Name()), zap.String("contextID", contextID), zap.Error(err))
			continue
		}
		all = append(all, items...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	for _, item := range all {
		_, _ = fmt.Fprint(w, item.Body)
	}
	return nil
}

// ReloadAllContexts iterates over every active context and delegates a full reload to each
// registered provider. It returns one result per context regardless of errors so the caller
// always receives a complete picture.
// An optional onProgress callback is invoked before each context is processed; current is
// 1-based and total reflects the number of active contexts being reloaded.
func (svc *ContextService) ReloadAllContexts(ctx context.Context, onProgress ...ReloadProgressFunc) ([]ReloadAllResult, error) {
	active, err := svc.store.ListContexts(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("listing contexts: %w", err)
	}

	var notify ReloadProgressFunc
	if len(onProgress) > 0 && onProgress[0] != nil {
		notify = onProgress[0]
	}

	results := make([]ReloadAllResult, len(active))
	sem := make(chan struct{}, 4)

	var notifyMu sync.Mutex
	var wg sync.WaitGroup
	for i, c := range active {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, c appstore.Context) {
			defer wg.Done()
			defer func() { <-sem }()

			r := ReloadAllResult{ContextID: c.ID, ContextKey: c.Key}
			before := svc.snapshotBefore(ctx, c.ID)

			if notify != nil {
				notifyMu.Lock()
				notify(i+1, len(active), c.Key, "")
				notifyMu.Unlock()
			}

			type providerResult struct {
				name  string
				stats provider.Stats
				err   error
			}
			ch := make(chan providerResult, len(svc.providers))

			var pwg sync.WaitGroup
			for _, p := range svc.providers {
				pwg.Add(1)
				go func(p provider.Provider) {
					defer pwg.Done()
					stats, err := p.Reload(ctx, c.ID)
					ch <- providerResult{name: p.Name(), stats: stats, err: err}
				}(p)
			}
			pwg.Wait()
			close(ch)

			var errs []string
			for res := range ch {
				if res.err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", res.name, res.err))
					svc.logger.Warn("reload: provider failed",
						zap.String("provider", res.name), zap.String("contextID", c.ID), zap.Error(res.err))
					continue
				}
				// Map each provider's stats to the named result fields for frontend compatibility.
				switch res.name {
				case providerJira:
					r.NewIssues += res.stats.ItemsRefreshed // new issues found by JQL
					r.NewComments += res.stats.NewItems     // new comments
				case providerSlack:
					r.ThreadsRefreshed += res.stats.ItemsRefreshed
					r.DMsRefreshed += res.stats.NewItems
				case "vcs":
					r.PRsRefreshed += res.stats.ItemsRefreshed
					r.NewPRComments += res.stats.NewItems
					r.NewNotifications += res.stats.NewNotifications
				}
			}
			if len(errs) > 0 {
				r.Error = strings.Join(errs, "; ")
			}

			svc.recordContextChange(ctx, c.ID, contextlog.TriggerReload, before)
			results[i] = r
		}(i, c)
	}
	wg.Wait()
	return results, nil
}

// ListSlackDMConfigs returns all DM load configurations for a context.
func (svc *ContextService) ListSlackDMConfigs(ctx context.Context, contextID string) ([]slackstore.SlackDMConfig, error) {
	return svc.slackRepo.ListSlackDMConfigs(ctx, contextID, false)
}

// UpsertSlackDMConfig creates or updates a DM load configuration and immediately
// loads the DM content via slack-cli list-dms for the given workspace and startFrom date.
func (svc *ContextService) UpsertSlackDMConfig(ctx context.Context, contextID, workspace, startFrom string) (*slackstore.SlackDMConfig, error) {
	if workspace == "" {
		return nil, fmt.Errorf("slack workspace is required")
	}
	if _, err := svc.store.GetContext(ctx, contextID); err != nil {
		return nil, fmt.Errorf("loading context: %w", err)
	}

	cfg, err := svc.slackRepo.UpsertSlackDMConfig(ctx, contextID, workspace, startFrom)
	if err != nil {
		return nil, fmt.Errorf("upserting dm config: %w", err)
	}

	content, err := svc.fetchDMContent(ctx, workspace, startFrom)
	if err != nil {
		svc.logger.Warn("upsert dm config: fetching dm content failed",
			zap.String("contextID", contextID), zap.String("workspace", workspace), zap.Error(err))
	} else if content != "" {
		if err := svc.slackRepo.UpdateSlackDMConfigContent(ctx, cfg.ID, content); err != nil {
			svc.logger.Warn("upsert dm config: storing content failed",
				zap.String("contextID", contextID), zap.Error(err))
		} else {
			cfg.Content = content
		}
	}

	return cfg, svc.store.TouchContext(ctx, contextID)
}

// DeleteSlackDMConfig removes a DM load configuration by its record ID.
func (svc *ContextService) DeleteSlackDMConfig(ctx context.Context, id string) error {
	return svc.slackRepo.DeleteSlackDMConfig(ctx, id)
}

// ToggleSlackDMConfigActive flips the is_active flag for a Slack DM config.
func (svc *ContextService) ToggleSlackDMConfigActive(ctx context.Context, id string) (*slackstore.SlackDMConfig, error) {
	return svc.slackRepo.ToggleSlackDMConfigActive(ctx, id)
}

// RemoveSlackDMEntry removes a single DM conversation entry from a DM config's content.
func (svc *ContextService) RemoveSlackDMEntry(ctx context.Context, dmConfigID, entryID string) error {
	return svc.slackRepo.RemoveSlackDMEntry(ctx, dmConfigID, entryID)
}

// PreviewSlackDMs calls slack-cli list-dms for the given workspace and startFrom date
// and returns the formatted output without persisting anything.
// Returns an empty string when no personal messages are found.
func (svc *ContextService) PreviewSlackDMs(ctx context.Context, workspace, startFrom string) (string, error) {
	if workspace == "" {
		return "", fmt.Errorf("slack workspace is required")
	}
	return svc.fetchDMContent(ctx, workspace, startFrom)
}

// fetchDMContent calls slack-cli list-dms --with-messages and returns formatted output.
func (svc *ContextService) fetchDMContent(ctx context.Context, workspace, startFrom string) (string, error) {
	flags := []string{workspace, "--with-messages"}
	if startFrom != "" {
		flags = append(flags, "--start-from", startFrom)
	}
	raw, err := svc.slack.Run(ctx, []string{"list-dms"}, flags)
	if err != nil {
		return "", err
	}
	return formatDMContent(raw), nil
}

// formatDMContent parses the JSON array returned by list-dms --with-messages
// and renders each conversation's messages into a readable plain-text block.
func formatDMContent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var entries []slackDMEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return raw
	}
	if len(entries) == 0 {
		return ""
	}

	// Sort newest-first: lexicographic comparison works for "YYYY-MM-DD HH:MM UTC" timestamps.
	sort.Slice(entries, func(i, j int) bool {
		ti, tj := "", ""
		if mi := entries[i].allMessages(); len(mi) > 0 {
			ti = mi[0].Timestamp
		}
		if mj := entries[j].allMessages(); len(mj) > 0 {
			tj = mj[0].Timestamp
		}
		return ti > tj
	})

	var sb strings.Builder
	for _, e := range entries {
		label := e.UserName
		if label == "" {
			label = e.Name
		}
		if label == "" {
			label = e.ID
		}

		msgs := e.allMessages()
		if len(msgs) == 0 {
			fmt.Fprintf(&sb, "# DM with %s\n(no messages)\n\n", label)
			continue
		}
		fmt.Fprintf(&sb, "# DM with %s\n\n", label)
		for _, m := range msgs {
			fmt.Fprintf(&sb, "[%s] %s: %s\n", m.Timestamp, m.DisplayUser(), m.Text)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

// ListUploadedFiles returns all uploaded files attached to a context.
func (svc *ContextService) ListUploadedFiles(ctx context.Context, contextID string) ([]appstore.ContextUploadedFile, error) {
	return svc.store.ListUploadedFiles(ctx, contextID, false)
}

// InsertUploadedFile stores an already-extracted document (plus its original binary) for a context.
func (svc *ContextService) InsertUploadedFile(ctx context.Context, contextID, originalName, fileType, extractedText string, fileContent []byte, fileSize int64) (*appstore.ContextUploadedFile, error) {
	return svc.store.InsertUploadedFile(ctx, contextID, originalName, fileType, extractedText, fileContent, fileSize)
}

// DeleteUploadedFile removes a single uploaded file by its internal ID.
func (svc *ContextService) DeleteUploadedFile(ctx context.Context, id string) error {
	return svc.store.DeleteUploadedFile(ctx, id)
}

// ToggleUploadedFileActive flips the is_active flag for an uploaded file.
func (svc *ContextService) ToggleUploadedFileActive(ctx context.Context, id string) (*appstore.ContextUploadedFile, error) {
	return svc.store.ToggleUploadedFileActive(ctx, id)
}

// SearchJQL runs an arbitrary JQL query and returns matching issues.
// This is intended for query preview / validation without saving to a context.
func (svc *ContextService) SearchJQL(ctx context.Context, jql string) ([]jira.SearchResult, error) {
	if jql == "" {
		return nil, fmt.Errorf("jql query is required")
	}
	results, err := svc.jira.Search(ctx, jql)
	if err != nil {
		return nil, fmt.Errorf("searching jira: %w", err)
	}
	return results, nil
}

// ListSlackWorkspaces calls slack-cli list-workspaces and returns the available workspaces.
// slack-cli list-workspaces emits one workspace name per line (plain text, no JSON).
func (svc *ContextService) ListSlackWorkspaces(ctx context.Context) ([]SlackWorkspaceResult, error) {
	output, err := svc.slack.Run(ctx, []string{"list-workspaces"}, nil)
	if err != nil {
		return nil, fmt.Errorf("listing slack workspaces: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	results := make([]SlackWorkspaceResult, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		results = append(results, SlackWorkspaceResult{ID: name, Name: name})
	}
	return results, nil
}

// MarkIssueViewed marks a Jira issue as viewed (sets viewed_at = now).
func (svc *ContextService) MarkIssueViewed(ctx context.Context, id string) error {
	return svc.jiraRepo.MarkIssueViewed(ctx, id)
}

// MarkCommentViewed marks a Jira comment as viewed.
func (svc *ContextService) MarkCommentViewed(ctx context.Context, id string) error {
	return svc.jiraRepo.MarkCommentViewed(ctx, id)
}

// MarkSlackThreadViewed marks a Slack thread as viewed.
func (svc *ContextService) MarkSlackThreadViewed(ctx context.Context, id string) error {
	return svc.slackRepo.MarkSlackThreadViewed(ctx, id)
}

// MarkSlackDMConfigViewed marks a Slack DM config as viewed.
func (svc *ContextService) MarkSlackDMConfigViewed(ctx context.Context, id string) error {
	return svc.slackRepo.MarkSlackDMConfigViewed(ctx, id)
}

// MarkContextAllViewed sets viewed_at = now for every unread item in a context.
func (svc *ContextService) MarkContextAllViewed(ctx context.Context, contextID string) error {
	return svc.store.MarkContextAllViewed(ctx, contextID)
}

// GetUnviewedBreakdown returns per-type unread counts for a context.
func (svc *ContextService) GetUnviewedBreakdown(ctx context.Context, contextID string) (appstore.UnviewedBreakdown, error) {
	return svc.store.GetUnviewedBreakdown(ctx, contextID)
}

// SetJiraBaseURL stores the Jira instance base URL for a context.
func (svc *ContextService) SetJiraBaseURL(ctx context.Context, contextID, baseURL string) error {
	return svc.jiraRepo.SetJiraBaseURL(ctx, contextID, baseURL)
}

// RefreshAllContexts refreshes every active context and returns aggregated results.
// notify is called before each provider step, same signature as ReloadAllContexts.
func (svc *ContextService) RefreshAllContexts(ctx context.Context, notify ReloadProgressFunc) ([]ReloadAllResult, error) {
	contexts, err := svc.store.ListContexts(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("listing contexts for refresh: %w", err)
	}
	total := len(contexts)
	results := make([]ReloadAllResult, 0, total)
	for i, c := range contexts {
		idx := i + 1
		res, err := svc.RefreshContext(ctx, c.ID, func(_, _ int, step string) {
			if notify != nil {
				notify(idx, total, c.Key, step)
			}
		})
		if err != nil {
			results = append(results, ReloadAllResult{ContextID: c.ID, ContextKey: c.Key, Error: err.Error()})
			continue
		}
		results = append(results, ReloadAllResult{
			ContextID:        c.ID,
			ContextKey:       c.Key,
			ThreadsRefreshed: res.ThreadsRefreshed,
			DMsRefreshed:     res.DMsRefreshed,
		})
	}
	return results, nil
}

// runPostRefreshWorkflow runs after every refresh: recomputes AI proposal and
// optionally runs the open-questions agent (first refresh only) or re-evaluates
// existing open questions (subsequent refreshes).
func (svc *ContextService) runPostRefreshWorkflow(ctx context.Context, contextID string, isFirstRefresh bool) {
	if svc.vcsStore == nil {
		return
	}

	// 1. Compute new status.
	status, comment, err := workflow.Compute(ctx, contextID, svc.store, svc.vcsStore)
	if err != nil {
		svc.logger.Warn("workflow compute failed", zap.Error(err), zap.String("contextID", contextID))
		return
	}

	// 2. Load the current user's next-step override to pass to the AI.
	c, err := svc.store.GetContext(ctx, contextID)
	prevUserNextStep := ""
	if err == nil && c.UserNextStepOverride != nil {
		prevUserNextStep = *c.UserNextStepOverride
	}

	// 3. Generate next step (uses AI if configured, templates otherwise).
	nextStep := workflow.GenerateNextStep(ctx, svc.nextStepAI, status, comment, prevUserNextStep)

	// 4. Persist the AI proposal (also appends to proposals history).
	if _, err := svc.store.SetAIProposal(ctx, contextID, status, comment, nextStep, prevUserNextStep); err != nil {
		svc.logger.Warn("set ai proposal failed", zap.Error(err), zap.String("contextID", contextID))
	}

	// 5. Run open-questions agent.
	if svc.openQuestions != nil {
		contentMD := svc.buildContextMD(ctx, contextID)
		if isFirstRefresh {
			repos := svc.repoInfoForContext(ctx, contextID)
			if err := svc.openQuestions.AnalyzeContext(ctx, contextID, contentMD, repos); err != nil {
				svc.logger.Warn("open questions analysis failed", zap.Error(err), zap.String("contextID", contextID))
			}
		} else {
			if err := svc.openQuestions.ReevaluateOpenQuestions(ctx, contextID, contentMD); err != nil {
				svc.logger.Warn("open questions re-evaluation failed", zap.Error(err), zap.String("contextID", contextID))
			}
		}
	}
}

// buildContextMD assembles a Markdown representation of all active text content
// and uploaded files for the given context.
func (svc *ContextService) buildContextMD(ctx context.Context, contextID string) string {
	var sb strings.Builder
	texts, _ := svc.store.ListContextTexts(ctx, contextID, true)
	for _, t := range texts {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", t.Name, t.Content)
	}
	files, _ := svc.store.ListUploadedFiles(ctx, contextID, true)
	for _, f := range files {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", f.OriginalName, f.ExtractedText)
	}
	return sb.String()
}

// repoInfoForContext returns slim repo info for all repos associated with branches
// linked to the given context.
func (svc *ContextService) repoInfoForContext(ctx context.Context, contextID string) []openquestions.RepoInfo {
	if svc.vcsStore == nil {
		return nil
	}
	branches, err := svc.vcsStore.Branches.ListBranches(ctx, contextID)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var result []openquestions.RepoInfo
	for _, b := range branches {
		if b.RepoID == "" || seen[b.RepoID] {
			continue
		}
		seen[b.RepoID] = true
		if b.Repo == nil {
			continue
		}
		result = append(result, openquestions.RepoInfo{
			FullName:    b.Repo.Owner + "/" + b.Repo.Name,
			Label:       b.Repo.Label,
			Description: b.Repo.Description,
		})
	}
	return result
}
