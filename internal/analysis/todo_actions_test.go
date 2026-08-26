package analysis

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"go.uber.org/zap"
)

// captureAI satisfies chatClient and records the last messages it received.
type captureAI struct {
	content string
	last    []openai.Message
}

func (c *captureAI) ChatCompletion(_ context.Context, m []openai.Message) (*openai.Response, error) {
	c.last = m
	return &openai.Response{Content: c.content}, nil
}

func (*captureAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}
func (*captureAI) EmbedText(_ context.Context, _ string) ([]float32, error) { return nil, nil }

func newDraftService(t *testing.T, ai chatClient) *Service {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &Service{store: s, ai: ai, logger: zap.NewNop()}
}

func TestDraftTodoMessage_RendersAndTrims(t *testing.T) {
	ai := &captureAI{content: "  Please review PR o/r#42 when you get a moment.  "}
	svc := newDraftService(t, ai)

	out, err := svc.DraftTodoMessage(context.Background(), "rerequest_pr", "Ping reviewers on #42", "o/r#42", "stale 10d")
	if err != nil {
		t.Fatalf("DraftTodoMessage error: %v", err)
	}
	if out != "Please review PR o/r#42 when you get a moment." {
		t.Fatalf("output not trimmed: %q", out)
	}
	sent := ai.last[0].Content
	if !strings.Contains(sent, "o/r#42") || !strings.Contains(sent, "Ping reviewers on #42") {
		t.Fatalf("placeholders not substituted: %q", sent)
	}
	if strings.Contains(sent, "{{") {
		t.Fatalf("unsubstituted placeholder remains: %q", sent)
	}
}

func TestDraftTodoMessage_EmptyRefKey(t *testing.T) {
	ai := &captureAI{content: "draft"}
	svc := newDraftService(t, ai)
	if _, err := svc.DraftTodoMessage(context.Background(), "ask_question", "Clarify scope", "", "unclear"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ai.last[0].Content, "(none)") {
		t.Fatalf("empty refKey should render as (none): %q", ai.last[0].Content)
	}
}

func TestDraftTodoMessage_UnknownKind(t *testing.T) {
	svc := newDraftService(t, &captureAI{})
	if _, err := svc.DraftTodoMessage(context.Background(), "implement", "x", "", ""); err == nil {
		t.Fatalf("expected error for kind with no draft action")
	}
}
