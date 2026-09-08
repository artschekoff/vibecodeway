package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// recordingClient captures the messages the agent sends and answers immediately,
// so one Ask round-trip is enough to inspect the prompt.
type recordingClient struct{ got []openai.Message }

func (c *recordingClient) ChatWithTools(_ context.Context, messages []openai.Message, _ []openai.ToolDef) (*openai.Response, error) {
	c.got = messages
	return &openai.Response{Content: "done"}, nil
}

// The date must be a message of its own rather than an edit to the built-in
// prompt: the prompt is user-editable and read from the DB, so an install that
// has saved its own copy would never receive a change to the constant.
func TestAskInjectsTheCurrentDateAsItsOwnSystemMessage(t *testing.T) {
	// A real store, because Ask loads the editable prompt through it before the
	// date message is built (agent.go loadPrompt).
	st, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	c := &recordingClient{}
	a := &Agent{
		ai:      c,
		reg:     tooling.NewRegistry(tooling.Deps{Logger: zap.NewNop()}),
		store:   st,
		logger:  zap.NewNop(),
		pending: map[string]chan bool{},
		// Monday 2026-09-07 — the day the bug was reported.
		now: func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC) },
	}

	_, err = a.Ask(context.Background(), "какие задачи я делал прошлой пятницей?", "", nil)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(c.got), 3, "expected prompt, date, and question")
	assert.Equal(t, "system", c.got[1].Role)
	assert.Contains(t, c.got[1].Content, "2026-09-07")
	assert.Contains(t, c.got[1].Content, "Monday")
	// The question must stay last so the model reads it after its context.
	assert.Equal(t, "user", c.got[len(c.got)-1].Role)
	assert.Contains(t, c.got[len(c.got)-1].Content, "прошлой пятницей")
}

// The message has to actually instruct the model, not just state a date —
// otherwise a relative expression can still reach a tool as a keyword.
func TestDateSystemMessageTellsTheModelToResolveRelativeDates(t *testing.T) {
	msg := dateSystemMessage(time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC))

	assert.Contains(t, msg, "2026-09-07")
	assert.Contains(t, msg, "Monday")
	assert.Contains(t, msg, "YYYY-MM-DD")
	assert.Contains(t, strings.ToLower(msg), "never")
}

// New must wire a real clock, or every install reports the zero time.
func TestNewDefaultsToARealClock(t *testing.T) {
	a := New(nil, tooling.NewRegistry(tooling.Deps{Logger: zap.NewNop()}), nil, zap.NewNop())

	require.NotNil(t, a.now)
	assert.WithinDuration(t, time.Now(), a.now(), time.Minute)
}
