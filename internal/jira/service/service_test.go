package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// errSlackRunFailed is a sentinel returned by fakeSlackRunner to simulate failures.
var errSlackRunFailed = errors.New("slack run failed")

// fakeSlackRunner is a test double for slack.Runner that returns preconfigured output.
type fakeSlackRunner struct {
	output string
	err    error
}

func (f *fakeSlackRunner) Run(_ context.Context, _ []string, _ []string) (string, error) {
	return f.output, f.err
}

func openTestStore(t *testing.T) *appstore.Store {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestService(t *testing.T, runner SlackRunner) *ContextService {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	return NewContextService(s, nil, jirastore.NewRepo(s.DB()), slackstore.NewRepo(s.DB()), runner, logger)
}

// createCtx is a helper that creates a bare context in the store.
func createCtx(t *testing.T, svc *ContextService, key string) *appstore.Context {
	t.Helper()
	c, err := svc.store.CreateContext(context.Background(), key)
	require.NoError(t, err)
	return c
}

// --- SetJiraConfig ---

func TestSetJiraConfig_Success(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")

	jp, err := svc.SetJiraConfig(ctx, c.ID, "project = TEAM")
	require.NoError(t, err)
	assert.Equal(t, "project = TEAM", jp.SearchQuery)
	assert.Equal(t, c.ID, jp.ContextID)
}

func TestSetJiraConfig_UpdatesExisting(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")
	jp1, err := svc.SetJiraConfig(ctx, c.ID, "project = OLD")
	require.NoError(t, err)

	jp2, err := svc.SetJiraConfig(ctx, c.ID, "project = NEW")
	require.NoError(t, err)
	assert.Equal(t, jp1.ID, jp2.ID)
	assert.Equal(t, "project = NEW", jp2.SearchQuery)
}

func TestSetJiraConfig_EmptyQueryRejected(t *testing.T) {
	svc := newTestService(t, nil)
	c := createCtx(t, svc, "my-ctx")
	_, err := svc.SetJiraConfig(context.Background(), c.ID, "")
	require.Error(t, err)
}

// --- SetSlackConfig ---

func TestSetSlackConfig_Success(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")

	sp, err := svc.SetSlackConfig(ctx, c.ID, "acme", "deployment failed")
	require.NoError(t, err)
	assert.Equal(t, "acme", sp.Workspace)
	assert.Equal(t, "deployment failed", sp.SearchQuery)
}

func TestSetSlackConfig_EmptyWorkspaceRejected(t *testing.T) {
	svc := newTestService(t, nil)
	c := createCtx(t, svc, "my-ctx")
	_, err := svc.SetSlackConfig(context.Background(), c.ID, "", "query")
	require.Error(t, err)
}

// --- LoadSlackContext ---

func TestLoadSlackContext_Success(t *testing.T) {
	markdownOutput := `# Slack Context: "deployment failed" — #general

**Source:** #general | **Started:** 2024-01-01 | **Messages:** 5`

	svc := newTestService(t, &fakeSlackRunner{output: markdownOutput})
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")

	err := svc.LoadSlackContext(ctx, LoadSlackContextArgs{
		ContextID:   c.ID,
		Workspace:   "acme",
		ChannelID:   "C01234567",
		ThreadTS:    "1700000000.123456",
		ChannelName: "general",
		Permalink:   "https://acme.slack.com/archives/C01234567/p1700000000123456",
		SearchQuery: "deployment failed",
	})
	require.NoError(t, err)

	threads, err := svc.slackRepo.ListSlackThreads(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, markdownOutput, threads[0].Content)
	assert.Equal(t, "acme", threads[0].Workspace)
	assert.Equal(t, "C01234567", threads[0].ChannelID)
	assert.Equal(t, "1700000000.123456", threads[0].ThreadTS)
	assert.Equal(t, "general", threads[0].ChannelName)
}

func TestLoadSlackContext_RunnerError(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{err: errSlackRunFailed})
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")

	err := svc.LoadSlackContext(ctx, LoadSlackContextArgs{
		ContextID: c.ID, Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1",
	})
	require.Error(t, err)
}

func TestLoadSlackContext_ContextNotFound(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "some content"})

	err := svc.LoadSlackContext(context.Background(), LoadSlackContextArgs{
		ContextID: "non-existent", Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1",
	})
	require.Error(t, err)
}

// --- ListSlackThreads ---

func TestListSlackThreads_ReturnsAllThreads(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "content"})
	ctx := context.Background()

	c := createCtx(t, svc, "my-ctx")

	for _, ts := range []string{"1.1", "2.2"} {
		err := svc.LoadSlackContext(ctx, LoadSlackContextArgs{
			ContextID: c.ID, Workspace: "acme", ChannelID: "C1", ThreadTS: ts,
		})
		require.NoError(t, err)
	}

	threads, err := svc.ListSlackThreads(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, threads, 2)
}

// --- SearchSlackChannels ---

func TestSearchSlackChannels_Success(t *testing.T) {
	jsonOutput := `[{"id":"C01234567","name":"general"},{"id":"C89012345","name":"deploy-alerts"}]`
	svc := newTestService(t, &fakeSlackRunner{output: jsonOutput})

	results, err := svc.SearchSlackChannels(context.Background(), "acme", "*deploy*")
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "C01234567", results[0].ID)
	assert.Equal(t, "general", results[0].Name)
	assert.Equal(t, "C89012345", results[1].ID)
	assert.Equal(t, "deploy-alerts", results[1].Name)
}

func TestSearchSlackChannels_EmptyWorkspaceRejected(t *testing.T) {
	svc := newTestService(t, nil)
	_, err := svc.SearchSlackChannels(context.Background(), "", "*name*")
	require.Error(t, err)
}

func TestSearchSlackChannels_EmptyPatternRejected(t *testing.T) {
	svc := newTestService(t, nil)
	_, err := svc.SearchSlackChannels(context.Background(), "acme", "")
	require.Error(t, err)
}

func TestSearchSlackChannels_RunnerError(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{err: errSlackRunFailed})
	_, err := svc.SearchSlackChannels(context.Background(), "acme", "*deploy*")
	require.Error(t, err)
}

func TestSearchSlackChannels_InvalidJSON(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "not-json"})
	_, err := svc.SearchSlackChannels(context.Background(), "acme", "*deploy*")
	require.Error(t, err)
}

func TestSearchSlackChannels_EmptyResultList(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "[]"})
	results, err := svc.SearchSlackChannels(context.Background(), "acme", "*nomatches*")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- ListSlackWorkspaces ---

func TestListSlackWorkspaces_Success(t *testing.T) {
	// slack-cli list-workspaces emits one name per line (no JSON).
	lineOutput := "acme\ndevteam"
	svc := newTestService(t, &fakeSlackRunner{output: lineOutput})

	results, err := svc.ListSlackWorkspaces(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "acme", results[0].ID)
	assert.Equal(t, "acme", results[0].Name)
	assert.Equal(t, "devteam", results[1].ID)
	assert.Equal(t, "devteam", results[1].Name)
}

func TestListSlackWorkspaces_RunnerError(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{err: errSlackRunFailed})
	_, err := svc.ListSlackWorkspaces(context.Background())
	require.Error(t, err)
}

func TestListSlackWorkspaces_SingleWorkspace(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "acme"})
	results, err := svc.ListSlackWorkspaces(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "acme", results[0].Name)
}

func TestListSlackWorkspaces_EmptyOutput(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: ""})
	results, err := svc.ListSlackWorkspaces(context.Background())
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestListSlackWorkspaces_SkipsBlankLines(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "acme\n\ndevteam\n"})
	results, err := svc.ListSlackWorkspaces(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "acme", results[0].Name)
	assert.Equal(t, "devteam", results[1].Name)
}

// --- AddSlackChannels ---

func TestAddSlackChannels_Success(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "ch-ctx")

	err := svc.AddSlackChannels(ctx, c.ID, "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
		{ChannelID: "C02", ChannelName: "deploy-alerts"},
	})
	require.NoError(t, err)

	channels, err := svc.ListSlackChannels(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, channels, 2)
}

func TestAddSlackChannels_EmptyWorkspaceRejected(t *testing.T) {
	svc := newTestService(t, nil)
	c := createCtx(t, svc, "ch-ctx")
	err := svc.AddSlackChannels(context.Background(), c.ID, "", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
	})
	require.Error(t, err)
}

func TestAddSlackChannels_EmptyChannelsRejected(t *testing.T) {
	svc := newTestService(t, nil)
	c := createCtx(t, svc, "ch-ctx")
	err := svc.AddSlackChannels(context.Background(), c.ID, "acme", nil)
	require.Error(t, err)
}

func TestAddSlackChannels_ContextNotFound(t *testing.T) {
	svc := newTestService(t, nil)
	err := svc.AddSlackChannels(context.Background(), "nonexistent", "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
	})
	require.Error(t, err)
}

func TestAddSlackChannels_Idempotent(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "ch-ctx")
	input := []slackstore.SlackChannelInput{{ChannelID: "C01", ChannelName: "general"}}

	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", input))
	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", input))

	channels, err := svc.ListSlackChannels(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, channels, 1, "second upsert should not duplicate the channel")
}

// --- ListSlackChannels ---

func TestListSlackChannels_EmptyWhenNoneAdded(t *testing.T) {
	svc := newTestService(t, nil)
	c := createCtx(t, svc, "ch-ctx")

	channels, err := svc.ListSlackChannels(context.Background(), c.ID)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

// --- RemoveSlackChannel ---

func TestRemoveSlackChannel_Success(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "ch-ctx")
	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
	}))

	channels, err := svc.ListSlackChannels(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, channels, 1)

	require.NoError(t, svc.RemoveSlackChannel(ctx, channels[0].ID))

	channels, err = svc.ListSlackChannels(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func TestRemoveSlackChannel_NotFound(t *testing.T) {
	svc := newTestService(t, nil)
	err := svc.RemoveSlackChannel(context.Background(), "nonexistent")
	require.Error(t, err)
}

// --- GetContextDetail ---

func TestGetContextDetail_IncludesProviders(t *testing.T) {
	svc := newTestService(t, &fakeSlackRunner{output: "# thread"})
	ctx := context.Background()

	c := createCtx(t, svc, "full-ctx")
	_, err := svc.SetJiraConfig(ctx, c.ID, "project = F")
	require.NoError(t, err)
	_, err = svc.SetSlackConfig(ctx, c.ID, "acme", "q")
	require.NoError(t, err)
	err = svc.LoadSlackContext(ctx, LoadSlackContextArgs{
		ContextID: c.ID, Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1",
	})
	require.NoError(t, err)

	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, detail.Jira)
	assert.Equal(t, "project = F", detail.Jira.SearchQuery)
	require.NotNil(t, detail.Slack)
	assert.Equal(t, "acme", detail.Slack.Workspace)
	assert.Len(t, detail.SlackThreads, 1)
	assert.Equal(t, "# thread", detail.SlackThreads[0].Content)
}

func TestGetContextDetail_NoProvidersOmitted(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "bare-ctx")

	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	assert.Nil(t, detail.Jira)
	assert.Nil(t, detail.Slack)
	assert.Empty(t, detail.Issues)
	assert.Empty(t, detail.SlackThreads)
	assert.Empty(t, detail.SlackChannels)
}

func TestGetContextDetail_IncludesSlackChannels(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "channel-ctx")
	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
		{ChannelID: "C02", ChannelName: "deploy"},
	}))

	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, detail.SlackChannels, 2)

	ids := []string{detail.SlackChannels[0].ChannelID, detail.SlackChannels[1].ChannelID}
	assert.ElementsMatch(t, []string{"C01", "C02"}, ids)
	assert.Equal(t, "acme", detail.SlackChannels[0].Workspace)
}

// TestEditContext_UpdatesJiraAndSlack verifies the full edit-context workflow:
// updating the JQL query and replacing Slack channels on an existing context.
func TestEditContext_UpdatesJiraAndSlack(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "edit-ctx")

	// Initial setup: create a Jira config and two Slack channels.
	_, err := svc.SetJiraConfig(ctx, c.ID, "project = OLD")
	require.NoError(t, err)

	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C01", ChannelName: "general"},
		{ChannelID: "C02", ChannelName: "to-be-removed"},
	}))

	channels, err := svc.ListSlackChannels(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, channels, 2)

	// Edit: update JQL, remove one channel, add a new one.
	jp, err := svc.SetJiraConfig(ctx, c.ID, "project = NEW AND sprint in openSprints()")
	require.NoError(t, err)
	assert.Equal(t, "project = NEW AND sprint in openSprints()", jp.SearchQuery)

	// Remove the second channel.
	removeID := channels[1].ID
	require.NoError(t, svc.RemoveSlackChannel(ctx, removeID))

	// Add a brand new channel.
	require.NoError(t, svc.AddSlackChannels(ctx, c.ID, "acme", []slackstore.SlackChannelInput{
		{ChannelID: "C03", ChannelName: "new-channel"},
	}))

	// Verify final state via GetContextDetail.
	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, detail.Jira)
	assert.Equal(t, "project = NEW AND sprint in openSprints()", detail.Jira.SearchQuery)
	require.Len(t, detail.SlackChannels, 2)
	channelNames := []string{detail.SlackChannels[0].ChannelName, detail.SlackChannels[1].ChannelName}
	assert.Contains(t, channelNames, "general")
	assert.Contains(t, channelNames, "new-channel")
	assert.NotContains(t, channelNames, "to-be-removed")
}
