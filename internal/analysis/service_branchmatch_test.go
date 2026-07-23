package analysis

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeBranchMatchAI is a scripted chatClient double returning a fixed response
// for the branch_match call.
type fakeBranchMatchAI struct {
	response string
}

func (f *fakeBranchMatchAI) ChatCompletion(_ context.Context, _ []openai.Message) (*openai.Response, error) {
	return &openai.Response{Content: f.response}, nil
}

func (f *fakeBranchMatchAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}

func (f *fakeBranchMatchAI) EmbedText(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

// newBranchMatchTestService builds a Service backed by a temp SQLite DB so
// loadPrompt's DB lookup (which falls back to the in-code default on a miss)
// doesn't panic on a nil store.
func newBranchMatchTestService(t *testing.T, ai *fakeBranchMatchAI) *Service {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	return &Service{store: s, ai: ai, logger: zap.NewNop()}
}

func TestMatchBranchesToContext_matchesGivenIndices(t *testing.T) {
	ai := &fakeBranchMatchAI{response: "[0,2]"}
	svc := newBranchMatchTestService(t, ai)

	titles := []string{"feat/JH-4532-slow-sql", "chore/deps", "feat/JH-4532-index"}
	got, err := svc.MatchBranchesToContext(context.Background(), "JH-4532", "slow sql query", titles)
	require.NoError(t, err)
	require.Equal(t, []int{0, 2}, got)
}

func TestMatchBranchesToContext_unparseableResponse_failsSoft(t *testing.T) {
	ai := &fakeBranchMatchAI{response: "garbage"}
	svc := newBranchMatchTestService(t, ai)

	titles := []string{"feat/JH-4532-slow-sql", "chore/deps", "feat/JH-4532-index"}
	got, err := svc.MatchBranchesToContext(context.Background(), "JH-4532", "slow sql query", titles)
	require.NoError(t, err)
	require.Equal(t, []int{}, got)
}

func TestMatchBranchesToContext_emptyTitles_returnsEmptyFailSoft(t *testing.T) {
	ai := &fakeBranchMatchAI{response: "[0]"}
	svc := newBranchMatchTestService(t, ai)

	got, err := svc.MatchBranchesToContext(context.Background(), "JH-4532", "slow sql query", []string{})
	require.NoError(t, err)
	require.Equal(t, []int{}, got)
}

func TestMatchBranchesToContext_outOfRangeIndex_filtered(t *testing.T) {
	ai := &fakeBranchMatchAI{response: "[0,99]"}
	svc := newBranchMatchTestService(t, ai)

	titles := []string{"feat/JH-4532-slow-sql", "chore/deps"}
	got, err := svc.MatchBranchesToContext(context.Background(), "JH-4532", "slow sql query", titles)
	require.NoError(t, err)
	require.Equal(t, []int{0}, got)
}
