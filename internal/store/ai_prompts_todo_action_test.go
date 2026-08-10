package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestTodoActionPromptsSeeded(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ids := []string{
		"todo_action_rerequest_pr",
		"todo_action_make_pr",
		"todo_action_ask_question",
		"todo_action_check_status",
	}
	for _, id := range ids {
		p, err := s.GetAIPrompt(context.Background(), id)
		if err != nil {
			t.Fatalf("prompt %s not seeded: %v", id, err)
		}
		if p.Body == "" || p.DefaultBody == "" {
			t.Fatalf("prompt %s has empty body/default", id)
		}
		if len(p.Placeholders) == 0 {
			t.Fatalf("prompt %s has no registered placeholders", id)
		}
	}
}
