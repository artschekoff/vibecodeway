package contextlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// AIClient is the minimal contract of the OpenAI wrapper. Declared here at the consumer
// side to avoid importing the openai package (which would create a wider dependency).
type AIClient interface {
	ChatCompletion(ctx context.Context, messages []AIMessage) (*AIResponse, error)
}

// AIMessage mirrors the structure expected by the AIClient. Keeping it in this package
// keeps the contextlog service decoupled from the concrete OpenAI SDK types.
type AIMessage struct {
	Role    string
	Content string
}

// AIResponse is the subset of OpenAI response fields the service needs.
type AIResponse struct {
	Content          string
	Model            string
	PromptTokens     int
	CompletionTokens int
}

// ErrAINotConfigured signals that no API key is available. Returned by service methods
// that require the model; the service still persists a structural record (no summary).
var ErrAINotConfigured = errors.New("ai client not configured")

// Service orchestrates the context-log lifecycle: diff recording, consolidation,
// chat, manual CRUD, and bootstrap initialisation.
type Service struct {
	repo   *Repo
	ai     AIClient
	logger *zap.Logger
}

// NewService wires a Service. ai may be nil; when nil, the service persists structural
// diff entries without AI-generated summaries and returns ErrAINotConfigured from
// explicit AI-dependent endpoints (Chat, Initialize, AnalyzeNow).
func NewService(repo *Repo, ai AIClient, logger *zap.Logger) *Service {
	return &Service{repo: repo, ai: ai, logger: logger}
}

// Repo returns the underlying repository (used by callers that need raw DB access
// such as snapshot.SnapshotFull).
func (s *Service) Repo() *Repo {
	return s.repo
}

// --- Change recording ---

// RecordChange persists a context_log entry describing the difference between before and after.
//
// Behaviour:
//   - Empty diff → no-op, no record created.
//   - AI unavailable (ai == nil or ErrAINotConfigured) → record persisted with only the
//     structural diff_json; summary_md is a fixed notice.
//   - AI available → record includes AI-generated summary, and UpdateConsolidated is
//     invoked afterwards.
func (s *Service) RecordChange(ctx context.Context, contextID, trigger string, before, after Snapshot) (*ContextLog, error) {
	diff := Diff(before, after)
	if diff.IsEmpty() {
		return nil, nil
	}

	diffJSON, err := json.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("marshalling diff: %w", err)
	}

	log := ContextLog{
		ContextID: contextID,
		Trigger:   trigger,
		DiffJSON:  string(diffJSON),
	}

	if s.ai == nil {
		log.SummaryMd = "_AI unavailable: configure OPENAI_API_KEY or save a key in Settings._"
		return s.repo.InsertLog(ctx, log)
	}

	prompt := renderDiffPrompt(diff)
	resp, err := s.ai.ChatCompletion(ctx, []AIMessage{
		{Role: "system", Content: SystemPromptChangeLog},
		{Role: "user", Content: UserDataMarker + "\n\n" + prompt},
	})
	if err != nil {
		if errors.Is(err, ErrAINotConfigured) {
			log.SummaryMd = "_AI unavailable: configure OPENAI_API_KEY or save a key in Settings._"
			return s.repo.InsertLog(ctx, log)
		}
		s.logger.Warn("contextlog.record_change.ai_failed",
			zap.String("contextID", contextID),
			zap.Error(err),
		)
		log.SummaryMd = "_Failed to generate summary: " + err.Error() + "_"
		return s.repo.InsertLog(ctx, log)
	}

	log.Model = resp.Model
	log.PromptTokens = resp.PromptTokens
	log.CompletionTokens = resp.CompletionTokens
	log.SummaryMd = strings.TrimSpace(resp.Content)

	stored, err := s.repo.InsertLog(ctx, log)
	if err != nil {
		return nil, err
	}

	if _, err := s.UpdateConsolidated(ctx, contextID); err != nil {
		s.logger.Warn("contextlog.record_change.consolidate_failed",
			zap.String("contextID", contextID),
			zap.Error(err),
		)
	}
	return stored, nil
}

// AnalyzeNow performs a change-log analysis on demand, using the current snapshot as
// "after" and an empty snapshot as "before". The resulting log is tagged trigger=manual.
// Caller is responsible for providing the current snapshot (normally obtained via
// repo.SnapshotFull).
func (s *Service) AnalyzeNow(ctx context.Context, contextID string, current Snapshot) (*ContextLog, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	empty := make(Snapshot)
	return s.RecordChange(ctx, contextID, TriggerManual, empty, current)
}

// --- Consolidation ---

// UpdateConsolidated rebuilds the consolidated log for contextID from the current list
// of context_logs plus the existing consolidated version.
func (s *Service) UpdateConsolidated(ctx context.Context, contextID string) (*ConsolidatedLog, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}

	logs, err := s.repo.ListLogs(ctx, contextID)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetConsolidated(ctx, contextID)
	if err != nil {
		return nil, err
	}

	userBlock := renderConsolidatePrompt(current, logs)
	resp, err := s.ai.ChatCompletion(ctx, []AIMessage{
		{Role: "system", Content: SystemPromptConsolidate},
		{Role: "user", Content: UserDataMarker + "\n\n" + userBlock},
	})
	if err != nil {
		return nil, fmt.Errorf("consolidating log: %w", err)
	}
	content := strings.TrimSpace(resp.Content)
	return s.repo.UpsertConsolidated(ctx, contextID, content, UpdatedByAI)
}

// UpdateConsolidatedManual replaces the consolidated log text with user-provided markdown.
func (s *Service) UpdateConsolidatedManual(ctx context.Context, contextID, contentMd string) (*ConsolidatedLog, error) {
	return s.repo.UpsertConsolidated(ctx, contextID, contentMd, UpdatedByUser)
}

// GetConsolidated exposes the current consolidated log (may be nil).
func (s *Service) GetConsolidated(ctx context.Context, contextID string) (*ConsolidatedLog, error) {
	return s.repo.GetConsolidated(ctx, contextID)
}

// Initialize bootstraps the consolidated log from the full current snapshot. It is a
// no-op if a consolidated log already exists — callers should check GetConsolidated first.
func (s *Service) Initialize(ctx context.Context, contextID string, current Snapshot) (*ConsolidatedLog, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}

	userBlock := renderInitPrompt(current)
	resp, err := s.ai.ChatCompletion(ctx, []AIMessage{
		{Role: "system", Content: SystemPromptInit},
		{Role: "user", Content: UserDataMarker + "\n\n" + userBlock},
	})
	if err != nil {
		return nil, fmt.Errorf("initialising consolidated log: %w", err)
	}
	return s.repo.UpsertConsolidated(ctx, contextID, strings.TrimSpace(resp.Content), UpdatedByAI)
}

// --- Manual log CRUD ---

// CreateManualLog adds a user-authored log entry and refreshes the consolidated log.
func (s *Service) CreateManualLog(ctx context.Context, contextID, summaryMd string) (*ContextLog, error) {
	if strings.TrimSpace(summaryMd) == "" {
		return nil, fmt.Errorf("summaryMd is required")
	}
	log := ContextLog{
		ContextID: contextID,
		Trigger:   TriggerManual,
		SummaryMd: summaryMd,
		IsManual:  true,
	}
	stored, err := s.repo.InsertLog(ctx, log)
	if err != nil {
		return nil, err
	}
	s.consolidateBestEffort(ctx, contextID)
	return stored, nil
}

// UpdateLog edits summary_md of an existing log entry and refreshes the consolidated log.
func (s *Service) UpdateLog(ctx context.Context, id, summaryMd string) (*ContextLog, error) {
	if strings.TrimSpace(summaryMd) == "" {
		return nil, fmt.Errorf("summaryMd is required")
	}
	updated, err := s.repo.UpdateLogSummary(ctx, id, summaryMd)
	if err != nil {
		return nil, err
	}
	s.consolidateBestEffort(ctx, updated.ContextID)
	return updated, nil
}

// DeleteLog removes a log entry and refreshes the consolidated log.
func (s *Service) DeleteLog(ctx context.Context, id string) error {
	existing, err := s.repo.GetLog(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteLog(ctx, id); err != nil {
		return err
	}
	s.consolidateBestEffort(ctx, existing.ContextID)
	return nil
}

// ListLogs returns all logs for contextID (newest first).
func (s *Service) ListLogs(ctx context.Context, contextID string) ([]ContextLog, error) {
	return s.repo.ListLogs(ctx, contextID)
}

// --- Chat ---

// ChatSend appends a user message, calls the AI, stores and returns the assistant reply.
// The prompt contains only the system prompt, the current consolidated log, and the last
// ChatHistoryLimit messages — raw context objects are deliberately excluded.
func (s *Service) ChatSend(ctx context.Context, contextID, userContent string) (*ChatMessage, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	if strings.TrimSpace(userContent) == "" {
		return nil, fmt.Errorf("message content is required")
	}

	if _, err := s.repo.InsertChatMessage(ctx, ChatMessage{
		ContextID: contextID,
		Role:      RoleUser,
		Content:   userContent,
	}); err != nil {
		return nil, err
	}

	consolidated, err := s.repo.GetConsolidated(ctx, contextID)
	if err != nil {
		return nil, err
	}
	history, err := s.repo.ListChatMessages(ctx, contextID, ChatHistoryLimit)
	if err != nil {
		return nil, err
	}

	messages := buildChatMessages(consolidated, history)
	resp, err := s.ai.ChatCompletion(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}
	content, proposed := splitProposedConsolidated(resp.Content)

	return s.repo.InsertChatMessage(ctx, ChatMessage{
		ContextID:              contextID,
		Role:                   RoleAssistant,
		Content:                strings.TrimSpace(content),
		ProposedConsolidatedMd: proposed,
	})
}

// ApplyChatSuggestion applies an assistant-proposed consolidated_md from a chat message
// to the consolidated log and marks the message as applied.
func (s *Service) ApplyChatSuggestion(ctx context.Context, messageID string) (*ConsolidatedLog, error) {
	msg, err := s.repo.GetChatMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.ProposedConsolidatedMd == "" {
		return nil, fmt.Errorf("message has no proposed consolidated text")
	}
	consolidated, err := s.repo.UpsertConsolidated(ctx, msg.ContextID, msg.ProposedConsolidatedMd, UpdatedByAI)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.MarkChatMessageApplied(ctx, messageID); err != nil {
		return nil, err
	}
	return consolidated, nil
}

// ListChat returns the full chat transcript for a context, oldest first.
func (s *Service) ListChat(ctx context.Context, contextID string) ([]ChatMessage, error) {
	return s.repo.ListChatMessages(ctx, contextID, 0)
}

// --- helpers ---

func (s *Service) consolidateBestEffort(ctx context.Context, contextID string) {
	if s.ai == nil {
		return
	}
	if _, err := s.UpdateConsolidated(ctx, contextID); err != nil {
		s.logger.Warn("contextlog.consolidate.failed",
			zap.String("contextID", contextID),
			zap.Error(err),
		)
	}
}

func renderDiffPrompt(diff DiffPayload) string {
	var b strings.Builder
	b.WriteString("Structural context diff (new → updated → removed objects):\n\n")
	if len(diff.Added) > 0 {
		b.WriteString("## Added\n\n")
		for _, c := range diff.Added {
			fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", c.Key, c.After.Content)
		}
	}
	if len(diff.Changed) > 0 {
		b.WriteString("## Changed\n\n")
		for _, c := range diff.Changed {
			fmt.Fprintf(&b, "### %s\n\n**Was:**\n```\n%s\n```\n\n**Now:**\n```\n%s\n```\n\n",
				c.Key, c.Before.Content, c.After.Content)
		}
	}
	if len(diff.Removed) > 0 {
		b.WriteString("## Removed\n\n")
		for _, c := range diff.Removed {
			fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", c.Key, c.Before.Content)
		}
	}
	return b.String()
}

func renderConsolidatePrompt(current *ConsolidatedLog, logs []ContextLog) string {
	var b strings.Builder
	b.WriteString("## Current consolidated log\n\n")
	if current == nil || current.ContentMd == "" {
		b.WriteString("_(not present yet)_\n\n")
	} else {
		b.WriteString(current.ContentMd)
		b.WriteString("\n\n")
	}
	b.WriteString("## All context_log entries (newest → oldest)\n\n")
	if len(logs) == 0 {
		b.WriteString("_(no entries)_\n")
		return b.String()
	}
	for _, l := range logs {
		fmt.Fprintf(&b, "### %s — %s%s\n\n%s\n\n",
			l.CreatedAt, l.Trigger, manualTag(l.IsManual), l.SummaryMd)
	}
	return b.String()
}

func renderInitPrompt(snap Snapshot) string {
	var b strings.Builder
	b.WriteString("Full current context snapshot:\n\n")
	if len(snap) == 0 {
		b.WriteString("_(context is empty — no objects)_\n")
		return b.String()
	}
	categories := groupByCategory(snap)
	for _, cat := range orderedCategories {
		items, ok := categories[cat]
		if !ok || len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s (%d)\n\n", cat, len(items))
		for _, it := range items {
			fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", it.key, it.snap.Content)
		}
	}
	return b.String()
}

func buildChatMessages(consolidated *ConsolidatedLog, history []ChatMessage) []AIMessage {
	var ctxBlock strings.Builder
	ctxBlock.WriteString("## Consolidated context log\n\n")
	if consolidated == nil || consolidated.ContentMd == "" {
		ctxBlock.WriteString("_(not present yet — raw context data is not sent to chat, please initialize the log first)_\n")
	} else {
		ctxBlock.WriteString(consolidated.ContentMd)
	}

	messages := []AIMessage{
		{Role: "system", Content: SystemPromptChat},
		{Role: "user", Content: UserDataMarker + "\n\n" + ctxBlock.String()},
	}
	for _, m := range history {
		if m.Role == RoleAssistant {
			messages = append(messages, AIMessage{Role: "assistant", Content: m.Content})
			continue
		}
		messages = append(messages, AIMessage{Role: "user", Content: m.Content})
	}
	return messages
}

func splitProposedConsolidated(text string) (content, proposed string) {
	start := strings.Index(text, ProposedOpenMarker)
	if start < 0 {
		return text, ""
	}
	end := strings.Index(text, ProposedCloseMarker)
	if end < 0 || end <= start {
		return text, ""
	}
	proposed = strings.TrimSpace(text[start+len(ProposedOpenMarker) : end])
	content = strings.TrimSpace(text[:start] + text[end+len(ProposedCloseMarker):])
	return content, proposed
}

func manualTag(isManual bool) string {
	if isManual {
		return " (manual)"
	}
	return ""
}

type snapshotEntry struct {
	key  string
	snap ObjectSnapshot
}

var orderedCategories = []string{"jira_issue", "jira_comment", "slack_thread", "context_file"}

func groupByCategory(snap Snapshot) map[string][]snapshotEntry {
	out := map[string][]snapshotEntry{}
	for key, s := range snap {
		cat := CategoryOf(key)
		out[cat] = append(out[cat], snapshotEntry{key: key, snap: s})
	}
	for cat := range out {
		entries := out[cat]
		for i := 0; i < len(entries); i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].key < entries[i].key {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}
		out[cat] = entries
	}
	return out
}

// keep time import usable if needed in future manual insertions.
var _ = time.Time{}
