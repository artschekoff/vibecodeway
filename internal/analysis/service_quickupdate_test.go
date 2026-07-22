package analysis

import (
	"context"
	"path/filepath"
	"testing"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeQuickUpdateAI is a scripted chatClient double that returns a fixed
// JSON array response for the context_updater prompt, capturing the prompt
// it was sent so tests can assert on injected placeholders.
type fakeQuickUpdateAI struct {
	response string
	prompt   string
}

func (f *fakeQuickUpdateAI) ChatCompletion(_ context.Context, messages []openai.Message) (*openai.Response, error) {
	f.prompt = messages[0].Content
	return &openai.Response{Content: f.response}, nil
}

func (f *fakeQuickUpdateAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}

func (f *fakeQuickUpdateAI) EmbedText(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

// newQuickUpdateTestService builds a Service backed by a temp SQLite DB with
// one org and one context, ready for InterpretContextUpdate.
func newQuickUpdateTestService(t *testing.T, ai *fakeQuickUpdateAI) (*Service, string) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	jiraSvc := jiraservice.NewContextService(s, nil, jiraRepo, nil, nil, zap.NewNop())

	svc := &Service{
		store:   s,
		jiraSvc: jiraSvc,
		ai:      ai,
		logger:  zap.NewNop(),
	}
	return svc, ctxRow.ID
}

func TestInterpretContextUpdate_returnsMultiActionSlice(t *testing.T) {
	ai := &fakeQuickUpdateAI{
		response: `[{"action":"set_status","statusSlug":"in_review","statusComment":"done","description":"Set status → In Review"},
 {"action":"add_event","eventType":"work_completed","summary":"perf fixed","description":"Add work_completed: perf fixed"}]`,
	}
	svc, contextID := newQuickUpdateTestService(t, ai)

	actions, err := svc.InterpretContextUpdate(context.Background(), contextID, "fixed the perf issue and moved to review")
	require.NoError(t, err)
	require.Len(t, actions, 2)

	require.Equal(t, "set_status", actions[0].Action)
	require.Equal(t, "in_review", actions[0].StatusSlug)

	require.Equal(t, "add_event", actions[1].Action)
	require.Equal(t, "work_completed", actions[1].EventType)
}

// TestInterpretContextUpdate_stripsFencesAndProse verifies that a response
// wrapped in markdown code fences with leading prose (common LLM behavior)
// still parses correctly via extractJSON, rather than failing a direct
// json.Unmarshal on the raw content.
func TestInterpretContextUpdate_stripsFencesAndProse(t *testing.T) {
	ai := &fakeQuickUpdateAI{
		response: "Here are the actions:\n```json\n" +
			`[{"action":"set_status","statusSlug":"in_review","statusComment":"done","description":"Set status → In Review"},
 {"action":"add_event","eventType":"work_completed","summary":"perf fixed","description":"Add work_completed: perf fixed"}]` +
			"\n```",
	}
	svc, contextID := newQuickUpdateTestService(t, ai)

	actions, err := svc.InterpretContextUpdate(context.Background(), contextID, "fixed the perf issue and moved to review")
	require.NoError(t, err)
	require.Len(t, actions, 2)

	require.Equal(t, "set_status", actions[0].Action)
	require.Equal(t, "in_review", actions[0].StatusSlug)

	require.Equal(t, "add_event", actions[1].Action)
	require.Equal(t, "work_completed", actions[1].EventType)
}
