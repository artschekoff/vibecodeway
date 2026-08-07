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

func TestEditContextTool(t *testing.T) {
	store := newTestStore(t)
	reg := NewRegistry(Deps{Store: store})
	orgID := seedOrg(t, store)
	id := seedContext(t, store, "sprint-42", orgID)

	out, err := reg.Call(context.Background(), "edit_context",
		map[string]any{"contextId": id, "status": "archived"}, Ctx{})
	if err != nil {
		t.Fatalf("edit_context: %v", err)
	}
	m := out.(map[string]any)
	if m["status"] != "archived" || m["key"] != "sprint-42" {
		t.Fatalf("edit_context should archive and keep key, got %v", m)
	}
	if tool, _ := reg.Get("edit_context"); !tool.Write {
		t.Fatal("edit_context must be a write tool")
	}
}

func TestListContextsTool(t *testing.T) {
	store := newTestStore(t)
	reg := NewRegistry(Deps{Store: store})
	orgID := seedOrg(t, store)
	seedContext(t, store, "ctx-a", orgID)
	seedContext(t, store, "ctx-b", orgID)

	out, err := reg.Call(context.Background(), "list_contexts", map[string]any{}, Ctx{OrgID: orgID})
	if err != nil {
		t.Fatalf("list_contexts: %v", err)
	}
	if out.(map[string]any)["count"].(int) != 2 {
		t.Fatalf("expected 2 contexts, got %v", out)
	}
}

func TestFindContextsByJiraStatusTool(t *testing.T) {
	store := newTestStore(t)
	reg := NewRegistry(Deps{Store: store})
	orgID := seedOrg(t, store)
	id := seedContext(t, store, "ctx-closed", orgID)
	_, err := store.DB().ExecContext(context.Background(),
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES ('ji-1', ?, 'JH-1', 's', 'Closed', '', datetime('now'))`, id)
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}

	out, err := reg.Call(context.Background(), "find_contexts_by_jira_status",
		map[string]any{"jiraStatus": "closed"}, Ctx{})
	if err != nil {
		t.Fatalf("find_contexts_by_jira_status: %v", err)
	}
	m := out.(map[string]any)
	if m["count"].(int) != 1 {
		t.Fatalf("expected 1 closed context, got %v", m)
	}
}

func TestAddEventTool(t *testing.T) {
	store := newTestStore(t)
	reg := NewRegistry(Deps{Store: store})
	orgID := seedOrg(t, store)
	id := seedContext(t, store, "sprint-42", orgID)

	out, err := reg.Call(context.Background(), "add_event", map[string]any{
		"contextId": id, "type": "user_note", "summary": "checked the staging deploy",
	}, Ctx{})
	if err != nil {
		t.Fatalf("add_event: %v", err)
	}
	if out.(map[string]any)["id"] == "" {
		t.Fatalf("add_event should return the new event id, got %v", out)
	}
	if tool, _ := reg.Get("add_event"); !tool.Write {
		t.Fatal("add_event must be a write tool")
	}
}
