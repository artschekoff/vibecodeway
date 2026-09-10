package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tooling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAskChatPrependsPriorMessagesInOrder verifies AskChat inserts the prior
// conversation turns, in order, between the system/date preamble and the new
// question — so the model sees the same conversation the UI is showing.
func TestAskChatPrependsPriorMessagesInOrder(t *testing.T) {
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
	}

	prior := []openai.Message{
		{Role: "user", Content: "what is context X about?"},
		{Role: "assistant", Content: "it's about the payments refactor"},
	}

	_, err = a.AskChat(context.Background(), prior, "who is blocking it?", "", nil)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(c.got), 5, "expected prompt, date, 2 prior turns, and the new question")
	assert.Equal(t, "system", c.got[0].Role)
	assert.Equal(t, "system", c.got[1].Role)
	assert.Equal(t, "user", c.got[2].Role)
	assert.Equal(t, "what is context X about?", c.got[2].Content)
	assert.Equal(t, "assistant", c.got[3].Role)
	assert.Equal(t, "it's about the payments refactor", c.got[3].Content)
	assert.Equal(t, "user", c.got[4].Role)
	assert.Equal(t, "who is blocking it?", c.got[4].Content)
}

// TestAskWrapsAskChatWithNilPrior verifies Ask (the pre-existing single-shot
// entry point) still behaves exactly as before: no prior turns are inserted.
func TestAskWrapsAskChatWithNilPrior(t *testing.T) {
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
	}

	_, err = a.Ask(context.Background(), "hello", "", nil)
	require.NoError(t, err)

	require.Len(t, c.got, 3, "expected exactly prompt, date, and the question — no prior turns")
	assert.Equal(t, "user", c.got[2].Role)
	assert.Equal(t, "hello", c.got[2].Content)
}

// TestSummarizeReturnsContentAndPassesNoTools verifies Summarize is a single
// plain model call: it returns resp.Content, and — critically — advertises no
// tools, since compacting a transcript must never trigger further tool use.
func TestSummarizeReturnsContentAndPassesNoTools(t *testing.T) {
	c := &toolCapturingClient{resp: &openai.Response{Content: "summary text"}}
	a := &Agent{ai: c, logger: zap.NewNop()}

	prior := []openai.Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
	}

	got, err := a.Summarize(context.Background(), prior)
	require.NoError(t, err)
	assert.Equal(t, "summary text", got)

	require.NotNil(t, c.gotMessages)
	assert.Nil(t, c.gotTools, "Summarize must not advertise any tools")
	assert.Equal(t, "system", c.gotMessages[0].Role)
	assert.Equal(t, "user", c.gotMessages[1].Role)
	assert.Equal(t, "q1", c.gotMessages[1].Content)
	assert.Equal(t, "assistant", c.gotMessages[2].Role)
	assert.Equal(t, "a1", c.gotMessages[2].Content)
}

// toolCapturingClient records the tools slice it was called with, so tests can
// assert a call was made without tool definitions.
type toolCapturingClient struct {
	resp        *openai.Response
	gotMessages []openai.Message
	gotTools    []openai.ToolDef
}

func (c *toolCapturingClient) ChatWithTools(_ context.Context, messages []openai.Message, tools []openai.ToolDef) (*openai.Response, error) {
	c.gotMessages = messages
	c.gotTools = tools
	return c.resp, nil
}
