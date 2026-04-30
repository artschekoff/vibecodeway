package contextlog_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func openTestSvc(t *testing.T) (*store.Store, *contextlog.Service) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err, "open store")
	t.Cleanup(func() { _ = s.Close() })
	repo := contextlog.NewRepo(s.DB())
	svc := contextlog.NewService(repo, nil, zap.NewNop())
	return s, svc
}

func TestCreateAgentLog_persists(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	entry, err := svc.CreateAgentLog(ctx, c.ID, "Agent completed code review.")
	require.NoError(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, c.ID, entry.ContextID)
	assert.Equal(t, contextlog.TriggerAgent, entry.Trigger)
	assert.Equal(t, "mcp", entry.Source)
	assert.Equal(t, "Agent completed code review.", entry.SummaryMd)
	assert.False(t, entry.IsManual)
	assert.NotEmpty(t, entry.CreatedAt)
}

func TestCreateAgentLog_emptyMessage_returnsError(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	_, err = svc.CreateAgentLog(ctx, c.ID, "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")
}

func TestCreateAgentLog_appearsInListLogs(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "test-ctx")
	require.NoError(t, err)

	_, err = svc.CreateAgentLog(ctx, c.ID, "First agent note.")
	require.NoError(t, err)
	_, err = svc.CreateAgentLog(ctx, c.ID, "Second agent note.")
	require.NoError(t, err)

	logs, err := svc.ListLogs(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, logs, 2)

	messages := []string{logs[0].SummaryMd, logs[1].SummaryMd}
	assert.Contains(t, messages, "First agent note.")
	assert.Contains(t, messages, "Second agent note.")
}
