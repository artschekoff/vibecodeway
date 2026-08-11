package applog

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/store"
	"go.uber.org/zap"
)

func TestDBCorePersistsInfoWithFields(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	repo := store.NewAppLogRepo(s.DB())

	logger := zap.New(NewDBCore(context.Background(), repo))
	logger.Info("llm.chat_completion", zap.String("request", `[{"role":"user"}]`), zap.Int64("durationMs", 2100))
	_ = logger.Sync()

	rows, err := repo.List(context.Background(), 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list = %d, %v", len(rows), err)
	}
	if rows[0].Level != "info" {
		t.Fatalf("info not persisted: %+v", rows[0])
	}
	if !strings.Contains(rows[0].Fields, "request") || !strings.Contains(rows[0].Fields, "durationMs") {
		t.Fatalf("fields not serialized: %q", rows[0].Fields)
	}
	if strings.Contains(rows[0].Fields, `"msg"`) || strings.Contains(rows[0].Fields, `"caller"`) {
		t.Fatalf("standard keys must be excluded from fields: %q", rows[0].Fields)
	}
}
