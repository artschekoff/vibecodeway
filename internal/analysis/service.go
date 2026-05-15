// Package analysis implements the AI-powered event extraction and snapshot building pipeline.
// It is triggered manually by the user via the "Analyze" button in the UI.
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/openai"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"go.uber.org/zap"
)

// defaultXxxPrompt constants mirror the seed values in migration 050_ai_prompts.sql.
// They are used as fallbacks when the DB row is unavailable (e.g. during tests without a seeded DB).
const (
	defaultEventExtractionPrompt = `You are an AI analyst extracting structured events from project communication data.

## Context
Project/module key: {{context_key}}
{{previous_snapshot}}
## New source data (since last analysis)
{{raw_content}}{{user_next_steps}}

## Task
Extract a JSON array of events. Each event represents a distinct, meaningful fact.
Skip noise (greetings, off-topic messages, duplicate information).

## Event types (use exactly these strings):
- requirement_stated      — official requirement from Jira or formal doc
- requirement_confirmed   — informal requirement confirmed by authorised person
- requirement_rejected    — requirement explicitly cancelled
- open_question_raised    — unresolved question or ambiguity
- open_question_answered  — previously open question answered
- decision_made           — technical or business decision taken
- blocker_identified      — blocker or problem found
- blocker_resolved        — blocker removed
- work_completed          — task/step done (PR merged, ticket closed, confirmed in chat)
- status_update           — general progress update
- next_step_recommendation — explicit recommendation about what should happen next
- user_note               — contextual note from a team member worth preserving

## Source types:
jira | slack | github | chat | user

## Output format — return ONLY valid JSON, no markdown fences:
[
  {
    "type": "requirement_stated",
    "timestamp": "2024-01-15T10:30:00Z",
    "source_type": "jira",
    "source_id": "PROJ-123",
    "author": "username",
    "summary": "One or two sentence summary of what happened.",
    "raw_reference": "Original text or URL"
  }
]`

	defaultSnapshotBuildPrompt = `You are an AI analyst maintaining a structured project snapshot.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## User-provided next-step hints
Short term (~1 day): {{next_steps_short}}
Mid term (~1 week): {{next_steps_mid}}
Long term (~1 month): {{next_steps_long}}

## Task
Produce a JSON object with these exact fields. All sections are Markdown.

{
  "current_step": {
    "description": "What is being worked on RIGHT NOW (1-2 sentences)",
    "confidence": "high|medium|low",
    "from_event_ids": ["event-id-1", "event-id-2"]
  },
  "status": "not_started|in_progress|scoping|needs_attention|ready",
  "section_operational": "## Current status\n...\n## Active blockers\n...\n## Open questions\n...\n## Who is doing what\n...",
  "section_narrative": "## History\n...\n## Decisions and rationale\n...\n## What was tried\n...\n## Domain entities\n...\n## Constraints\n...",
  "section_engineering": "## Technical entities\n...\n## Relevant modules\n...\n## Active branch and PRs\n...\n## Known issues\n...\n## Architectural constraints\n...",
  "ai_next_steps_short": "Concrete actions for the next ~1 day",
  "ai_next_steps_mid": "Goals for the next ~1 week",
  "ai_next_steps_long": "Strategic direction for the next ~1 month",
  "estimated_hours": 3.5,
  "estimation_confidence": "high|medium|low",
  "estimation_basis": "One sentence explaining the estimate (e.g. '3 requirements at ~1h each, 0.5h already done')"
}

Rules:
- status "needs_attention" wins over all other statuses if there are open blockers or CI failures
- status "scoping" if there are unresolved open questions with no blocker
- Incorporate user-provided next-step hints when generating ai_next_steps_* fields
- estimated_hours is the TOTAL remaining work hours from NOW (not elapsed, not total effort)
- estimation_confidence: "high" if requirements are well-defined and scope is clear; "medium" if some uncertainty; "low" if many open questions
- If there are no events yet, omit estimated_hours from the response
- Return ONLY valid JSON, no markdown fences`

	defaultHandoffPacketPrompt = `You are an expert business analyst extracting structured business requirements from project communication logs.
Do NOT reference code files, repositories, function names, or database schema — focus entirely on BUSINESS requirements, rules, goals, and decisions.

## Context key: {{context_key}}

## Full event log (chronological)
{{events}}

## Task
Produce a JSON object with exactly this structure. Return ONLY valid JSON, no markdown fences.

{
  "summary": { "product_or_initiative": null, "business_problem": null, "target_users": [], "desired_outcomes": [], "non_goals": [], "inferred_tradeoffs": [], "global_confidence": 0.8 },
  "inferred_requirements": [],
  "handoff_candidate": null,
  "extraction_notes": null
}`

	defaultTaskAssignmentPrompt = `You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.`

	defaultTaskAssignmentRefinePrompt = `You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.`
)

// Service orchestrates the full analysis pipeline for one context.
type Service struct {
	store  *appstore.Store
	jiraSvc *jiraservice.ContextService
	ai     *openai.Client
	logger *zap.Logger
}

// NewService creates an analysis service.
func NewService(store *appstore.Store, jiraSvc *jiraservice.ContextService, ai *openai.Client, logger *zap.Logger) *Service {
	return &Service{store: store, jiraSvc: jiraSvc, ai: ai, logger: logger}
}

// computeCompletionDate adds hours to an RFC3339 base time string and returns RFC3339.
// Returns base unchanged if parsing fails or hours is 0.
func computeCompletionDate(base string, hours float64) string {
	if hours == 0 {
		return base
	}
	t, err := time.Parse(time.RFC3339, base)
	if err != nil {
		return base
	}
	return t.Add(time.Duration(hours * float64(time.Hour))).UTC().Format(time.RFC3339)
}

// estimationTriggerType returns "initial" for the first estimate, "re-analysis" for subsequent ones.
func estimationTriggerType(prev *appstore.ContextSnapshot) string {
	if prev == nil || prev.EstimatedHours == nil {
		return "initial"
	}
	return "re-analysis"
}

// AnalyzeContext runs a full analysis cycle for contextID:
//  1. Determines the delta (new data since last analysis).
//  2. Formats raw data into a text dump.
//  3. Calls the AI to extract typed events from the delta.
//  4. Appends the new events to the event log.
//  5. Rebuilds the snapshot from the complete event log.
//  6. Persists the snapshot and clears the stale flag.
func (s *Service) AnalyzeContext(ctx context.Context, contextID string) (*appstore.ContextSnapshot, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	snapshot, err := s.store.GetSnapshot(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}

	// Determine last analyzed session for delta.
	var lastSessionID string
	if snapshot != nil && snapshot.LastAnalyzedSessionID != nil {
		lastSessionID = *snapshot.LastAnalyzedSessionID
	}

	// Get the latest completed session to mark as "analyzed up to".
	sessions, err := s.store.ListSyncSessions(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list sync sessions: %w", err)
	}
	var latestSessionID string
	if len(sessions) > 0 {
		latestSessionID = sessions[0].ID // sessions are newest-first
	}

	// Determine delta start time so we only send new items to AI.
	var deltaStart time.Time
	if lastSessionID != "" {
		if sess, err := s.store.GetSyncSession(ctx, lastSessionID); err == nil {
			if t, parseErr := time.Parse(time.RFC3339, sess.StartedAt); parseErr == nil {
				deltaStart = t
			}
		}
	}

	// Format only items added since the last analysis for event extraction.
	var buf strings.Builder
	if err := s.jiraSvc.FormatContext(ctx, &buf, contextID, "", deltaStart, true); err != nil {
		s.logger.Warn("analysis: format context failed", zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := buf.String()

	// Pull existing user next-step hints for the prompt.
	userHints := nextStepsForPrompt{}
	if snapshot != nil {
		userHints = nextStepsForPrompt{
			Short: snapshot.UserNextStepsShort,
			Mid:   snapshot.UserNextStepsMid,
			Long:  snapshot.UserNextStepsLong,
		}
	}

	// 1. Extract new events from the delta content.
	if rawContent != "" {
		newEvents, err := s.extractEvents(ctx, ctxRow.Key, rawContent, userHints, snapshot)
		if err != nil {
			s.logger.Warn("analysis: event extraction failed", zap.String("contextID", contextID), zap.Error(err))
			// Continue — we can still rebuild snapshot from existing events.
		} else {
			// Tag each new event with contextID and the latest session.
			var sessionIDPtr *string
			if latestSessionID != "" {
				sessionIDPtr = &latestSessionID
			}
			for i := range newEvents {
				newEvents[i].ContextID = contextID
				newEvents[i].SyncSessionID = sessionIDPtr
			}
			if err := s.store.AddEvents(ctx, newEvents); err != nil {
				s.logger.Warn("analysis: add events failed", zap.String("contextID", contextID), zap.Error(err))
			}
		}
	}

	// 2. Load the full event log and rebuild the snapshot.
	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	newSnapshot, err := s.buildSnapshot(ctx, ctxRow.Key, allEvents, userHints)
	if err != nil {
		return nil, fmt.Errorf("build snapshot: %w", err)
	}

	// Preserve user-written next steps and override flag across rebuilds.
	newSnapshot.ContextID = contextID
	if snapshot != nil {
		newSnapshot.UserNextStepsShort = snapshot.UserNextStepsShort
		newSnapshot.UserNextStepsMid = snapshot.UserNextStepsMid
		newSnapshot.UserNextStepsLong = snapshot.UserNextStepsLong
		if snapshot.CurrentStepOverridden {
			newSnapshot.CurrentStepOverridden = true
			newSnapshot.CurrentStepDesc = snapshot.CurrentStepDesc
			newSnapshot.CurrentStepConfidence = "high"
		}
	}
	if latestSessionID != "" {
		newSnapshot.LastAnalyzedSessionID = &latestSessionID
	} else if lastSessionID != "" {
		newSnapshot.LastAnalyzedSessionID = &lastSessionID
	}
	newSnapshot.IsStale = false
	if snapshot != nil {
		newSnapshot.Version = snapshot.Version + 1
	}

	if err := s.store.UpsertSnapshot(ctx, newSnapshot); err != nil {
		return nil, fmt.Errorf("upsert snapshot: %w", err)
	}

	// Log estimation change when hours were produced.
	if newSnapshot.EstimatedHours != nil {
		analyzedAt := ""
		if newSnapshot.AnalyzedAt != nil {
			analyzedAt = *newSnapshot.AnalyzedAt
		}
		completionDate := computeCompletionDate(analyzedAt, *newSnapshot.EstimatedHours)
		newSnapshot.EstimatedCompletionDate = ptrStr(completionDate)

		triggerType := estimationTriggerType(snapshot)
		triggerDesc := ""
		if triggerType == "re-analysis" && snapshot != nil && snapshot.EstimatedHours != nil {
			delta := *newSnapshot.EstimatedHours - *snapshot.EstimatedHours
			if delta > 0 {
				triggerDesc = fmt.Sprintf("estimate increased by %.1fh", delta)
			} else if delta < 0 {
				triggerDesc = fmt.Sprintf("estimate decreased by %.1fh", -delta)
			} else {
				triggerDesc = "estimate unchanged"
			}
		}

		entry := appstore.EstimationLog{
			ContextID:           contextID,
			TriggerType:         triggerType,
			TriggerDescription:  triggerDesc,
			EstimationBasis:     newSnapshot.EstimationBasis,
			EstimatedHoursAfter: newSnapshot.EstimatedHours,
		}
		if newSnapshot.EstimatedCompletionDate != nil {
			entry.EstimatedCompletionAfter = newSnapshot.EstimatedCompletionDate
		}
		if snapshot != nil && snapshot.EstimatedHours != nil {
			entry.EstimatedHoursBefore = snapshot.EstimatedHours
			if snapshot.EstimatedCompletionDate != nil {
				entry.EstimatedCompletionBefore = snapshot.EstimatedCompletionDate
			}
			delta := *newSnapshot.EstimatedHours - *snapshot.EstimatedHours
			entry.DeltaHours = &delta
		}

		if logErr := s.store.AppendEstimationLog(ctx, entry); logErr != nil {
			s.logger.Warn("analysis: append estimation log failed",
				zap.String("contextID", contextID), zap.Error(logErr))
		}
		// Persist the computed completion date.
		if updateErr := s.store.UpsertSnapshot(ctx, newSnapshot); updateErr != nil {
			s.logger.Warn("analysis: update snapshot completion date failed",
				zap.String("contextID", contextID), zap.Error(updateErr))
		}
	}

	return newSnapshot, nil
}

// GenerateHandoffPacket calls the AI to produce a structured business requirements packet
// from the full event log, saves it to the database, and returns it.
// The packet contains inferred requirements with evidence and an optional handoff candidate.
// It intentionally excludes code references — business context only.
func (s *Service) GenerateHandoffPacket(ctx context.Context, contextID string) (*appstore.InferredRequirementsPacket, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	sessions, err := s.store.ListSyncSessions(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list sync sessions: %w", err)
	}
	var sessionID *string
	if len(sessions) > 0 {
		id := sessions[0].ID
		sessionID = &id
	}

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	var windowStart, windowEnd string
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID:         e.ID,
			Type:       e.Type,
			Timestamp:  e.Timestamp,
			SourceType: e.SourceType,
			Author:     e.Author,
			Summary:    e.Summary,
		})
		if windowStart == "" || e.Timestamp < windowStart {
			windowStart = e.Timestamp
		}
		if e.Timestamp > windowEnd {
			windowEnd = e.Timestamp
		}
	}

	tmpl := s.loadPrompt(ctx, "handoff_packet", defaultHandoffPacketPrompt)
	prompt := inferredPacketPrompt(tmpl, ctxRow.Key, promptEvents)
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("ai handoff packet: %w", err)
	}

	pkt, err := parsePacketResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse handoff packet: %w", err)
	}

	pkt.ContextID = contextID
	pkt.SyncSessionID = sessionID
	pkt.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	if windowStart != "" {
		pkt.ContextWindowStart = &windowStart
	}
	if windowEnd != "" {
		pkt.ContextWindowEnd = &windowEnd
	}

	if err := s.store.SavePacket(ctx, pkt); err != nil {
		s.logger.Warn("analysis: save packet failed", zap.String("contextID", contextID), zap.Error(err))
	}
	return pkt, nil
}

// GenerateTaskAssignments generates per-participant task assignment text from Jira issues
// and the event log, saves it to the DB, and returns the result.
func (s *Service) GenerateTaskAssignments(ctx context.Context, contextID string) (*appstore.TaskAssignment, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	var rawBuf strings.Builder
	if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil {
		s.logger.Warn("task assignment: format jira context failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := rawBuf.String()

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}

	tmpl := s.loadPrompt(ctx, "task_assignment", defaultTaskAssignmentPrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"context_key":         ctxRow.Key,
		"raw_content":         rawContent,
		"events":              formatEventsForPrompt(promptEvents),
		"custom_instructions": "",
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai task assignment: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ta := appstore.TaskAssignment{
		ContextID:   contextID,
		Content:     resp.Content,
		GeneratedAt: &now,
	}
	return s.store.UpsertTaskAssignment(ctx, ta)
}

// RefineTaskAssignments refines the existing task assignment document based on a user instruction.
func (s *Service) RefineTaskAssignments(ctx context.Context, contextID, userInstruction string) (*appstore.TaskAssignment, error) {
	existing, err := s.store.GetTaskAssignment(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get existing task assignment: %w", err)
	}
	currentContent := ""
	if existing != nil {
		currentContent = existing.Content
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	var rawBuf strings.Builder
	if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil {
		s.logger.Warn("task assignment refine: format jira context failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := rawBuf.String()

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}

	tmpl := s.loadPrompt(ctx, "task_assignment_refine", defaultTaskAssignmentRefinePrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"current_content":  currentContent,
		"user_instruction": userInstruction,
		"raw_content":      rawContent,
		"events":           formatEventsForPrompt(promptEvents),
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai task assignment refine: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ta := appstore.TaskAssignment{
		ContextID:   contextID,
		Content:     resp.Content,
		GeneratedAt: &now,
	}
	return s.store.UpsertTaskAssignment(ctx, ta)
}

// loadPrompt fetches a prompt template from the DB. Logs a warning and returns
// the fallback string on error so analysis can continue even if the DB row is missing.
func (s *Service) loadPrompt(ctx context.Context, id, fallback string) string {
	p, err := s.store.GetAIPrompt(ctx, id)
	if err != nil {
		s.logger.Warn("analysis: could not load prompt from DB, using built-in default",
			zap.String("promptID", id), zap.Error(err))
		return fallback
	}
	return p.Body
}

// extractEvents calls the AI to extract typed events from raw content.
// prevSnapshot is passed as prior-state context so the AI avoids re-extracting already-known events.
func (s *Service) extractEvents(ctx context.Context, contextKey, rawContent string, hints nextStepsForPrompt, prevSnapshot *appstore.ContextSnapshot) ([]appstore.Event, error) {
	tmpl := s.loadPrompt(ctx, "event_extraction", defaultEventExtractionPrompt)
	prompt := eventExtractionPrompt(tmpl, contextKey, rawContent, hints.Short+"\n"+hints.Mid+"\n"+hints.Long, prevSnapshot)
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("ai event extraction: %w", err)
	}
	return parseEventArray(resp.Content)
}

// buildSnapshot calls the AI to build the three-section snapshot from events.
func (s *Service) buildSnapshot(ctx context.Context, contextKey string, events []appstore.Event, hints nextStepsForPrompt) (*appstore.ContextSnapshot, error) {
	prompts := make([]eventForPrompt, 0, len(events))
	for _, e := range events {
		prompts = append(prompts, eventForPrompt{
			ID:         e.ID,
			Type:       e.Type,
			Timestamp:  e.Timestamp,
			SourceType: e.SourceType,
			Author:     e.Author,
			Summary:    e.Summary,
		})
	}

	tmpl := s.loadPrompt(ctx, "snapshot_build", defaultSnapshotBuildPrompt)
	prompt := snapshotBuildPrompt(tmpl, contextKey, prompts, hints)
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("ai snapshot build: %w", err)
	}
	return parseSnapshotResponse(resp.Content)
}

// parseEventArray parses a JSON array of events from the AI response.
func parseEventArray(content string) ([]appstore.Event, error) {
	content = extractJSON(content, '[')
	var raw []struct {
		Type         string `json:"type"`
		Timestamp    string `json:"timestamp"`
		SourceType   string `json:"source_type"`
		SourceID     string `json:"source_id"`
		Author       string `json:"author"`
		Summary      string `json:"summary"`
		RawReference string `json:"raw_reference"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse event array: %w (content: %.200s)", err, content)
	}
	events := make([]appstore.Event, 0, len(raw))
	for _, r := range raw {
		if r.Type == "" || r.Summary == "" {
			continue
		}
		ts := r.Timestamp
		if ts == "" {
			ts = time.Now().UTC().Format(time.RFC3339)
		}
		events = append(events, appstore.Event{
			Type:         r.Type,
			Timestamp:    ts,
			SourceType:   r.SourceType,
			SourceID:     r.SourceID,
			Author:       r.Author,
			Summary:      r.Summary,
			RawReference: r.RawReference,
		})
	}
	return events, nil
}

// parseSnapshotResponse parses the AI's snapshot JSON response.
func parseSnapshotResponse(content string) (*appstore.ContextSnapshot, error) {
	content = extractJSON(content, '{')
	var raw struct {
		CurrentStep struct {
			Description  string   `json:"description"`
			Confidence   string   `json:"confidence"`
			FromEventIDs []string `json:"from_event_ids"`
		} `json:"current_step"`
		Status             string `json:"status"`
		SectionOperational string `json:"section_operational"`
		SectionNarrative   string `json:"section_narrative"`
		SectionEngineering string `json:"section_engineering"`
		AINextStepsShort     string   `json:"ai_next_steps_short"`
		AINextStepsMid       string   `json:"ai_next_steps_mid"`
		AINextStepsLong      string   `json:"ai_next_steps_long"`
		EstimatedHours       *float64 `json:"estimated_hours"`
		EstimationConfidence string   `json:"estimation_confidence"`
		EstimationBasis      string   `json:"estimation_basis"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse snapshot response: %w (content: %.200s)", err, content)
	}

	fromIDs, _ := json.Marshal(raw.CurrentStep.FromEventIDs)
	confidence := raw.CurrentStep.Confidence
	if confidence == "" {
		confidence = "medium"
	}
	status := raw.Status
	if status == "" {
		status = "not_started"
	}

	return &appstore.ContextSnapshot{
		CurrentStepDesc:       raw.CurrentStep.Description,
		CurrentStepConfidence: confidence,
		CurrentStepFrom:       string(fromIDs),
		SectionOperational:    raw.SectionOperational,
		SectionNarrative:      raw.SectionNarrative,
		SectionEngineering:    raw.SectionEngineering,
		Status:                status,
		AINextStepsShort:      raw.AINextStepsShort,
		AINextStepsMid:        raw.AINextStepsMid,
		AINextStepsLong:       raw.AINextStepsLong,
		EstimatedHours:        raw.EstimatedHours,
		EstimationConfidence:  raw.EstimationConfidence,
		EstimationBasis:       raw.EstimationBasis,
	}, nil
}

// parsePacketResponse parses the AI's InferredRequirementsPacket JSON response.
func parsePacketResponse(content string) (*appstore.InferredRequirementsPacket, error) {
	content = extractJSON(content, '{')
	var pkt appstore.InferredRequirementsPacket
	if err := json.Unmarshal([]byte(content), &pkt); err != nil {
		return nil, fmt.Errorf("parse packet response: %w (content: %.300s)", err, content)
	}
	// Ensure slice fields are never nil so JSON output is consistent.
	if pkt.Requirements == nil {
		pkt.Requirements = []appstore.RequirementInference{}
	}
	for i := range pkt.Requirements {
		r := &pkt.Requirements[i]
		if r.SourceArtifacts == nil {
			r.SourceArtifacts = []appstore.ArtifactRef{}
		}
		if r.Evidence == nil {
			r.Evidence = []appstore.EvidenceSpan{}
		}
		if r.AffectedEntities == nil {
			r.AffectedEntities = []appstore.EntityRef{}
		}
		if r.Relations == nil {
			r.Relations = []appstore.RequirementRelation{}
		}
	}
	if pkt.Summary.TargetUsers == nil {
		pkt.Summary.TargetUsers = []string{}
	}
	if pkt.Summary.DesiredOutcomes == nil {
		pkt.Summary.DesiredOutcomes = []string{}
	}
	if pkt.Summary.NonGoals == nil {
		pkt.Summary.NonGoals = []string{}
	}
	if pkt.Summary.InferredTradeoffs == nil {
		pkt.Summary.InferredTradeoffs = []string{}
	}
	return &pkt, nil
}

// extractJSON finds the first occurrence of startChar ('{' or '[') in content
// and returns everything from that point. Strips markdown fences if present.
func extractJSON(content string, startChar byte) string {
	// Strip markdown code fences.
	if idx := strings.Index(content, "```"); idx != -1 {
		content = content[idx:]
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		if end := strings.Index(content, "```"); end != -1 {
			content = content[:end]
		}
	}
	if idx := strings.IndexByte(content, startChar); idx != -1 {
		content = content[idx:]
	}
	return strings.TrimSpace(content)
}

func ptrStr(s string) *string { return &s }

// Ensure io.Writer is imported (used by FormatContext).
var _ io.Writer = (*strings.Builder)(nil)
