package analysis

import (
	"testing"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/stretchr/testify/assert"
)

func TestFormatJiraIssuesForPrompt(t *testing.T) {
	issues := []jirastore.Issue{{
		JiraKey:     "JH-4264",
		Summary:     "Update topline card labels for revenue referral framing",
		Status:      "In Progress",
		IssueType:   "Task",
		Assignee:    "Artem Krivoshchekov",
		Description: "Rename topline card labels.",
		Comments: []jirastore.Comment{
			{Author: "Artem", Body: "Branch feat_JH-4264_api-update-topline-card-labels created in juna-api from main"},
		},
	}}
	out := formatJiraIssuesForPrompt(issues)
	assert.Contains(t, out, "[JH-4264] Update topline card labels for revenue referral framing")
	assert.Contains(t, out, "Status: In Progress")
	assert.Contains(t, out, "Type: Task")
	assert.Contains(t, out, "Assignee: Artem Krivoshchekov")
	assert.Contains(t, out, "Description: Rename topline card labels.")
	assert.Contains(t, out, "feat_JH-4264_api-update-topline-card-labels")
}

func TestFormatJiraIssuesForPrompt_Empty(t *testing.T) {
	assert.Equal(t, "", formatJiraIssuesForPrompt(nil))
}

func TestFormatJiraIssuesForPrompt_BlankStatus(t *testing.T) {
	out := formatJiraIssuesForPrompt([]jirastore.Issue{{JiraKey: "JH-1", Summary: "x"}})
	assert.Contains(t, out, "Status: unknown") // never emit a blank status
}
