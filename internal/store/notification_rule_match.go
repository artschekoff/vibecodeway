package store

import (
	"fmt"
	"strings"
)

// MatchesRule reports whether a pipeline event fires this rule. Comparison is
// case- and padding-insensitive because Jira statuses are workflow-defined
// prose ("In Review", "in review") and the user types whatever their board
// shows. A rule whose note is blank never matches — it has nothing to say.
func MatchesRule(r NotificationRule, eventType, newValue string) bool {
	if !r.Enabled || strings.TrimSpace(r.Note) == "" {
		return false
	}
	if r.EventType != eventType {
		return false
	}
	want := strings.TrimSpace(r.MatchValue)
	if want == "" {
		return true // "" = any new value of this event type
	}
	return strings.EqualFold(want, strings.TrimSpace(newValue))
}

// NoteForEvent returns the note of the first rule that matches, or "" when none
// does. Rules arrive in creation order, so the user's earliest rule wins.
//
// ponytail: one note per event. Two matching rules produce one notification
// carrying the first note, not two notifications — make it a list the first
// time somebody actually writes overlapping rules.
func NoteForEvent(rules []NotificationRule, eventType, newValue string) string {
	for _, r := range rules {
		if MatchesRule(r, eventType, newValue) {
			return r.Note
		}
	}
	return ""
}

// ConditionMetText is the generated first line of a rule's notification: what
// happened, in the user's terms. The note they wrote follows it.
func ConditionMetText(eventType, oldValue, newValue string) string {
	if eventType == "jira_status_changed" {
		if strings.TrimSpace(oldValue) != "" && strings.TrimSpace(newValue) != "" {
			return fmt.Sprintf("Condition met: Jira status %q → %q", oldValue, newValue)
		}
		if strings.TrimSpace(newValue) != "" {
			return fmt.Sprintf("Condition met: Jira status is now %q", newValue)
		}
		return "Condition met: Jira status changed"
	}
	label := strings.ReplaceAll(eventType, "_", " ")
	if strings.TrimSpace(newValue) != "" {
		return fmt.Sprintf("Condition met: %s → %q", label, newValue)
	}
	return fmt.Sprintf("Condition met: %s", label)
}
