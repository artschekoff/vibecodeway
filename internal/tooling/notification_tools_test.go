package tooling

import (
	"context"
	"strings"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

// newNotificationRegistry builds a registry over a real store with one context.
func newNotificationRegistry(t *testing.T) (*Registry, *appstore.Store, string) {
	t.Helper()
	s := newTestStore(t)
	orgID := seedOrg(t, s)
	c, err := s.CreateContext(context.Background(), "PROJ-1", orgID)
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	return NewRegistry(Deps{Store: s}), s, c.ID
}

func TestAddNotificationRuleTool(t *testing.T) {
	r, s, contextID := newNotificationRegistry(t)
	ctx := context.Background()

	out, err := r.Call(ctx, "add_notification_rule", map[string]any{
		"contextId":  contextID,
		"matchValue": "Done",
		"note":       "tell QA we are ready",
	}, Ctx{})
	if err != nil {
		t.Fatalf("call add_notification_rule: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected a map result, got %T", out)
	}
	if m["id"] == "" || m["id"] == nil {
		t.Fatalf("expected an id in the result, got %v", m)
	}
	// eventType defaults to the only event the pipeline honours.
	if m["eventType"] != "jira_status_changed" {
		t.Fatalf("expected a defaulted eventType, got %v", m["eventType"])
	}

	rules, err := s.ListNotificationRules(ctx, contextID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || rules[0].Note != "tell QA we are ready" || rules[0].MatchValue != "Done" {
		t.Fatalf("rule not stored as asked: %+v", rules)
	}
}

func TestAddNotificationRuleToolValidates(t *testing.T) {
	r, _, contextID := newNotificationRegistry(t)
	ctx := context.Background()

	if _, err := r.Call(ctx, "add_notification_rule", map[string]any{
		"contextId": contextID, "matchValue": "Done", "note": "   ",
	}, Ctx{}); err == nil || !strings.Contains(err.Error(), "note is required") {
		t.Fatalf("expected a blank note to be rejected by the tool's own guard, got: %v", err)
	}

	// contextId must be caught by the tool's own guard, not by the FK constraint
	// on notification_rules.context_id — a wrong error here (e.g. a foreign key
	// violation) would mean the guard was removed and the DB caught it instead.
	if _, err := r.Call(ctx, "add_notification_rule", map[string]any{
		"contextId": "", "matchValue": "Done", "note": "n",
	}, Ctx{}); err == nil || !strings.Contains(err.Error(), "contextId is required") {
		t.Fatalf("expected a missing contextId to be rejected by the tool's own guard, got: %v", err)
	}

	// A rule on an unwired event would be stored and never fire.
	if _, err := r.Call(ctx, "add_notification_rule", map[string]any{
		"contextId": contextID, "eventType": "pr_merged", "matchValue": "merged", "note": "n",
	}, Ctx{}); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected an unsupported eventType to be rejected by the tool's own guard, got: %v", err)
	}
}

func TestListNotificationRulesTool(t *testing.T) {
	r, s, contextID := newNotificationRegistry(t)
	ctx := context.Background()

	if _, err := s.CreateNotificationRule(ctx, contextID, "jira_status_changed", "Done", "tell QA"); err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	out, err := r.Call(ctx, "list_notification_rules", map[string]any{"contextId": contextID}, Ctx{})
	if err != nil {
		t.Fatalf("call list_notification_rules: %v", err)
	}
	rules, ok := out.([]appstore.NotificationRule)
	if !ok {
		t.Fatalf("expected []NotificationRule, got %T", out)
	}
	if len(rules) != 1 || rules[0].Note != "tell QA" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestNotificationToolsAreRegistered(t *testing.T) {
	r, _, _ := newNotificationRegistry(t)
	add, ok := r.Get("add_notification_rule")
	if !ok {
		t.Fatal("add_notification_rule not registered")
	}
	if !add.Write {
		t.Fatal("add_notification_rule must be a write tool so it asks for confirmation")
	}
	list, ok := r.Get("list_notification_rules")
	if !ok {
		t.Fatal("list_notification_rules not registered")
	}
	if list.Write {
		t.Fatal("list_notification_rules must not be a write tool")
	}
}
