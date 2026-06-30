package analysis

import (
	"fmt"
	"strings"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

const defaultContextUpdaterPrompt = "" // loaded from migration 069 — seed value is in DB

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
func snapshotBuildPrompt(tmpl, contextKey string, events []eventForPrompt, userNextSteps nextStepsForPrompt, statusDefs, currentStatusLogEntry string) string {
	return applyPlaceholders(tmpl, map[string]string{
		"context_key":              contextKey,
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

// contextUpdaterPrompt builds the prompt for interpreting a user's free-form Quick Update text.
func contextUpdaterPrompt(tmpl, contextKey, currentStatus, userText string, workflowStatuses []string) string {
	statusList := ""
	for _, s := range workflowStatuses {
		statusList += "- " + s + "\n"
	}
	if statusList == "" {
		statusList = "(none configured)"
	}
	return applyPlaceholders(tmpl, map[string]string{
		"context_key":       contextKey,
		"current_status":    currentStatus,
		"workflow_statuses": statusList,
		"user_text":         userText,
	})
}
