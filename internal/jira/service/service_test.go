package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// cmdRoutingRunner returns different output depending on the subcommand path.
type cmdRoutingRunner struct {
	outputs map[string]string
}

func (r *cmdRoutingRunner) Run(_ context.Context, subCmdPath []string, _ []string) (string, error) {
	if len(subCmdPath) > 0 {
		if out, ok := r.outputs[subCmdPath[0]]; ok {
			return out, nil
		}
	}
	return "", nil
}

func openTestStore(t *testing.T) *appstore.Store {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newTestService(t *testing.T, runner SlackRunner) *ContextService {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	return NewContextService(s, nil, jirastore.NewRepo(s.DB()), slackstore.NewRepo(s.DB()), runner, logger)
}

// createCtx is a helper that creates a bare context in the store.
// It reuses the first existing organization, or creates one named "test-org" on first call.
func createCtx(t *testing.T, svc *ContextService, key string) *appstore.Context {
	t.Helper()
	ctx := context.Background()
	orgs, err := svc.store.ListOrganizations(ctx)
	require.NoError(t, err)
	var orgID string
	if len(orgs) > 0 {
		orgID = orgs[0].ID
	} else {
		org, err := svc.store.CreateOrganization(ctx, "test-org")
		require.NoError(t, err)
		orgID = org.ID
	}
	c, err := svc.store.CreateContext(ctx, key, orgID)
	require.NoError(t, err)
	return c
}

// --- ListContexts filtering ---

func TestListContexts_DefaultExcludesArchived(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	createCtx(t, svc, "active-one")
	archived := createCtx(t, svc, "archived-one")
	_, err := svc.UpdateContext(ctx, archived.ID, archived.Key, "archived")
	require.NoError(t, err)

	list, err := svc.ListContexts(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "active-one", list[0].Key)
}

func TestListAllContexts_IncludesArchived(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	createCtx(t, svc, "active-one")
	archived := createCtx(t, svc, "archived-one")
	_, err := svc.UpdateContext(ctx, archived.ID, archived.Key, "archived")
	require.NoError(t, err)

	list, err := svc.ListAllContexts(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestToggleContextStatus(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "toggle-ctx")
	assert.Equal(t, "active", c.Status)

	toggled, err := svc.ToggleContextStatus(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", toggled.Status)

	toggled, err = svc.ToggleContextStatus(ctx, toggled.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", toggled.Status)
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

	threads, err := svc.slackRepo.ListSlackThreads(ctx, c.ID, false)
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

func TestGetContextDetail_IncludesSlackDMConfigs(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "dm-ctx")

	cfg, err := svc.slackRepo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2024-01-01")
	require.NoError(t, err)

	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, detail.SlackDMConfigs, 1)
	assert.Equal(t, cfg.ID, detail.SlackDMConfigs[0].ID)
	assert.Equal(t, "acme", detail.SlackDMConfigs[0].Workspace)
	assert.Equal(t, "2024-01-01", detail.SlackDMConfigs[0].StartFrom)
}

// TestGetContextDetail_NoProvidersOmitsDMConfigs verifies that an empty DM config list is excluded.
func TestGetContextDetail_NoProvidersOmitsDMConfigs(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	c := createCtx(t, svc, "bare-dm-ctx")

	detail, err := svc.GetContextDetail(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, detail.SlackDMConfigs)
}

// --- RefreshContext DM tracking ---

func TestRefreshContext_SeparatesDMsFromThreads(t *testing.T) {
	dmJSON := `[{"id":"D1","userId":"U1","userName":"alice","isIm":true,"messages":[{"user":"alice","text":"hello","timestamp":"2024-01-01 10:00"}]}]`
	threadContent := "# thread content"

	runner := &cmdRoutingRunner{
		outputs: map[string]string{
			"list-dms":     dmJSON,
			"load-context": threadContent,
		},
	}

	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	jRepo := jirastore.NewRepo(s.DB())
	sRepo := slackstore.NewRepo(s.DB())

	slackProv := NewSlackProvider(sRepo, runner, logger)
	svc := NewContextService(s, nil, jRepo, sRepo, runner, logger, slackProv)

	ctx := context.Background()
	c := createCtx(t, svc, "refresh-dm-ctx")

	_, err := sRepo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2024-01-01")
	require.NoError(t, err)

	_, err = sRepo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1", ChannelName: "general", Content: "old",
	})
	require.NoError(t, err)

	result, err := svc.RefreshContext(ctx, c.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, result.ThreadsRefreshed, "should count only threads")
	assert.Equal(t, 1, result.DMsRefreshed, "should count only DM configs")
}

func TestReloadAllContexts_SeparatesDMsFromThreads(t *testing.T) {
	dmJSON := `[{"id":"D1","userId":"U1","userName":"bob","isIm":true,"messages":[{"user":"bob","text":"hi","timestamp":"2024-02-01 09:00"}]}]`
	threadContent := "# reloaded thread"

	runner := &cmdRoutingRunner{
		outputs: map[string]string{
			"list-dms":     dmJSON,
			"load-context": threadContent,
		},
	}

	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	jRepo := jirastore.NewRepo(s.DB())
	sRepo := slackstore.NewRepo(s.DB())

	slackProv := NewSlackProvider(sRepo, runner, logger)
	svc := NewContextService(s, nil, jRepo, sRepo, runner, logger, slackProv)

	ctx := context.Background()
	c := createCtx(t, svc, "reload-dm-ctx")

	_, err := sRepo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2024-02-01")
	require.NoError(t, err)

	_, err = sRepo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C2", ThreadTS: "2.2", ChannelName: "deploy", Content: "old",
	})
	require.NoError(t, err)

	results, err := svc.ReloadAllContexts(ctx)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, 1, results[0].ThreadsRefreshed, "should count only threads")
	assert.Equal(t, 1, results[0].DMsRefreshed, "should count only DM configs")
}

// --- buildSlackItems DM output ---

func TestBuildSlackItems_DMsWrittenWithoutThreads(t *testing.T) {
	dmConfigs := []slackstore.SlackDMConfig{
		{ID: "dm1", Workspace: "acme", StartFrom: "2024-01-01", Content: "# DM with alice\n\n[2024-01-01] alice: hello"},
	}

	items := buildSlackItems(nil, dmConfigs, time.Time{})
	require.Len(t, items, 1)
	output := items[0].Body

	assert.Contains(t, output, "DMs (acme) since 2024-01-01", "DM label should be present")
	assert.Contains(t, output, "# DM with alice", "DM content should be present")
}

func TestBuildSlackItems_DMsWrittenAlongsideThreads(t *testing.T) {
	threads := []slackstore.SlackThread{
		{ID: "t1", Workspace: "acme", ChannelID: "C1", ChannelName: "general", Content: "thread content"},
	}
	dmConfigs := []slackstore.SlackDMConfig{
		{ID: "dm1", Workspace: "acme", Content: "dm content"},
	}

	items := buildSlackItems(threads, dmConfigs, time.Time{})
	require.Len(t, items, 2)

	combined := items[0].Body + items[1].Body
	assert.Contains(t, combined, "thread content")
	assert.Contains(t, combined, "dm content")
}

// --- formatDMContent ---

func TestFormatDMContent_PluralmessagesField(t *testing.T) {
	raw := `[{"id":"D1","userId":"U1","userName":"alice","isIm":true,"messages":[{"user":"alice","text":"hello","timestamp":"2024-01-01 10:00"}]}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with alice")
	assert.Contains(t, got, "alice: hello")
}

func TestFormatDMContent_LastMessageField_GroupDM(t *testing.T) {
	raw := `[{"id":"C01","name":"mpdm-alice--bob-1","isIm":false,"lastMessage":{"userId":"U123","text":"hey there","timestamp":"2024-01-15 09:30 UTC"}}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with mpdm-alice--bob-1", "should use name field for group DMs")
	assert.Contains(t, got, "hey there", "should render message text")
	assert.Contains(t, got, "2024-01-15 09:30 UTC", "should render timestamp")
}

func TestFormatDMContent_LastMessageField_DirectDM(t *testing.T) {
	raw := `[{"id":"D01","userId":"U1","userName":"alice","isIm":true,"lastMessage":{"userId":"U1","text":"hi from alice","timestamp":"2024-02-10 11:00 UTC"}}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with alice", "should use userName for 1:1 DMs")
	assert.Contains(t, got, "hi from alice")
}

func TestFormatDMContent_LastMessageField_MultipleConversations(t *testing.T) {
	raw := `[
		{"id":"C01","name":"mpdm-alice--bob-1","isIm":false,"lastMessage":{"userId":"U1","text":"group msg","timestamp":"2024-01-15 UTC"}},
		{"id":"D01","userId":"U2","userName":"carol","isIm":true,"lastMessage":{"userId":"U2","text":"direct msg","timestamp":"2024-01-16 UTC"}}
	]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "group msg")
	assert.Contains(t, got, "direct msg")
}

func TestFormatDMContent_NoLastMessageAndNoMessages_ShowsPlaceholder(t *testing.T) {
	raw := `[{"id":"C01","name":"mpdm-alice--bob-1","isIm":false}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with mpdm-alice--bob-1")
	assert.Contains(t, got, "no messages")
}

func TestFormatDMContent_SortsNewestFirst(t *testing.T) {
	raw := `[
		{"id":"D1","userName":"alice","isIm":true,"lastMessage":{"userId":"U1","text":"older","timestamp":"2024-01-01 10:00 UTC"}},
		{"id":"D2","userName":"bob","isIm":true,"lastMessage":{"userId":"U2","text":"newest","timestamp":"2024-03-15 12:00 UTC"}},
		{"id":"D3","userName":"carol","isIm":true,"lastMessage":{"userId":"U3","text":"middle","timestamp":"2024-02-10 09:00 UTC"}}
	]`
	got := formatDMContent(raw)
	bobPos := strings.Index(got, "# DM with bob")
	carolPos := strings.Index(got, "# DM with carol")
	alicePos := strings.Index(got, "# DM with alice")
	assert.True(t, bobPos < carolPos, "bob (newest) should appear before carol")
	assert.True(t, carolPos < alicePos, "carol (middle) should appear before alice (oldest)")
}

func TestFormatDMContent_EntriesWithoutMessages_SortedLast(t *testing.T) {
	raw := `[
		{"id":"D1","userName":"alice","isIm":true,"lastMessage":{"userId":"U1","text":"hi","timestamp":"2024-01-01 10:00 UTC"}},
		{"id":"D2","userName":"silent","isIm":true}
	]`
	got := formatDMContent(raw)
	alicePos := strings.Index(got, "# DM with alice")
	silentPos := strings.Index(got, "# DM with silent")
	assert.True(t, alicePos < silentPos, "entry with message should appear before entry without")
}

func TestFormatDMContent_SingularMessageField(t *testing.T) {
	raw := `[{"id":"D1","userId":"U1","userName":"alice","isIm":true,"message":{"user":"alice","text":"hello world","timestamp":"2024-01-15 09:30"}}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with alice", "should render DM header with user name")
	assert.Contains(t, got, "alice: hello world", "should render message text")
	assert.Contains(t, got, "2024-01-15 09:30", "should render timestamp")
}

func TestFormatDMContent_SingularMessageField_MultipleConversations(t *testing.T) {
	raw := `[
		{"id":"D1","userId":"U1","userName":"alice","isIm":true,"message":{"user":"alice","text":"hey","timestamp":"2024-01-15"}},
		{"id":"D2","userId":"U2","userName":"bob","isIm":true,"message":{"user":"bob","text":"sup","timestamp":"2024-01-16"}}
	]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with alice")
	assert.Contains(t, got, "alice: hey")
	assert.Contains(t, got, "# DM with bob")
	assert.Contains(t, got, "bob: sup")
}

func TestFormatDMContent_NoMessage_ShowsPlaceholder(t *testing.T) {
	raw := `[{"id":"D1","userId":"U1","userName":"alice","isIm":true}]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "# DM with alice", "DM entry should still appear even with no messages")
	assert.Contains(t, got, "no messages", "placeholder text should be shown when no messages exist")
}

func TestFormatDMContent_MixedSingularAndPlural(t *testing.T) {
	raw := `[
		{"id":"D1","userId":"U1","userName":"alice","isIm":true,"message":{"user":"alice","text":"singular","timestamp":"2024-01-15"}},
		{"id":"D2","userId":"U2","userName":"bob","isIm":true,"messages":[{"user":"bob","text":"plural","timestamp":"2024-01-16"}]}
	]`
	got := formatDMContent(raw)
	assert.Contains(t, got, "alice: singular")
	assert.Contains(t, got, "bob: plural")
}

func TestRefreshContext_SavesDMContent_LastMessageField(t *testing.T) {
	dmJSON := `[{"id":"D1","userId":"U1","userName":"alice","isIm":true,"lastMessage":{"userId":"U1","text":"hello from refresh","timestamp":"2024-03-01 10:00 UTC"}}]`

	runner := &cmdRoutingRunner{
		outputs: map[string]string{
			"list-dms": dmJSON,
		},
	}

	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	sRepo := slackstore.NewRepo(s.DB())
	slackProv := NewSlackProvider(sRepo, runner, logger)
	svc := NewContextService(s, nil, jirastore.NewRepo(s.DB()), sRepo, runner, logger, slackProv)

	ctx := context.Background()
	c := createCtx(t, svc, "refresh-dm-lastmsg")

	cfg, err := sRepo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2024-01-01")
	require.NoError(t, err)
	require.Empty(t, cfg.Content, "content should be empty before refresh")

	result, err := svc.RefreshContext(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.DMsRefreshed, "DM should be counted as refreshed")

	configs, err := sRepo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Contains(t, configs[0].Content, "# DM with alice", "content should be saved to DB")
	assert.Contains(t, configs[0].Content, "hello from refresh")
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

func TestJiraLinkToRelationType(t *testing.T) {
	cases := []struct {
		typeName  string
		direction string
		want      string
	}{
		{"Blocks", "outward", "blocks"},
		{"Blocks", "inward", "is-blocked-by"},
		{"is blocked by", "outward", "blocks"},
		{"is blocked by", "inward", "is-blocked-by"},
		{"Duplicates", "outward", "duplicates"},
		{"Duplicates", "inward", "is-duplicated-by"},
		{"duplicate issue", "outward", "duplicates"},
		{"Relates", "outward", "relates-to"},
		{"Relates", "inward", "relates-to"},
		{"cloners", "outward", "relates-to"},
		{"Problem/Incident", "inward", "relates-to"},
		{"", "outward", "relates-to"},
	}
	for _, tc := range cases {
		got := jiraLinkToRelationType(tc.typeName, tc.direction)
		assert.Equal(t, tc.want, got, "typeName=%q direction=%q", tc.typeName, tc.direction)
	}
}

func TestIsValidWorkflowStatus_KnownStatuses(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	for _, slug := range []string{"in_progress", "not_started", "scoping", "needs_attention", "ready"} {
		valid, err := svc.IsValidWorkflowStatus(ctx, slug)
		require.NoError(t, err, "slug=%s", slug)
		assert.True(t, valid, "slug=%s should be valid", slug)
	}
}

func TestIsValidWorkflowStatus_UnknownSlug(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	valid, err := svc.IsValidWorkflowStatus(ctx, "nonexistent_slug")
	require.NoError(t, err)
	assert.False(t, valid)
}

// --- SlackProvider per-message reload tests ---

func TestSlackProvider_Reload_StoresPerMessageRecords(t *testing.T) {
	ctx := context.Background()
	searchOut := `[{"id":"C1","name":"general","messages":[
		{"user":"Alice","text":"hello","timestamp":"2026-05-05 10:00 UTC","rawTs":"1746441000.000000"},
		{"user":"Bob","text":"world","timestamp":"2026-05-05 11:00 UTC","rawTs":"1746444600.000000"}
	]}]`

	runner := &cmdRoutingRunner{outputs: map[string]string{"search-channels": searchOut}}
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	sRepo := slackstore.NewRepo(s.DB())

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	ctxRec, err := s.CreateContext(ctx, "test-per-msg", org.ID)
	require.NoError(t, err)

	_, err = sRepo.UpsertSlackChannel(ctx, ctxRec.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	p := NewSlackProvider(sRepo, runner, logger)
	_, err = p.Reload(ctx, ctxRec.ID)
	require.NoError(t, err)

	threads, err := sRepo.ListSlackThreads(ctx, ctxRec.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 2, "one record per message")
	assert.Contains(t, threads[0].Content, "Alice")
	assert.Contains(t, threads[0].Content, "hello")
	assert.Contains(t, threads[1].Content, "Bob")
	assert.Contains(t, threads[1].Content, "world")
}

func TestSlackProvider_Reload_DeletesOldBlob(t *testing.T) {
	ctx := context.Background()
	searchOut := `[{"id":"C1","name":"general","messages":[
		{"user":"Alice","text":"hi","timestamp":"2026-05-05 10:00 UTC","rawTs":"1746441000.000000"}
	]}]`

	runner := &cmdRoutingRunner{outputs: map[string]string{"search-channels": searchOut}}
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	sRepo := slackstore.NewRepo(s.DB())

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	ctxRec, err := s.CreateContext(ctx, "test-del-blob", org.ID)
	require.NoError(t, err)

	ch, err := sRepo.UpsertSlackChannel(ctx, ctxRec.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	// Insert an old channel blob (thread_ts="")
	_, err = sRepo.UpsertSlackThread(ctx, ctxRec.ID, slackstore.UpsertSlackThreadArgs{
		SlackChannelID: ch.ID,
		Workspace:      "acme",
		ChannelID:      "C1",
		ThreadTS:       "",
		ChannelName:    "general",
		Content:        "old blob content",
	})
	require.NoError(t, err)

	p := NewSlackProvider(sRepo, runner, logger)
	_, err = p.Reload(ctx, ctxRec.ID)
	require.NoError(t, err)

	threads, err := sRepo.ListSlackThreads(ctx, ctxRec.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1, "blob deleted; one per-message record remains")
	assert.NotEmpty(t, threads[0].ThreadTS, "per-message record has non-empty thread_ts")
}
