package main

import (
	"testing"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterActiveItems_FiltersInactiveIssues(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		Issues: []jirastore.Issue{
			{ID: "i1", IsActive: true},
			{ID: "i2", IsActive: false},
		},
	}
	filtered := filterActiveItems(detail)
	require.Len(t, filtered.Issues, 1)
	assert.Equal(t, "i1", filtered.Issues[0].ID)
}

func TestFilterActiveItems_FiltersInactiveChannels(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		SlackChannels: []slackstore.SlackChannel{
			{ID: "ch1", IsActive: true},
			{ID: "ch2", IsActive: false},
		},
	}
	filtered := filterActiveItems(detail)
	require.Len(t, filtered.SlackChannels, 1)
	assert.Equal(t, "ch1", filtered.SlackChannels[0].ID)
}

func TestFilterActiveItems_FiltersInactiveThreads(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		SlackThreads: []slackstore.SlackThread{
			{ID: "t1", IsActive: true},
			{ID: "t2", IsActive: false},
		},
	}
	filtered := filterActiveItems(detail)
	require.Len(t, filtered.SlackThreads, 1)
	assert.Equal(t, "t1", filtered.SlackThreads[0].ID)
}

func TestFilterActiveItems_FiltersInactiveContextTexts(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		ContextTexts: []appstore.ContextText{
			{ID: "f1", IsActive: true},
			{ID: "f2", IsActive: false},
		},
	}
	filtered := filterActiveItems(detail)
	require.Len(t, filtered.ContextTexts, 1)
	assert.Equal(t, "f1", filtered.ContextTexts[0].ID)
}

func TestFilterActiveItems_FiltersInactiveDMConfigs(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		SlackDMConfigs: []slackstore.SlackDMConfig{
			{ID: "dm1", IsActive: true, Content: "active DM content"},
			{ID: "dm2", IsActive: false, Content: "inactive DM content"},
		},
	}
	filtered := filterActiveItems(detail)
	require.Len(t, filtered.SlackDMConfigs, 1, "inactive DM configs should be excluded")
	assert.Equal(t, "dm1", filtered.SlackDMConfigs[0].ID)
}

func TestFilterActiveItems_AllActive_NoneFiltered(t *testing.T) {
	detail := &jiraservice.ContextDetail{
		SlackDMConfigs: []slackstore.SlackDMConfig{
			{ID: "dm1", IsActive: true},
			{ID: "dm2", IsActive: true},
		},
	}
	filtered := filterActiveItems(detail)
	assert.Len(t, filtered.SlackDMConfigs, 2)
}

func TestFilterActiveItems_EmptyDMConfigs_NoError(t *testing.T) {
	detail := &jiraservice.ContextDetail{}
	filtered := filterActiveItems(detail)
	assert.Empty(t, filtered.SlackDMConfigs)
}
