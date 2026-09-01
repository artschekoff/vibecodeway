package tooling

import (
	"context"
	"fmt"
	"strings"
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
			return map[string]any{
				"id":         rule.ID,
				"contextId":  rule.ContextID,
				"eventType":  rule.EventType,
				"matchValue": rule.MatchValue,
				"note":       rule.Note,
			}, nil
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
