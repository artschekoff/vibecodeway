package analysis

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"go.uber.org/zap"
)

// fakeTodoAI returns a fixed JSON list for any ChatCompletion call.
type fakeTodoAI struct{ content string }

func (f fakeTodoAI) ChatCompletion(_ context.Context, _ []openai.Message) (*openai.Response, error) {
	return &openai.Response{Content: f.content}, nil
}
func (fakeTodoAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}
func (fakeTodoAI) EmbedText(_ context.Context, _ string) ([]float32, error) { return nil, nil }

func TestParseTodoResponse(t *testing.T) {
	raw := "```json\n[{\"kind\":\"rerequest_pr\",\"title\":\"Ping reviewers on #42\",\"rationale\":\"stale 10d\",\"refType\":\"pr\",\"refKey\":\"o/r#42\",\"priority\":\"high\"},{\"kind\":\"implement\",\"title\":\"Finish JH-1\",\"priority\":\"medium\"}]\n```"
	items, err := parseTodoResponse(raw)
	if err != nil || len(items) != 2 {
		t.Fatalf("parseTodoResponse = %d, %v", len(items), err)
	}
	if items[0].Position != 0 || items[1].Position != 1 {
		t.Fatalf("positions must follow array order: %+v", items)
	}
	if items[0].Kind != "rerequest_pr" || items[0].RefKey != "o/r#42" {
		t.Fatalf("bad parse: %+v", items[0])
	}
	if items[0].Source != "ai" {
		t.Fatalf("items must be marked ai-sourced: %+v", items[0])
	}
}

func TestGenerateTodoNextPersists(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	svc := &Service{store: s, ai: fakeTodoAI{content: `[{"kind":"implement","title":"Do JH-1","priority":"high"}]`}, logger: zap.NewNop()}

	input := TodoInput{
		JiraIssues: []appstore.BoardJiraAssignment{{Key: "JH-1", Summary: "x", Priority: "High", Status: "In Progress"}},
		Now:        time.Now(), ThresholdDays: 7,
	}
	items, err := svc.GenerateTodoNext(context.Background(), input)
	if err != nil || len(items) != 1 || items[0].ID == "" {
		t.Fatalf("GenerateTodoNext = %+v, %v", items, err)
	}
	if items[0].Kind != "implement" || items[0].Source != "ai" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}
