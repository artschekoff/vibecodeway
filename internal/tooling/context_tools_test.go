package tooling

import (
	"context"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

// newTestStore opens a fresh migrated SQLite store in a temp dir.
func newTestStore(t *testing.T) *appstore.Store {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedOrg creates an organization and returns its id (contexts FK to orgs).
func seedOrg(t *testing.T, s *appstore.Store) string {
	t.Helper()
	org, err := s.CreateOrganization(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	return org.ID
}

// seedContext creates a context row and returns its id.
func seedContext(t *testing.T, s *appstore.Store, key, orgID string) string {
	t.Helper()
	c, err := s.CreateContext(context.Background(), key, orgID)
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	return c.ID
}

func TestSearchContextsTool(t *testing.T) {
	store := newTestStore(t)
	reg := NewRegistry(Deps{Store: store})

	orgID := seedOrg(t, store)
	ctxID := seedContext(t, store, "sprint-42", orgID)

	out, err := reg.Call(context.Background(), "search_contexts",
		map[string]any{"query": "sprint"}, Ctx{OrgID: orgID})
	if err != nil {
		t.Fatalf("search_contexts: %v", err)
	}
	m := out.(map[string]any)
	if m["count"].(int) < 1 {
		t.Fatalf("expected >=1 hit for seeded context %s, got %v", ctxID, m)
	}
}
