package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskChatRoundTripAndPrune(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.UpsertAskChat(ctx, "c1", "first", `[{"role":"user","content":"hi"}]`))
	require.NoError(t, s.UpsertAskChat(ctx, "c1", "first (edited)", `[{"role":"user","content":"hi again"}]`))

	got, err := s.GetAskChat(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, "first (edited)", got.Title)
	assert.Contains(t, got.LogJSON, "hi again")

	list, err := s.ListAskChats(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].LogJSON, "list omits bodies")

	// Age the row past a 7-day retention and prune.
	old := time.Now().UTC().AddDate(0, 0, -8).Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `UPDATE ask_chats SET updated_at = ? WHERE id = 'c1'`, old)
	require.NoError(t, err)
	n, err := s.PruneAskChats(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	_, err = s.GetAskChat(ctx, "c1")
	assert.ErrorIs(t, err, ErrNotFound)

	// 0 = keep forever.
	require.NoError(t, s.UpsertAskChat(ctx, "c2", "", "[]"))
	n, err = s.PruneAskChats(ctx, 0)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestAppSettingsAskChatRetentionRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	def, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 30, def.AskChatRetentionDays)
	def.AskChatRetentionDays = 90
	require.NoError(t, s.SetAppSettings(ctx, *def))
	got, err := s.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 90, got.AskChatRetentionDays)
}
