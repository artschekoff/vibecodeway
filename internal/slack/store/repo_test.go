package slackstore_test

import (
	"context"
	"path/filepath"
	"testing"

	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) (*appstore.Store, *slackstore.Repo) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, slackstore.NewRepo(s.DB())
}

// --- Slack provider ---

func TestUpsertAndGetSlackProvider(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "slack-ctx", org.ID)
	require.NoError(t, err)

	sp, err := repo.UpsertSlackProvider(ctx, c.ID, "acme", "deployment failed")
	require.NoError(t, err)
	assert.NotEmpty(t, sp.ID)
	assert.Equal(t, c.ID, sp.ContextID)
	assert.Equal(t, "acme", sp.Workspace)
	assert.Equal(t, "deployment failed", sp.SearchQuery)

	got, err := repo.GetSlackProvider(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, sp.ID, got.ID)
	assert.Equal(t, "acme", got.Workspace)
}

func TestUpsertSlackProviderUpdatesOnDuplicate(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "slack-ctx", org.ID)
	require.NoError(t, err)

	sp1, err := repo.UpsertSlackProvider(ctx, c.ID, "ws-old", "old query")
	require.NoError(t, err)

	sp2, err := repo.UpsertSlackProvider(ctx, c.ID, "ws-new", "new query")
	require.NoError(t, err)
	assert.Equal(t, sp1.ID, sp2.ID, "same row on duplicate upsert")
	assert.Equal(t, "ws-new", sp2.Workspace)
	assert.Equal(t, "new query", sp2.SearchQuery)
}

func TestGetSlackProviderNotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.GetSlackProvider(context.Background(), "no-context")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestDeleteContextCascadesToSlackProvider(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "del-ctx", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertSlackProvider(ctx, c.ID, "ws", "q")
	require.NoError(t, err)

	require.NoError(t, s.DeleteContext(ctx, c.ID))

	_, err = repo.GetSlackProvider(ctx, c.ID)
	assert.ErrorIs(t, err, slackstore.ErrNotFound, "slack_provider should cascade-delete")
}

// --- Slack thread CRUD ---

func TestUpsertAndListSlackThreads(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "slack-ctx", org.ID)
	require.NoError(t, err)

	thread, err := repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace:   "acme",
		ChannelID:   "C01234567",
		ThreadTS:    "1700000000.123456",
		ChannelName: "general",
		Permalink:   "https://acme.slack.com/archives/C01234567/p1700000000123456",
		SearchQuery: "deployment failed",
		Content:     "# Slack Context\n\nSome messages here.",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, thread.ID)
	assert.Equal(t, c.ID, thread.ContextID)
	assert.Equal(t, "acme", thread.Workspace)
	assert.Equal(t, "C01234567", thread.ChannelID)
	assert.Equal(t, "1700000000.123456", thread.ThreadTS)
	assert.Equal(t, "# Slack Context\n\nSome messages here.", thread.Content)

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, thread.ID, threads[0].ID)
}

func TestUpsertSlackThreadUpdatesOnDuplicate(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "slack-ctx", org.ID)
	require.NoError(t, err)

	args := slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C01", ThreadTS: "1.23", Content: "original",
	}
	t1, err := repo.UpsertSlackThread(ctx, c.ID, args)
	require.NoError(t, err)

	args.Content = "updated"
	t2, err := repo.UpsertSlackThread(ctx, c.ID, args)
	require.NoError(t, err)
	assert.Equal(t, t1.ID, t2.ID, "same ID on duplicate upsert")

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, "updated", threads[0].Content)
}

func TestDeleteContextCascadesToSlackThreads(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "del-ctx", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "ws", ChannelID: "C1", ThreadTS: "1.1", Content: "msg",
	})
	require.NoError(t, err)

	require.NoError(t, s.DeleteContext(ctx, c.ID))

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, threads, "threads should be deleted with context (cascade)")
}

// --- Slack channel CRUD ---

func TestUpsertAndListSlackChannels(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)

	ch, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C01234567", "general", "")
	require.NoError(t, err)
	assert.NotEmpty(t, ch.ID)
	assert.Equal(t, c.ID, ch.ContextID)
	assert.Equal(t, "acme", ch.Workspace)
	assert.Equal(t, "C01234567", ch.ChannelID)
	assert.Equal(t, "general", ch.ChannelName)

	channels, err := repo.ListSlackChannels(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, ch.ID, channels[0].ID)
}

func TestUpsertSlackChannelIsIdempotent(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)

	ch1, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C01", "general", "")
	require.NoError(t, err)

	ch2, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C01-updated", "general", "")
	require.NoError(t, err)

	assert.Equal(t, ch1.ID, ch2.ID, "same row on duplicate upsert")
	assert.Equal(t, "C01-updated", ch2.ChannelID)
}

func TestListSlackChannels_MultipleChannels(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)

	for _, name := range []string{"general", "deploy-alerts", "backend"} {
		_, err = repo.UpsertSlackChannel(ctx, c.ID, "acme", "", name, "")
		require.NoError(t, err)
	}

	channels, err := repo.ListSlackChannels(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Len(t, channels, 3)
}

func TestListSlackChannels_EmptyForOtherContext(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c1, err := s.CreateContext(ctx, "ctx-1", org.ID)
	require.NoError(t, err)
	c2, err := s.CreateContext(ctx, "ctx-2", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertSlackChannel(ctx, c1.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	channels, err := repo.ListSlackChannels(ctx, c2.ID, false)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func TestRemoveSlackChannel_Success(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)

	ch, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	require.NoError(t, repo.RemoveSlackChannel(ctx, ch.ID))

	channels, err := repo.ListSlackChannels(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

func TestRemoveSlackChannel_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	err := repo.RemoveSlackChannel(context.Background(), "nonexistent-id")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestRemoveSlackChannel_CascadesToThreads(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "cascade-ctx", org.ID)
	require.NoError(t, err)

	ch, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C99", "incidents", "")
	require.NoError(t, err)

	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		SlackChannelID: ch.ID,
		Workspace:      "acme",
		ChannelID:      "C99",
		ThreadTS:       "1.1",
		Content:        "some incident",
	})
	require.NoError(t, err)

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1, "thread should exist before channel deletion")

	require.NoError(t, repo.RemoveSlackChannel(ctx, ch.ID))

	threads, err = repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, threads, "threads should cascade-delete when their channel is removed")
}

// --- IsActive toggle ---

func TestSlackChannelActiveByDefault(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)
	_, err = repo.UpsertSlackChannel(ctx, c.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	channels, err := repo.ListSlackChannels(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.True(t, channels[0].IsActive, "new channel should be active by default")
}

func TestToggleSlackChannelActive(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)
	ch, err := repo.UpsertSlackChannel(ctx, c.ID, "acme", "C1", "general", "")
	require.NoError(t, err)

	toggled, err := repo.ToggleSlackChannelActive(ctx, ch.ID)
	require.NoError(t, err)
	assert.False(t, toggled.IsActive)

	toggled, err = repo.ToggleSlackChannelActive(ctx, ch.ID)
	require.NoError(t, err)
	assert.True(t, toggled.IsActive)
}

func TestToggleSlackChannelActive_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.ToggleSlackChannelActive(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestSlackThreadActiveByDefault(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)
	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1", Content: "msg",
	})
	require.NoError(t, err)

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.True(t, threads[0].IsActive, "new thread should be active by default")
}

func TestToggleSlackThreadActive(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "ch-ctx", org.ID)
	require.NoError(t, err)
	thread, err := repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "1.1", Content: "msg",
	})
	require.NoError(t, err)

	toggled, err := repo.ToggleSlackThreadActive(ctx, thread.ID)
	require.NoError(t, err)
	assert.False(t, toggled.IsActive)

	toggled, err = repo.ToggleSlackThreadActive(ctx, thread.ID)
	require.NoError(t, err)
	assert.True(t, toggled.IsActive)
}

func TestToggleSlackThreadActive_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	_, err := repo.ToggleSlackThreadActive(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

// --- RemoveSlackDMEntry ---

func TestRemoveSlackDMEntry_Success(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := `[{"id":"D1","userName":"alice","messages":[]},{"id":"D2","userName":"bob","messages":[]}]`
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "D1")
	require.NoError(t, err)

	updated, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Contains(t, updated[0].Content, `"id":"D2"`)
	assert.NotContains(t, updated[0].Content, `"id":"D1"`)
}

func TestRemoveSlackDMEntry_LastEntry(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := `[{"id":"D1","userName":"alice","messages":[]}]`
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "D1")
	require.NoError(t, err)

	updated, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Equal(t, "[]", updated[0].Content)
}

func TestRemoveSlackDMEntry_NotFoundEntry(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := `[{"id":"D1","userName":"alice","messages":[]}]`
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "nonexistent")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestRemoveSlackDMEntry_DMConfigNotFound(t *testing.T) {
	_, repo := openTestDB(t)
	err := repo.RemoveSlackDMEntry(context.Background(), "nonexistent-config", "D1")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestRemoveSlackDMEntry_EmptyContent(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "D1")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestRemoveSlackDMEntry_PlainTextContent(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := "# DM with Alice\n\n[2026-03-18 14:12 UTC] U09ABC: Hello Alice\n[2026-03-18 14:15 UTC] U09XYZ: Hi there!\n\n# DM with Bob\n\n[2026-03-19 10:00 UTC] U09ABC: Hey Bob\n"
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "conv-0")
	require.NoError(t, err, "should handle plain-text DM content without JSON parse error")

	updated, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Contains(t, updated[0].Content, "# DM with Bob")
	assert.NotContains(t, updated[0].Content, "# DM with Alice")
}

func TestRemoveSlackDMEntry_PlainTextLastEntry(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := "# DM with Alice\n\n[2026-03-18 14:12 UTC] U09ABC: Hello Alice\n"
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "conv-0")
	require.NoError(t, err)

	updated, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Empty(t, updated[0].Content, "removing last entry should leave empty content")
}

func TestRemoveSlackDMEntry_PlainTextEntryNotFound(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	content := "# DM with Alice\n\n[2026-03-18 14:12 UTC] U09ABC: Hello Alice\n"
	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, content))

	err = repo.RemoveSlackDMEntry(ctx, dm.ID, "conv-5")
	assert.ErrorIs(t, err, slackstore.ErrNotFound, "out-of-range conv index should return not found")
}

// --- Slack DM config CRUD ---

func TestUpsertSlackDMConfigIsIdempotent(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm1, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)
	require.NotEmpty(t, dm1.ID)

	dm2, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-06-01")
	require.NoError(t, err)

	assert.Equal(t, dm1.ID, dm2.ID, "same row on duplicate upsert")
	assert.Equal(t, "2025-06-01", dm2.StartFrom, "start_from should be updated")

	configs, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Len(t, configs, 1, "duplicate upsert must not create extra rows")
}

func TestUpsertSlackDMConfigPreservesContent(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	require.NoError(t, repo.UpdateSlackDMConfigContent(ctx, dm.ID, "# DM with alice\nhello"))

	dm2, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-06-01")
	require.NoError(t, err)
	assert.Equal(t, dm.ID, dm2.ID)
	assert.Equal(t, "# DM with alice\nhello", dm2.Content, "content must survive date upsert")
}

func TestDeleteSlackDMConfig_Success(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "dm-ctx", org.ID)
	require.NoError(t, err)

	dm, err := repo.UpsertSlackDMConfig(ctx, c.ID, "acme", "2025-01-01")
	require.NoError(t, err)

	require.NoError(t, repo.DeleteSlackDMConfig(ctx, dm.ID))

	configs, err := repo.ListSlackDMConfigs(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, configs, "config should be deleted")
}

func TestDeleteSlackDMConfig_NotFound(t *testing.T) {
	_, repo := openTestDB(t)
	err := repo.DeleteSlackDMConfig(context.Background(), "nonexistent-id")
	assert.ErrorIs(t, err, slackstore.ErrNotFound)
}

func TestDeleteContextCascadesToSlackChannels(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "del-ctx", org.ID)
	require.NoError(t, err)

	_, err = repo.UpsertSlackChannel(ctx, c.ID, "ws", "C1", "general", "")
	require.NoError(t, err)

	require.NoError(t, s.DeleteContext(ctx, c.ID))

	channels, err := repo.ListSlackChannels(ctx, c.ID, false)
	require.NoError(t, err)
	assert.Empty(t, channels, "channels should be deleted with context (cascade)")
}

func TestDeleteChannelBlobs_RemovesOnlyEmptyThreadTS(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "blob-ctx", org.ID)
	require.NoError(t, err)

	// Old whole-channel blob (thread_ts="")
	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "", ChannelName: "general", Content: "old blob",
	})
	require.NoError(t, err)

	// Per-message record (thread_ts set)
	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "1746441180.000000", ChannelName: "general", Content: "message",
	})
	require.NoError(t, err)

	err = repo.DeleteChannelBlobs(ctx, c.ID, "C1")
	require.NoError(t, err)

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1, "only the per-message record should remain")
	assert.Equal(t, "1746441180.000000", threads[0].ThreadTS, "per-message record must survive")
}

func TestDeleteChannelBlobs_LeavesOtherChannelsUntouched(t *testing.T) {
	s, repo := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "blob-ctx2", org.ID)
	require.NoError(t, err)

	// Blob for C1 (to be deleted)
	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C1", ThreadTS: "", ChannelName: "general", Content: "c1 blob",
	})
	require.NoError(t, err)

	// Blob for C2 (must NOT be deleted)
	_, err = repo.UpsertSlackThread(ctx, c.ID, slackstore.UpsertSlackThreadArgs{
		Workspace: "acme", ChannelID: "C2", ThreadTS: "", ChannelName: "other", Content: "c2 blob",
	})
	require.NoError(t, err)

	err = repo.DeleteChannelBlobs(ctx, c.ID, "C1")
	require.NoError(t, err)

	threads, err := repo.ListSlackThreads(ctx, c.ID, false)
	require.NoError(t, err)
	require.Len(t, threads, 1, "C2 blob must survive")
	assert.Equal(t, "C2", threads[0].ChannelID)
}
