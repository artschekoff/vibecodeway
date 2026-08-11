package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAppLogRepoFieldsRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	repo := NewAppLogRepo(s.DB())
	ctx := context.Background()

	if err := repo.Add(ctx, "info", 1.5, "openai/client.go:200", "llm.chat_completion …", "", `{"request":"[{\"role\":\"user\"}]","durationMs":2100}`); err != nil {
		t.Fatalf("add: %v", err)
	}
	rows, err := repo.List(ctx, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list = %d, %v", len(rows), err)
	}
	if rows[0].Fields == "" || rows[0].Level != "info" {
		t.Fatalf("fields/level not persisted: %+v", rows[0])
	}
}
