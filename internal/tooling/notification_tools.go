package tooling

import (
	"context"
	"fmt"
	"strings"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

// defaultRuleEventType is what a rule watches when the caller does not say.
const defaultRuleEventType = "jira_status_changed"

// supportedRuleEventTypes are the events the notify pipeline actually checks
// rules against. A rule on anything else would be stored and never fire, so the
// tool rejects it rather than leaving the user with a silent dud.
//
// ponytail: one entry, because only the Jira path is wired. Adding PR rules is
// this list plus the same three lines in onVCSNotify.
var supportedRuleEventTypes = map[string]bool{
	"jira_status_changed": true,
}

// osNotificationConfiguredFor reports whether an os_notification hook would
// actually fire for eventCategory on contextID, mirroring the resolution
// App.dispatchContextHooks performs at hook-dispatch time: the context's own
// enabled hooks for the category, falling back to the global ones only when
// the context has none at all. Best-effort — any lookup error is reported as
// "not configured" rather than surfaced as a tool-call failure, since the
// caller (add_notification_rule) already saved the rule by the time this runs.
func osNotificationConfiguredFor(ctx context.Context, s *appstore.Store, contextID, eventCategory string) bool {
	ctxHooks, err := s.ListEnabledHooks(ctx, contextID, eventCategory)
	if err != nil {
		return false
	}
	if len(ctxHooks) > 0 {
		for _, h := range ctxHooks {
			if h.ActionType == "os_notification" {
				return true
			}
		}
		return false // context has hooks for this category, but none is os_notification — no global fallback
	}

	globalHooks, err := s.ListEnabledGlobalHooks(ctx, eventCategory)
	if err != nil {
		return false
	}
	for _, h := range globalHooks {
		if h.ActionType == "os_notification" {
			return true
		}
	}
	return false
}

func registerNotificationTools(r *Registry, d Deps) {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}

	r.Register(Tool{
		Name: "add_notification_rule",
		Description: "Create a notification rule on a context: when a Jira issue's status changes to matchValue, " +
			"notify the user and show them `note` verbatim. Example: matchValue \"Done\" with note \"tell QA we are ready\". " +
			"Leave matchValue empty to fire on any status change.",
		Write: true,
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"contextId":  strProp("the context id the rule watches"),
				"note":       strProp("the user's own text, shown verbatim in the notification (required)"),
				"matchValue": strProp("the new status that fires the rule, e.g. Done; empty means any status change"),
				"eventType":  strProp("which event to watch; only jira_status_changed is supported (the default)"),
			},
			"required": []string{"contextId", "note"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID := strings.TrimSpace(strArg(args, "contextId"))
			note := strings.TrimSpace(strArg(args, "note"))
			if contextID == "" {
				return nil, fmt.Errorf("contextId is required")
			}
			if note == "" {
				return nil, fmt.Errorf("note is required — it is the text the user will be shown")
			}
			eventType := strings.TrimSpace(strArg(args, "eventType"))
			if eventType == "" {
				eventType = defaultRuleEventType
			}
			if !supportedRuleEventTypes[eventType] {
				return nil, fmt.Errorf("eventType %q is not supported; only %q fires rules today",
					eventType, defaultRuleEventType)
			}

			rule, err := d.Store.CreateNotificationRule(ctx, contextID, eventType,
				strings.TrimSpace(strArg(args, "matchValue")), note)
			if err != nil {
				return nil, err
			}

			result := map[string]any{
				"id":         rule.ID,
				"contextId":  rule.ContextID,
				"eventType":  rule.EventType,
				"matchValue": rule.MatchValue,
				"note":       rule.Note,
			}
			// The rule is already saved by this point. Whether a desktop popup can
			// ever fire depends on a separate feature (event hooks) that this tool
			// neither reads nor mentions elsewhere, so tell the user now rather than
			// let them discover a silent rule later. Best-effort: a failed lookup
			// reports "not configured" (with a hint) rather than failing the whole
			// call — the rule was created either way.
			configured := osNotificationConfiguredFor(ctx, d.Store, contextID, "jira_update")
			result["osNotificationConfigured"] = configured
			if !configured {
				result["hint"] = "This rule is saved and will appear in the Activity Feed, but no desktop " +
					"popup will show for it yet: Jira updates have no os_notification hook enabled for this " +
					"context (or globally). Enable OS notifications for Jira updates in Settings to also get a popup."
			}
			return result, nil
		},
	})

	r.Register(Tool{
		Name:        "list_notification_rules",
		Description: "List the notification rules on a context, so the user can see what they will be notified about.",
		Params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"contextId": strProp("the context id")},
			"required":   []string{"contextId"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID := strings.TrimSpace(strArg(args, "contextId"))
			if contextID == "" {
				return nil, fmt.Errorf("contextId is required")
			}
			return d.Store.ListNotificationRules(ctx, contextID)
		},
	})
}
