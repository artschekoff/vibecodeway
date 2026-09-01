package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func matchRule(matchValue, note string) NotificationRule {
	return NotificationRule{
		EventType:  "jira_status_changed",
		MatchValue: matchValue,
		Note:       note,
		Enabled:    true,
	}
}

func TestMatchesRule(t *testing.T) {
	assert.True(t, MatchesRule(matchRule("Done", "n"), "jira_status_changed", "Done"))

	// Jira status casing and padding vary by workflow; the user types what they see.
	assert.True(t, MatchesRule(matchRule("done", "n"), "jira_status_changed", "DONE"))
	assert.True(t, MatchesRule(matchRule("  Done  ", "n"), "jira_status_changed", "Done"))

	// An empty match value is "any change of this event type".
	assert.True(t, MatchesRule(matchRule("", "n"), "jira_status_changed", "In Review"))

	assert.False(t, MatchesRule(matchRule("Done", "n"), "jira_status_changed", "In Review"))
	assert.False(t, MatchesRule(matchRule("Done", "n"), "pr_merged", "Done"), "event type must match")

	disabled := matchRule("Done", "n")
	disabled.Enabled = false
	assert.False(t, MatchesRule(disabled, "jira_status_changed", "Done"))

	assert.False(t, MatchesRule(matchRule("Done", "   "), "jira_status_changed", "Done"),
		"a rule with no note has nothing to say")
}

func TestNoteForEvent(t *testing.T) {
	rules := []NotificationRule{
		matchRule("In Review", "ping the reviewer"),
		matchRule("Done", "tell QA we are ready"),
		matchRule("Done", "second Done rule"),
	}

	assert.Equal(t, "tell QA we are ready", NoteForEvent(rules, "jira_status_changed", "Done"),
		"first matching rule wins")
	assert.Equal(t, "ping the reviewer", NoteForEvent(rules, "jira_status_changed", "In Review"))
	assert.Equal(t, "", NoteForEvent(rules, "jira_status_changed", "Blocked"))
	assert.Equal(t, "", NoteForEvent(nil, "jira_status_changed", "Done"))
}

func TestConditionMetText(t *testing.T) {
	assert.Equal(t, `Condition met: Jira status "To Do" → "Done"`,
		ConditionMetText("jira_status_changed", "To Do", "Done"))

	// A first sync has no previous status to show.
	assert.Equal(t, `Condition met: Jira status is now "Done"`,
		ConditionMetText("jira_status_changed", "", "Done"))

	assert.Equal(t, `Condition met: pr merged → "merged"`,
		ConditionMetText("pr_merged", "", "merged"))

	assert.Equal(t, "Condition met: pr merged", ConditionMetText("pr_merged", "", ""))
}
