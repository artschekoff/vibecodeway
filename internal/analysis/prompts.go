package analysis

import (
	"fmt"
	"strings"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

// defaultContextUpdaterPrompt is the in-code fallback used when the
// context_updater row cannot be loaded from the DB (see loadPrompt). The
// canonical, editable copy lives in ai_prompts and is seeded/updated by
// migrations 069 and 078 — keep this constant in sync with 078's body.
const defaultContextUpdaterPrompt = `You are a context management assistant. The user has typed a free-form note about a project context. Your job is to propose a LIST of structured actions that reflect everything actionable in the note — not just one.

## Context key
{{context_key}}

## Current workflow status
{{current_status}}

## Available workflow statuses
{{workflow_statuses}}

## Repos already linked to this context
{{existing_repos}}

## Jira tickets already linked to this context
{{existing_tickets}}

## User's note
{{user_text}}

Break the note down into one action object per distinct change described. Use ONLY these action types:

1. "set_status" — change the workflow status
   {
     "action": "set_status",
     "statusSlug": "<slug from the available list>",
     "statusComment": "<reason for the change>",
     "description": "<human-readable preview, e.g. 'Set status to in_review'>"
   }

2. "add_event" — add a structured event to the context history
   {
     "action": "add_event",
     "eventType": "<one of the event types below>",
     "summary": "<1-2 sentence summary>",
     "description": "<human-readable preview, e.g. 'Add blocker_resolved: task A unblocked'>"
   }
   Available event types (use EXACTLY these strings):
   requirement_stated, requirement_confirmed, requirement_rejected,
   open_question_raised, open_question_answered, decision_made,
   blocker_identified, blocker_resolved, work_completed, status_update,
   next_step_recommendation, user_note

3. "add_ticket" — link one or more Jira issues to this context
   {
     "action": "add_ticket",
     "jiraKeys": ["<issue key>", "..."],
     "description": "<human-readable preview, e.g. 'Link ticket ABC-123'>"
   }
   Skip keys already present in "Jira tickets already linked to this context" above.

4. "add_repo" — link a VCS repository/branch to this context
   {
     "action": "add_repo",
     "platform": "<e.g. github>",
     "ownerRepo": "<owner/repo>",
     "branch": "<branch name>",
     "repoLabel": "<optional short label>",
     "description": "<human-readable preview, e.g. 'Link github.com/acme/widgets@main'>"
   }
   Skip repos already present in "Repos already linked to this context" above.

5. "add_relation" — relate this context to another context
   {
     "action": "add_relation",
     "targetContextKey": "<the other context's key>",
     "relationType": "<one of: blocks, is-blocked-by, relates-to, duplicates, is-duplicated-by>",
     "description": "<human-readable preview, e.g. 'Mark as is-blocked-by JH-42'>"
   }

Rules:
- Include one object per distinct change the user described. A single note may produce zero, one, or several actions.
- Do not invent actions the note does not support. If nothing is actionable, return [].
- Do not propose add_ticket/add_repo for items already linked (see the "already linked" sections above).
- If the note only contains a general remark with no clear structured action, propose a single add_event with eventType "user_note".

Respond with ONLY a JSON ARRAY of action objects (no markdown fences, no explanation, no wrapping object):
[
  { ... action object ... },
  { ... action object ... }
]`

// defaultPlanGenerationPrompt is the in-code fallback used when the
// plan_generation row cannot be loaded from the DB (see loadPrompt). The
// canonical, editable copy lives in ai_prompts and is seeded by migration
// 080 — keep this constant BYTE-IDENTICAL to that migration's body.
const defaultPlanGenerationPrompt = `You are a senior engineer writing a precise, writing-plans-style implementation plan. Ground every step in the assignment, the context, and the shown codebase. Use concrete file paths and inline ` + "`path:line`" + ` references from the Codebase context when present. Output GitHub-flavored markdown only.

## Context key
{{context_key}}

## Assignment
{{assignment_body}}

## Raw context content
{{raw_content}}
{{codebase_context}}
## Task
Write a step-by-step implementation plan for the assignment above. The plan must:
- Be grounded in the assignment, the raw context content, and the codebase context — do not invent requirements not supported by them
- Use concrete file paths and inline ` + "`path:line`" + ` references from the Codebase context section when it is non-empty
- Break the work into small, ordered, independently verifiable steps
- Call out open questions or blockers as a separate section when relevant

Output ONLY the plan as GitHub-flavored markdown — no preamble, no trailing commentary.`

// applyPlaceholders replaces all {{key}} tokens in tmpl with the values from vars.
func applyPlaceholders(tmpl string, vars map[string]string) string {
	for k, v := range vars {
		tmpl = strings.ReplaceAll(tmpl, "{{"+k+"}}", v)
	}
	return tmpl
}

// eventExtractionPrompt builds the prompt for extracting typed events from raw context content.
// tmpl is loaded from the DB (ai_prompts.body for id="event_extraction").
// prevSnapshot is included as a "previous state" section so the AI skips already-captured events.
func eventExtractionPrompt(tmpl, contextKey, rawContent, userNextStepsHints string, prevSnapshot *appstore.ContextSnapshot) string {
	nextSteps := ""
	if userNextStepsHints != "" {
		nextSteps = fmt.Sprintf("\n\n## User hints about next steps\n%s", userNextStepsHints)
	}
	snapshotSection := ""
	if prevSnapshot != nil {
		snapshotSection = fmt.Sprintf(
			"\n## Previous analysis state (already captured — do NOT re-extract as new events)\n%s\n\n%s\n\n%s\n",
			prevSnapshot.SectionOperational,
			prevSnapshot.SectionNarrative,
			prevSnapshot.SectionEngineering,
		)
	}
	return applyPlaceholders(tmpl, map[string]string{
		"context_key":       contextKey,
		"raw_content":       rawContent,
		"user_next_steps":   nextSteps,
		"previous_snapshot": snapshotSection,
	})
}

// snapshotBuildPrompt builds the prompt for generating the three-section snapshot from events.
// tmpl is loaded from the DB (ai_prompts.body for id="snapshot_build").
// When prevSnapshot is non-nil, its sections are injected as prior state so the AI only needs new events.
func snapshotBuildPrompt(tmpl, contextKey string, events []eventForPrompt, userNextSteps nextStepsForPrompt, statusDefs, currentStatusLogEntry string, prevSnapshot *appstore.ContextSnapshot) string {
	priorSnapshotSection := ""
	if prevSnapshot != nil {
		priorSnapshotSection = fmt.Sprintf(
			"## Prior snapshot (already-analyzed base — update with new events below)\n%s\n\n%s\n\n%s\n",
			prevSnapshot.SectionOperational,
			prevSnapshot.SectionNarrative,
			prevSnapshot.SectionEngineering,
		)
	}
	return applyPlaceholders(tmpl, map[string]string{
		"context_key":              contextKey,
		"prior_snapshot":           priorSnapshotSection,
		"events":                   formatEventsForPrompt(events),
		"next_steps_short":         userNextSteps.Short,
		"next_steps_mid":           userNextSteps.Mid,
		"next_steps_long":          userNextSteps.Long,
		"status_definitions":       statusDefs,
		"current_status_log_entry": currentStatusLogEntry,
	})
}

// inferredPacketPrompt builds the prompt for generating a structured business requirements packet.
// tmpl is loaded from the DB (ai_prompts.body for id="handoff_packet").
func inferredPacketPrompt(tmpl, contextKey string, events []eventForPrompt) string {
	return applyPlaceholders(tmpl, map[string]string{
		"context_key": contextKey,
		"events":      formatEventsForPrompt(events),
	})
}

type eventForPrompt struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Timestamp  string `json:"timestamp"`
	SourceType string `json:"source_type"`
	Author     string `json:"author"`
	Summary    string `json:"summary"`
}

type nextStepsForPrompt struct {
	Short string
	Mid   string
	Long  string
}

func formatEventsForPrompt(events []eventForPrompt) string {
	if len(events) == 0 {
		return "(no events yet)"
	}
	out := "[\n"
	for _, e := range events {
		out += fmt.Sprintf(`  {"id":"%s","type":"%s","ts":"%s","source":"%s","author":"%s","summary":%q},`+"\n",
			e.ID, e.Type, e.Timestamp, e.SourceType, e.Author, e.Summary)
	}
	out += "]"
	return out
}

// contextUpdaterPrompt builds the prompt for interpreting a user's free-form
// Quick Update text into a list of proposed actions. existingRepos and
// existingTickets are best-effort summaries of state already linked to the
// context (empty string when unavailable) so the AI can avoid proposing
// duplicates.
func contextUpdaterPrompt(tmpl, contextKey, currentStatus, userText string, workflowStatuses []string, existingRepos, existingTickets string) string {
	statusList := ""
	for _, s := range workflowStatuses {
		statusList += "- " + s + "\n"
	}
	if statusList == "" {
		statusList = "(none configured)"
	}
	if existingRepos == "" {
		existingRepos = "(none linked)"
	}
	if existingTickets == "" {
		existingTickets = "(none linked)"
	}
	return applyPlaceholders(tmpl, map[string]string{
		"context_key":       contextKey,
		"current_status":    currentStatus,
		"workflow_statuses": statusList,
		"user_text":         userText,
		"existing_repos":    existingRepos,
		"existing_tickets":  existingTickets,
	})
}
