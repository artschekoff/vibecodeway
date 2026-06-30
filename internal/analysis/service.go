// Package analysis implements the AI-powered event extraction and snapshot building pipeline.
// It is triggered manually by the user via the "Analyze" button in the UI.
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/openai"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/viterin/vek/vek32"
	"go.uber.org/zap"
)

const (
	promptVersionIncrementalV1 = "pa_incremental_v1"
	promptVersionRefineV1      = "pa_refine_v1"
	promptVersionInitialV1     = "pa_initial_v1"
)

const summaryThreshold = 5

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

{{prior_snapshot}}
## New events since last analysis (chronological)
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

	defaultPersonAssignmentPrompt = `You are an AI analyst generating task assignments for each participant in a project.

## Context key: {{context_key}}

## Org team members
The following people are the confirmed members of this organisation. Generate an assignment entry for EACH of them (do not add extra people):
{{org_users}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
For each person listed in "Org team members" produce an entry in the JSON array below.
If a person has no relevant work found in the source data, still include them with a brief "No active tasks found" body.

The "body" field must be Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: which features, services, or domain areas they need to work in — named at the capability level (e.g. "authentication service", "notification feature") — do NOT name specific file paths, function names, class names, or directory paths
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Next concrete actions**: step-by-step tasks described as behaviors or outcomes (e.g. "add pagination to the user list") — do NOT reference specific file locations or function signatures
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]`

	defaultSinglePersonAssignmentPrompt = `You are a senior technical lead writing a task assignment for a single developer.
The document must be detailed enough for a coding agent to execute the work, but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.

## Context key: {{context_key}}

## Developer: {{person_name}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_prompt}}
## Task
Write a comprehensive task assignment for **{{person_name}} only**.
Focus exclusively on what this person owns or has commented on.

Include ALL sections below. Write "N/A" in any section with no data — do not omit sections.

### Current Assignment
- Exact Jira issue keys, summaries, and current statuses they own
- Acceptance criteria extracted from issue descriptions and comments

### Technical Context
- Which features, services, or domain areas to work in (named at the capability level, e.g. "authentication service", "dashboard feature")
- API endpoints, HTTP methods, request/response field names and types (only if found in the source data)
- Data entities, field names, validation rules, business logic constraints (only if found in the source data)
- Active branch name(s) and open PR numbers or URLs (if mentioned)
- Do NOT name specific file paths, directory paths, function names, class names, or module paths

### Implementation Steps
Numbered step-by-step guide. For each step describe:
1. What capability, behavior, or feature area to work on
2. What outcome the change should produce (describe the behavior — do NOT specify file locations or function signatures)
3. Expected observable result or acceptance signal

### Infrastructure & Configuration
- Environment variables and feature flags required (if found in source data)
- External services, queues, webhooks, or cron jobs to configure (if mentioned)
- Required DB migrations or deployment actions (if mentioned)

### Testing Requirements
- Behaviors and scenarios to cover with tests
- How to verify the full implementation is correct end-to-end

### Blockers & Dependencies
- Unanswered questions that block this work
- Other people's tasks this work depends on

Return ONLY the Markdown body — no JSON wrapper, no preamble.`

	defaultPersonAssignmentRefinePrompt = `You are refining a single person's task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve business and task details (Jira issue keys, acceptance criteria, blockers, deadlines, domain rules).
Do NOT add or preserve specific file paths, directory paths, function names, class names, or component paths.
Return ONLY the updated Markdown for this person's body — no JSON, no preamble.`

	defaultPersonAssignmentIncrementalPrompt = `You are updating a developer's task assignment based on new context.
Maximum output: {{max_tokens_hint}} words. Be surgical — only modify sections directly affected by the new messages.

## Developer: {{person_name}}

## Current Assignment
{{current_body}}

## New Messages (appeared since last update)
{{delta_content}}

## Recent Events
{{events}}

---
EXAMPLE (do not copy verbatim — illustrates edit style):

New message: "[Slack] @alice: API deadline moved to Friday, need auth endpoint done by EOD Thursday"

Before excerpt:
### Authentication Feature
- Implement login endpoint (timeline: next sprint)

After excerpt:
### Authentication Feature
- Implement login endpoint (**deadline: Thursday EOD** — Slack update from @alice)

---

Update the assignment. Preserve all unchanged sections verbatim. Do NOT add specific file paths, function names, class names, or directory paths.
Return ONLY the updated Markdown body.`

	defaultPersonAssignmentSummaryPrompt = `Compress the following task assignment into a concise summary (max 150 words).
Preserve: assignee name, key responsibilities, blockers, deadlines.
Discard: verbose explanations, unchanged background context.

Assignment:
{{body}}

Return ONLY the compressed summary — no preamble.`

	defaultPersonAssignmentJudgePrompt = `You are a code review judge evaluating a task assignment update.

## Updated Assignment
{{output_state}}

## Message that triggered this update
{{message_content}}

Score on three criteria (each 0–10, integer):
1. Completeness: does it mention all people and their roles?
2. Specificity: are tasks concrete (not vague like "handle X")?
3. Delta-accuracy: does it correctly incorporate the triggering message?

Respond in this exact format (nothing else):
SCORE: <avg 0.0-1.0 float>
NOTES: <one sentence explaining the lowest-scoring criterion>`
)

// Service orchestrates the full analysis pipeline for one context.
type Service struct {
	store    *appstore.Store
	jiraSvc  *jiraservice.ContextService
	ai       *openai.Client
	logger   *zap.Logger
	chainSvc *ReasoningChainService
	paJudge  *PersonAssignmentJudge
}

// NewService creates an analysis service.
func NewService(store *appstore.Store, jiraSvc *jiraservice.ContextService, ai *openai.Client, logger *zap.Logger) *Service {
	svc := &Service{
		store:    store,
		jiraSvc:  jiraSvc,
		ai:       ai,
		logger:   logger,
		chainSvc: NewReasoningChainService(store, logger),
	}
	svc.paJudge = &PersonAssignmentJudge{svc: svc}
	return svc
}

// PersonAssignmentProcessor runs one incremental or refine step of a person assignment chain.
type PersonAssignmentProcessor struct {
	svc *Service
}

func (p *PersonAssignmentProcessor) RunStep(ctx context.Context, inputState string, step *appstore.ReasoningChainStep) (string, error) {
	pa, err := p.svc.store.GetPersonAssignment(ctx, step.EntityID)
	if err != nil {
		return "", fmt.Errorf("get person assignment for processor: %w", err)
	}
	if pa == nil {
		return "", fmt.Errorf("person assignment %s not found", step.EntityID)
	}

	allEvents, _ := p.svc.store.ListEvents(ctx, pa.ContextID)
	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}
	eventsJSON, _ := json.Marshal(allEvents)
	step.EventsSnapshot = string(eventsJSON)

	eventsStr := formatEventsForPrompt(promptEvents)

	// Populate compressed summary when chain is getting long.
	steps, _ := p.svc.store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, step.EntityID)
	if len(steps) >= summaryThreshold && step.CompressedSummary == "" {
		if summary, err := p.svc.compressAssignmentBody(ctx, inputState); err == nil {
			step.CompressedSummary = summary
		}
	}

	var prompt string
	var promptVer string
	if step.SourceType == "instruction" {
		promptVer = promptVersionRefineV1
		// Retrieve only the top-15 most semantically relevant messages to stay token-efficient.
		relevantMsgs, semErr := p.svc.searchSemantic(ctx, pa.ContextID, step.UserInstruction, semanticSearchTopK)
		if semErr != nil || len(relevantMsgs) == 0 {
			// Fallback: retrieve all messages if semantic search fails.
			relevantMsgs, _ = p.svc.listMessages(ctx, pa.ContextID, time.Time{})
		}
		tmpl := p.svc.loadPrompt(ctx, "person_assignment_refine", defaultPersonAssignmentRefinePrompt)
		prompt = applyPlaceholders(tmpl, map[string]string{
			"person_name":      pa.Name,
			"current_body":     inputState,
			"user_instruction": step.UserInstruction,
			"raw_content":      formatMessages(relevantMsgs),
			"events":           eventsStr,
		})
	} else {
		promptVer = promptVersionIncrementalV1
		effectiveInput := inputState
		if step.CompressedSummary != "" {
			effectiveInput = step.CompressedSummary
		}
		tokenHint := "400"
		tmpl := p.svc.loadPrompt(ctx, "person_assignment_incremental", defaultPersonAssignmentIncrementalPrompt)
		prompt = applyPlaceholders(tmpl, map[string]string{
			"person_name":     pa.Name,
			"current_body":    effectiveInput,
			"delta_content":   step.MessageContent,
			"events":          eventsStr,
			"max_tokens_hint": tokenHint,
		})
	}

	step.PromptVersion = promptVer
	step.LLMPrompt = prompt

	resp, err := p.svc.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// PersonAssignmentJudge scores a completed person assignment step.
type PersonAssignmentJudge struct {
	svc *Service
}

func (j *PersonAssignmentJudge) Score(ctx context.Context, step appstore.ReasoningChainStep) (float64, string, error) {
	if step.OutputState == "" {
		return 0, "empty output", nil
	}
	tmpl := j.svc.loadPrompt(ctx, "person_assignment_judge", defaultPersonAssignmentJudgePrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"output_state":    step.OutputState,
		"message_content": step.MessageContent,
	})
	resp, err := j.svc.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return 0, "", fmt.Errorf("judge AI call: %w", err)
	}
	var score float64
	var notes string
	lines := strings.SplitN(resp.Content, "\n", 2)
	if len(lines) >= 1 {
		fmt.Sscanf(strings.TrimPrefix(strings.TrimSpace(lines[0]), "SCORE:"), "%f", &score)
	}
	if len(lines) >= 2 {
		notes = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[1]), "NOTES:"))
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score, notes, nil
}

// compressAssignmentBody calls the AI to produce a compressed summary of a person assignment body.
func (s *Service) compressAssignmentBody(ctx context.Context, body string) (string, error) {
	tmpl := s.loadPrompt(ctx, "person_assignment_summary", defaultPersonAssignmentSummaryPrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{"body": body})
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
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
	msgs, err := s.listMessages(ctx, contextID, deltaStart)
	if err != nil {
		s.logger.Warn("analysis: list messages failed", zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := formatMessages(msgs)

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
	var newEvents []appstore.Event
	if rawContent != "" {
		extracted, err := s.extractEvents(ctx, ctxRow.Key, rawContent, userHints, snapshot)
		if err != nil {
			s.logger.Warn("analysis: event extraction failed", zap.String("contextID", contextID), zap.Error(err))
			// Continue — we can still rebuild snapshot from existing events.
		} else {
			newEvents = extracted
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
	// On re-analysis, only send new events + prior snapshot — saves tokens vs. sending full history.
	// ponytail: falls back to allEvents on first analysis (no prior snapshot).
	eventsForBuild := allEvents
	if snapshot != nil && len(newEvents) > 0 {
		eventsForBuild = newEvents
	}

	// Load workflow statuses for dynamic status definitions
	workflowStatuses, err := s.store.ListWorkflowStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("load workflow statuses: %w", err)
	}
	var statusDefsB strings.Builder
	for _, st := range workflowStatuses {
		fmt.Fprintf(&statusDefsB, "- %s (%s): %s\n", st.Slug, st.Label, st.AIHint)
	}
	statusDefs := strings.TrimRight(statusDefsB.String(), "\n")

	// Load latest status log entry for prior-status context
	latestLog, err := s.store.GetLatestStatusLogEntry(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("load latest status log: %w", err)
	}
	currentStatusLogEntry := "No prior status entry."
	if latestLog != nil {
		currentStatusLogEntry = fmt.Sprintf("Status: %s — Reason: %s", latestLog.StatusSlug, latestLog.Comment)
	}

	newSnapshot, err := s.buildSnapshot(ctx, ctxRow.Key, eventsForBuild, userHints, statusDefs, currentStatusLogEntry, snapshot)
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

	// Append status log entry — record the AI's status decision + reason
	var prevLogID *string
	if latestLog != nil {
		prevLogID = &latestLog.ID
	}
	statusComment := newSnapshot.StatusComment
	if statusComment == "" {
		statusComment = "Status determined by AI analysis."
	}
	if _, logErr := s.store.AppendStatusLog(ctx, contextID, newSnapshot.Status, statusComment, "ai", prevLogID); logErr != nil {
		s.logger.Warn("analysis: append status log failed", zap.String("contextID", contextID), zap.Error(logErr))
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

// GeneratePersonAssignments generates per-person technical assignment records from Jira issues
// and the event log. Replaces all existing assignments for the context and returns the new set.
func (s *Service) GeneratePersonAssignments(ctx context.Context, contextID string) ([]appstore.PersonAssignment, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	orgUsers, err := s.store.ListUsers(ctx, ctxRow.OrgID)
	if err != nil {
		s.logger.Warn("person assignment: list org users failed",
			zap.String("contextID", contextID), zap.Error(err))
		// proceed with empty list — AI will still process raw content
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	allMsgs, err := s.listMessages(ctx, contextID, time.Time{})
	if err != nil {
		s.logger.Warn("person assignment: list messages failed",
			zap.String("contextID", contextID), zap.Error(err))
	}

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}

	tmpl := s.loadPrompt(ctx, "person_assignment", defaultPersonAssignmentPrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"context_key":         ctxRow.Key,
		"org_users":           formatOrgUsers(orgUsers),
		"raw_content":         formatMessages(allMsgs),
		"events":              formatEventsForPrompt(promptEvents),
		"custom_instructions": "",
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai person assignment: %w", err)
	}

	parsed, err := parsePersonAssignmentArray(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse person assignment response: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range parsed {
		parsed[i].GeneratedAt = &now
	}

	result, err := s.store.ReplacePersonAssignments(ctx, contextID, parsed)
	if err != nil {
		return nil, fmt.Errorf("replace person assignments: %w", err)
	}
	rawContent := formatMessages(allMsgs)
	eventsJSON, _ := json.Marshal(allEvents)
	eventsSnapshot := string(eventsJSON)
	for _, pa := range result {
		_, _ = s.store.AppendChainStep(ctx, appstore.ReasoningChainStep{
			EntityType:     appstore.ChainEntityPersonAssignment,
			EntityID:       pa.ID,
			SourceType:     "initial",
			MessageContent: rawContent,
			InputState:     "",
			OutputState:    pa.Body,
			PromptVersion:  promptVersionInitialV1,
			EventsSnapshot: eventsSnapshot,
			IsCurrent:      true,
		})
		_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, pa.ID, now)
	}
	return result, nil
}

// RefinePersonAssignment refines a single person's assignment body based on a user instruction.
// Returns the updated PersonAssignment.
func (s *Service) RefinePersonAssignment(ctx context.Context, contextID, personID, userInstruction string) (*appstore.PersonAssignment, error) {
	existing, err := s.store.GetPersonAssignment(ctx, personID)
	if err != nil {
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	currentBody := ""
	if existing != nil {
		currentBody = existing.Body
	}

	allMsgs, err := s.listMessages(ctx, contextID, time.Time{})
	if err != nil {
		s.logger.Warn("analysis: list messages failed", zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := formatMessages(allMsgs)

	processor := &PersonAssignmentProcessor{svc: s}
	newStep, err := s.chainSvc.AppendStep(ctx, appstore.ReasoningChainStep{
		EntityType:      appstore.ChainEntityPersonAssignment,
		EntityID:        personID,
		SourceType:      "instruction",
		UserInstruction: userInstruction,
		MessageContent:  rawContent,
		InputState:      currentBody,
	}, processor, s.paJudge)
	if err != nil {
		return nil, fmt.Errorf("append refine step: %w", err)
	}
	outputState, _ := s.store.SetChainCurrentStep(ctx, appstore.ChainEntityPersonAssignment, personID, newStep.ID)
	if outputState != "" {
		_ = s.store.UpdatePersonAssignmentBody(ctx, personID, outputState)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personID, now)
	return s.store.GetPersonAssignment(ctx, personID)
}

// GetContextParticipants returns sorted unique participant names (assignees + comment authors)
// from the Jira issues linked to the context.
func (s *Service) GetContextParticipants(ctx context.Context, contextID string) ([]string, error) {
	return s.jiraSvc.GetContextParticipants(ctx, contextID)
}

// MarkAssignmentsOutdated checks every PersonAssignment for pending messages (created after
// analyzed_through). Sets is_outdated=true on assignments with pending messages.
// Returns a map of personAssignmentID → pending message count.
// Does NOT call the AI — the user triggers re-generation explicitly.
func (s *Service) MarkAssignmentsOutdated(ctx context.Context, contextID string) (map[string]int, error) {
	assignments, err := s.store.ListPersonAssignments(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}

	result := make(map[string]int, len(assignments))
	for _, pa := range assignments {
		watermark := ""
		if pa.AnalyzedThrough != nil {
			watermark = *pa.AnalyzedThrough
		} else if pa.GeneratedAt != nil {
			watermark = *pa.GeneratedAt
		}
		if watermark == "" {
			result[pa.ID] = 0
			continue
		}

		pending, err := s.store.ListPendingMessages(ctx, contextID, watermark)
		if err != nil {
			s.logger.Warn("mark outdated: list pending failed", zap.String("pa", pa.ID), zap.Error(err))
			continue
		}

		result[pa.ID] = len(pending)
		if len(pending) > 0 && !pa.IsOutdated {
			_ = s.store.SetPersonAssignmentOutdated(ctx, pa.ID, true)
		}
	}
	return result, nil
}

// GetPendingMessages returns messages for a PersonAssignment that haven't been analyzed yet.
func (s *Service) GetPendingMessages(ctx context.Context, personAssignmentID string) ([]appstore.Message, error) {
	pa, err := s.store.GetPersonAssignment(ctx, personAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	if pa == nil {
		return nil, fmt.Errorf("person assignment %s not found", personAssignmentID)
	}
	watermark := ""
	if pa.AnalyzedThrough != nil {
		watermark = *pa.AnalyzedThrough
	} else if pa.GeneratedAt != nil {
		watermark = *pa.GeneratedAt
	}
	if watermark == "" {
		return nil, nil
	}
	return s.store.ListPendingMessages(ctx, pa.ContextID, watermark)
}

// RegeneratePersonAssignment takes the current assignment body plus all pending messages
// (created after analyzed_through) and produces a new step via the AI.
// The new step becomes is_current; is_outdated is cleared; analyzed_through advances.
func (s *Service) RegeneratePersonAssignment(ctx context.Context, personAssignmentID string) (*appstore.PersonAssignment, error) {
	pa, err := s.store.GetPersonAssignment(ctx, personAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	if pa == nil {
		return nil, fmt.Errorf("person assignment %s not found", personAssignmentID)
	}

	watermark := ""
	if pa.AnalyzedThrough != nil {
		watermark = *pa.AnalyzedThrough
	} else if pa.GeneratedAt != nil {
		watermark = *pa.GeneratedAt
	}

	var pendingMsgs []appstore.Message
	if watermark != "" {
		pendingMsgs, err = s.store.ListPendingMessages(ctx, pa.ContextID, watermark)
		if err != nil {
			return nil, fmt.Errorf("list pending messages: %w", err)
		}
	}
	if len(pendingMsgs) == 0 {
		_ = s.store.SetPersonAssignmentOutdated(ctx, personAssignmentID, false)
		return pa, nil
	}

	deltaContent := formatMessages(pendingMsgs)

	processor := &PersonAssignmentProcessor{svc: s}
	newStep, err := s.chainSvc.AppendStep(ctx, appstore.ReasoningChainStep{
		EntityType:     appstore.ChainEntityPersonAssignment,
		EntityID:       personAssignmentID,
		SourceType:     "message",
		MessageContent: deltaContent,
		InputState:     pa.Body,
	}, processor, s.paJudge)
	if err != nil {
		return nil, fmt.Errorf("append regeneration step: %w", err)
	}

	outputState, err := s.store.SetChainCurrentStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, newStep.ID)
	if err != nil {
		return nil, fmt.Errorf("set current step: %w", err)
	}
	if err := s.store.UpdatePersonAssignmentBody(ctx, personAssignmentID, outputState); err != nil {
		return nil, fmt.Errorf("sync assignment body: %w", err)
	}
	_ = s.store.SetPersonAssignmentOutdated(ctx, personAssignmentID, false)

	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personAssignmentID, now)

	return s.store.GetPersonAssignment(ctx, personAssignmentID)
}

// RebuildAssignmentChainFrom re-runs the reasoning chain from fromStepIndex.
// overrideContent replaces that step's original message_content when non-empty.
// All steps from fromStepIndex onward are deleted and re-created.
func (s *Service) RebuildAssignmentChainFrom(ctx context.Context, personAssignmentID string, fromStepIndex int, overrideContent string) (*appstore.PersonAssignment, error) {
	processor := &PersonAssignmentProcessor{svc: s}
	if err := s.chainSvc.RebuildFrom(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, fromStepIndex, overrideContent, processor, s.paJudge); err != nil {
		return nil, fmt.Errorf("rebuild chain: %w", err)
	}

	steps, err := s.store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list chain steps after rebuild: %w", err)
	}
	for _, st := range steps {
		if st.IsCurrent {
			if err := s.store.UpdatePersonAssignmentBody(ctx, personAssignmentID, st.OutputState); err != nil {
				return nil, fmt.Errorf("sync assignment body: %w", err)
			}
			break
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personAssignmentID, now)
	return s.store.GetPersonAssignment(ctx, personAssignmentID)
}

// GenerateEventsFromMessages extracts typed events from the message layer since the last event.
// Call after RefreshContext to keep the event log current without a full AnalyzeContext run.
func (s *Service) GenerateEventsFromMessages(ctx context.Context, contextID string) ([]appstore.Event, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	existing, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	var since time.Time
	for _, e := range existing {
		t, _ := time.Parse(time.RFC3339, e.Timestamp)
		if t.After(since) {
			since = t
		}
	}

	msgs, err := s.listMessages(ctx, contextID, since)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	if len(msgs) == 0 {
		return nil, nil
	}

	snapshot, _ := s.store.GetSnapshot(ctx, contextID)
	extracted, err := s.extractEvents(ctx, ctxRow.Key, formatMessages(msgs), nextStepsForPrompt{}, snapshot)
	if err != nil {
		return nil, fmt.Errorf("extract events: %w", err)
	}

	var saved []appstore.Event
	for _, e := range extracted {
		e.ContextID = contextID
		ev, saveErr := s.store.AddEvent(ctx, e)
		if saveErr != nil {
			s.logger.Warn("GenerateEventsFromMessages: save failed", zap.Error(saveErr))
			continue
		}
		saved = append(saved, *ev)
	}
	if len(saved) > 0 {
		_ = s.store.MarkSnapshotStale(ctx, contextID)
	}
	return saved, nil
}

// GenerateSinglePersonAssignment produces a focused, implementation-ready technical assignment
// for one named person. customPrompt is injected as additional context when non-empty.
// The result is upserted into the DB (create or update by contextID + personName).
func (s *Service) GenerateSinglePersonAssignment(ctx context.Context, contextID, personName, customPrompt string) (*appstore.PersonAssignment, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	allMsgs, err := s.listMessages(ctx, contextID, time.Time{})
	if err != nil {
		s.logger.Warn("analysis: list messages failed", zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := formatMessages(allMsgs)

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}

	customSection := ""
	if customPrompt != "" {
		customSection = "\n## Additional context\n" + customPrompt + "\n"
	}

	tmpl := s.loadPrompt(ctx, "person_assignment_single", defaultSinglePersonAssignmentPrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"context_key":   ctxRow.Key,
		"person_name":   personName,
		"raw_content":   rawContent,
		"events":        formatEventsForPrompt(promptEvents),
		"custom_prompt": customSection,
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai single person assignment: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	saved, err := s.store.UpsertPersonAssignment(ctx, contextID, appstore.PersonAssignment{
		Name:        personName,
		Body:        resp.Content,
		GeneratedAt: &now,
	})
	if err != nil {
		return nil, err
	}
	rawContent2 := formatMessages(allMsgs)
	eventsJSON2, _ := json.Marshal(allEvents)
	_, _ = s.store.AppendChainStep(ctx, appstore.ReasoningChainStep{
		EntityType:     appstore.ChainEntityPersonAssignment,
		EntityID:       saved.ID,
		SourceType:     "initial",
		MessageContent: rawContent2,
		InputState:     "",
		OutputState:    saved.Body,
		PromptVersion:  promptVersionInitialV1,
		EventsSnapshot: string(eventsJSON2),
		IsCurrent:      true,
	})
	now2 := time.Now().UTC().Format(time.RFC3339)
	_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, saved.ID, now2)
	return saved, nil
}

// PreviewAssignmentStep runs the AI for a PersonAssignment chain step without saving.
// Used by the propagation wizard in the UI. customInputState overrides the recorded input.
func (s *Service) PreviewAssignmentStep(ctx context.Context, personAssignmentID string, stepIndex int, customInputState string) (string, error) {
	processor := &PersonAssignmentProcessor{svc: s}
	return s.chainSvc.PreviewStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, customInputState, processor)
}

// CommitAssignmentStep saves a wizard-confirmed step to the PersonAssignment chain.
// Truncates from stepIndex onward, re-creates this step with the given states.
func (s *Service) CommitAssignmentStep(ctx context.Context, personAssignmentID string, stepIndex int, inputState, outputState string) (*appstore.ReasoningChainStep, error) {
	return s.chainSvc.CommitStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, inputState, outputState, "", s.paJudge)
}

// RunSingleAssignmentStep previews the AI output for one step without persisting anything.
// customInputState is the (possibly edited) output of the previous step.
func (s *Service) RunSingleAssignmentStep(ctx context.Context, personAssignmentID string, stepIndex int, customInputState string) (string, error) {
	processor := &PersonAssignmentProcessor{svc: s}
	return s.chainSvc.PreviewStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, customInputState, processor)
}

// CommitStepOutput saves a confirmed step output from the propagation wizard.
// Truncates from stepIndex onward, re-creates this step. If it's the last step,
// promotes it to is_current and syncs PersonAssignment.body.
func (s *Service) CommitStepOutput(ctx context.Context, personAssignmentID string, stepIndex int, inputState, outputState string) (*appstore.PersonAssignment, error) {
	saved, err := s.chainSvc.CommitStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, inputState, outputState, "", s.paJudge)
	if err != nil {
		return nil, fmt.Errorf("commit step: %w", err)
	}
	if saved.IsCurrent {
		if err := s.store.UpdatePersonAssignmentBody(ctx, personAssignmentID, saved.OutputState); err != nil {
			return nil, fmt.Errorf("sync assignment body: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personAssignmentID, now)
	}
	return s.store.GetPersonAssignment(ctx, personAssignmentID)
}

// SaveAssignmentOutput saves a manually edited output body as a new active version
// and syncs PersonAssignment.body atomically.
func (s *Service) SaveAssignmentOutput(ctx context.Context, personAssignmentID, body string) (*appstore.PersonAssignment, error) {
	pa, err := s.store.UpsertAssignmentBodyAndVersion(ctx, personAssignmentID, body, "")
	if err != nil {
		return nil, fmt.Errorf("save assignment output: %w", err)
	}
	return pa, nil
}

// RunAssignmentStepWithCustomPrompt runs the AI with customPrompt verbatim for the given step.
// Does NOT write to the DB — returns the AI output for preview only.
func (s *Service) RunAssignmentStepWithCustomPrompt(ctx context.Context, personAssignmentID string, stepIndex int, customPrompt string) (string, error) {
	steps, err := s.store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID)
	if err != nil {
		return "", fmt.Errorf("list steps: %w", err)
	}
	if stepIndex < 0 {
		return "", fmt.Errorf("step index must be non-negative")
	}
	if stepIndex >= len(steps) {
		return "", fmt.Errorf("step %d out of range (chain has %d steps)", stepIndex, len(steps))
	}
	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: customPrompt}})
	if err != nil {
		return "", fmt.Errorf("custom prompt AI call: %w", err)
	}
	return resp.Content, nil
}

// CommitAssignmentVersionFromCustomPrompt saves a new chain step using the given custom
// prompt and output, creating a new active version of the assignment.
func (s *Service) CommitAssignmentVersionFromCustomPrompt(ctx context.Context, personAssignmentID string, stepIndex int, customPrompt, outputState string) (*appstore.PersonAssignment, error) {
	steps, err := s.store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	if stepIndex < 0 {
		return nil, fmt.Errorf("step index must be non-negative")
	}
	if stepIndex >= len(steps) {
		return nil, fmt.Errorf("step %d out of range (chain has %d steps)", stepIndex, len(steps))
	}
	originalInputState := steps[stepIndex].InputState

	saved, err := s.chainSvc.CommitStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, originalInputState, outputState, customPrompt, s.paJudge)
	if err != nil {
		return nil, fmt.Errorf("commit step: %w", err)
	}
	if saved.IsCurrent {
		if _, err := s.store.UpsertAssignmentBodyAndVersion(ctx, personAssignmentID, saved.OutputState, customPrompt); err != nil {
			return nil, fmt.Errorf("save assignment version: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personAssignmentID, now)
	}
	return s.store.GetPersonAssignment(ctx, personAssignmentID)
}

// ListAssignmentVersions returns all version records for a PersonAssignment, newest first.
func (s *Service) ListAssignmentVersions(ctx context.Context, personAssignmentID string) ([]appstore.PersonAssignmentVersion, error) {
	return s.store.ListAssignmentVersions(ctx, personAssignmentID)
}

// InterpretContextUpdate parses userText using AI and returns a suggested action
// (add_event or set_status) for the given context.
func (s *Service) InterpretContextUpdate(ctx context.Context, contextID, userText string) (*appstore.ContextUpdateAction, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	statuses, err := s.store.ListWorkflowStatuses(ctx)
	if err != nil {
		// non-fatal: proceed without status list
		s.logger.Warn("context updater: list workflow statuses failed",
			zap.String("contextID", contextID), zap.Error(err))
		statuses = nil
	}

	slugLines := make([]string, 0, len(statuses))
	for _, st := range statuses {
		slugLines = append(slugLines, st.Slug+" ("+st.Label+")")
	}

	currentStatus := ctxRow.SnapshotStatus
	if currentStatus == "" {
		currentStatus = "not_started"
	}

	tmpl := s.loadPrompt(ctx, "context_updater", defaultContextUpdaterPrompt)
	prompt := contextUpdaterPrompt(tmpl, ctxRow.Key, currentStatus, userText, slugLines)

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai completion: %w", err)
	}

	var action appstore.ContextUpdateAction
	if err := json.Unmarshal([]byte(resp.Content), &action); err != nil {
		return nil, fmt.Errorf("parse ai response (raw: %.200s): %w", resp.Content, err)
	}
	if action.Action != "add_event" && action.Action != "set_status" {
		return nil, fmt.Errorf("unknown action %q from AI", action.Action)
	}
	return &action, nil
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
func (s *Service) buildSnapshot(ctx context.Context, contextKey string, events []appstore.Event, hints nextStepsForPrompt, statusDefs, currentStatusLogEntry string, prevSnapshot *appstore.ContextSnapshot) (*appstore.ContextSnapshot, error) {
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
	prompt := snapshotBuildPrompt(tmpl, contextKey, prompts, hints, statusDefs, currentStatusLogEntry, prevSnapshot)
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
		StatusComment      string `json:"status_comment"`
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
		StatusComment:         raw.StatusComment,
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

// parsePersonAssignmentArray parses the AI's JSON array of {name, body} objects.
func parsePersonAssignmentArray(content string) ([]appstore.PersonAssignment, error) {
	content = extractJSON(content, '[')
	var raw []struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal person assignments: %w (content: %.300s)", err, content)
	}
	result := make([]appstore.PersonAssignment, 0, len(raw))
	for _, r := range raw {
		if r.Name == "" {
			continue
		}
		result = append(result, appstore.PersonAssignment{Name: r.Name, Body: r.Body})
	}
	return result, nil
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

// listMessages fetches messages for contextID on or after since.
// Pass sourceIDs to filter by specific sources; pass none for all sources.
func (s *Service) listMessages(ctx context.Context, contextID string, since time.Time, sourceIDs ...string) ([]appstore.Message, error) {
	return s.store.ListMessagesSince(ctx, contextID, since, sourceIDs...)
}

const semanticSearchTopK = 15

// searchSemantic returns the top-K messages most semantically similar to queryText.
// For messages without a stored embedding, it generates and stores one lazily.
// Falls back to returning all messages if the embeddings API is unavailable.
func (s *Service) searchSemantic(ctx context.Context, contextID, queryText string, topK int) ([]appstore.Message, error) {
	// Semantic search only available with OpenAI provider.
	settings, _, err := s.ai.DescribeSettings(ctx)
	if err != nil || (settings.Provider != "" && settings.Provider != appstore.ProviderOpenAI && settings.Provider != appstore.ProviderLocal) {
		return s.listMessages(ctx, contextID, time.Time{})
	}

	allMsgs, err := s.listMessages(ctx, contextID, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(allMsgs) == 0 {
		return nil, nil
	}

	// Lazy-embed any messages that don't have an embedding yet.
	for _, msg := range allMsgs {
		ok, _ := s.store.HasEmbedding(ctx, msg.ID)
		if ok {
			continue
		}
		text := msg.Title + "\n" + msg.Body
		vec, embErr := s.ai.EmbedText(ctx, text)
		if embErr != nil {
			s.logger.Warn("embed message failed", zap.String("msg", msg.ID), zap.Error(embErr))
			// If embedding fails, fall back to returning all messages unranked.
			return allMsgs, nil
		}
		_ = s.store.UpsertMessageEmbedding(ctx, msg.ID, appstore.DefaultEmbeddingModel, vec)
	}

	queryVec, err := s.ai.EmbedText(ctx, queryText)
	if err != nil {
		s.logger.Warn("embed query failed — falling back to all messages", zap.Error(err))
		return allMsgs, nil
	}

	// Load all stored vectors for this context.
	vectors, err := s.store.ListContextEmbeddings(ctx, contextID)
	if err != nil {
		return allMsgs, nil
	}

	// Cosine rank using vek32.
	type scored struct {
		msg   appstore.Message
		score float32
	}
	var ranked []scored
	for _, msg := range allMsgs {
		vec, ok := vectors[msg.ID]
		if !ok {
			continue
		}
		sim := vek32.CosineSimilarity(queryVec, vec)
		ranked = append(ranked, scored{msg, sim})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if topK > len(ranked) {
		topK = len(ranked)
	}
	result := make([]appstore.Message, topK)
	for i := range result {
		result[i] = ranked[i].msg
	}
	return result, nil
}

// formatMessages renders messages into a text block for AI prompts.
// Children (parent_id != nil) are indented under their parent.
func formatMessages(msgs []appstore.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	byParent := make(map[string][]appstore.Message)
	var roots []appstore.Message
	for _, m := range msgs {
		if m.ParentID == nil {
			roots = append(roots, m)
		} else {
			byParent[*m.ParentID] = append(byParent[*m.ParentID], m)
		}
	}

	var b strings.Builder
	var write func(m appstore.Message, indent string)
	write = func(m appstore.Message, indent string) {
		author := m.AuthorDisplayName
		if author == "" {
			author = m.RawAuthor
		}
		if author == "" {
			author = "unknown"
		}
		label := m.SourceLabel
		if label == "" {
			label = strings.ToUpper(m.SourceID)
		}
		fmt.Fprintf(&b, "\n%s=== [%s] %s (by %s) ===\n", indent, label, m.CreatedAt, author)
		if m.Title != "" {
			fmt.Fprintf(&b, "%sTitle: %s\n", indent, m.Title)
		}
		if m.Body != "" {
			fmt.Fprintf(&b, "%s%s\n", indent, m.Body)
		}
		for _, child := range byParent[m.ID] {
			write(child, indent+"  ")
		}
	}
	for _, root := range roots {
		write(root, "")
	}
	return b.String()
}

// formatOrgUsers renders org users as a simple bulleted list for prompt injection.
func formatOrgUsers(users []appstore.User) string {
	if len(users) == 0 {
		return "(no org users configured)"
	}
	var b strings.Builder
	for _, u := range users {
		b.WriteString("- ")
		b.WriteString(u.DisplayName)
		b.WriteByte('\n')
	}
	return b.String()
}
