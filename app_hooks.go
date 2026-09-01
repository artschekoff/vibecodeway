package main

import (
	"os"
	"os/exec"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// runScriptHook runs a user shell command for a hook. The context key is passed
// both as $1 and via CONTEXT_KEY; CONTEXT_ID and EVENT_CATEGORY are also exported.
// Synchronous; returns the command error if any.
func runScriptHook(command, contextKey, contextID, eventCategory string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	cmd := exec.Command(shell, "-c", command, "hook", contextKey)
	cmd.Env = append(
		os.Environ(),
		"CONTEXT_KEY="+contextKey,
		"CONTEXT_ID="+contextID,
		"EVENT_CATEGORY="+eventCategory,
	)
	return cmd.Run()
}

// hookDetail carries the user-authored half of a notification-rule match into
// the hook payload: the generated "your condition fired" line and the note the
// user wrote. The zero value means no rule matched, which is the normal case.
type hookDetail struct {
	ConditionText string
	Note          string
}

// runHook executes a single resolved hook action for context c. detail is empty
// unless a notification rule matched the event that triggered this dispatch.
func (a *App) runHook(c *appstore.Context, eventCategory, actionType, scriptCommand string, detail hookDetail) {
	switch actionType {
	case "os_notification":
		runtime.EventsEmit(a.ctx, "hook:notify", map[string]interface{}{
			"contextId":     c.ID,
			"contextKey":    c.Key,
			"eventCategory": eventCategory,
			"conditionText": detail.ConditionText,
			"note":          detail.Note,
		})
	case "run_script":
		if err := runScriptHook(scriptCommand, c.Key, c.ID, eventCategory); err != nil {
			a.logger.Warn("hooks: script failed",
				zap.String("contextID", c.ID), zap.String("category", eventCategory), zap.Error(err))
		}
	}
}

// dispatchContextHooks runs the resolved hooks for (contextID, eventCategory):
// the context's own hooks if any, otherwise the global hooks for the category.
// Best-effort: never returns an error, never blocks on failure.
func (a *App) dispatchContextHooks(contextID, eventCategory string, detail hookDetail) {
	ctxHooks, err := a.db.ListEnabledHooks(a.ctx, contextID, eventCategory)
	if err != nil {
		a.logger.Warn("hooks: list enabled failed",
			zap.String("contextID", contextID), zap.String("category", eventCategory), zap.Error(err))
		return
	}

	// Resolve action set: per-context overrides global for this category.
	type resolved struct{ actionType, scriptCommand string }
	var actions []resolved
	if len(ctxHooks) > 0 {
		for _, h := range ctxHooks {
			actions = append(actions, resolved{h.ActionType, h.ScriptCommand})
		}
	} else {
		globalHooks, gErr := a.db.ListEnabledGlobalHooks(a.ctx, eventCategory)
		if gErr != nil {
			a.logger.Warn("hooks: list global failed",
				zap.String("category", eventCategory), zap.Error(gErr))
			return
		}
		for _, h := range globalHooks {
			actions = append(actions, resolved{h.ActionType, h.ScriptCommand})
		}
	}
	if len(actions) == 0 {
		return
	}

	c, err := a.db.GetContext(a.ctx, contextID)
	if err != nil {
		a.logger.Warn("hooks: get context failed",
			zap.String("contextID", contextID), zap.Error(err))
		return
	}
	for _, act := range actions {
		a.runHook(c, eventCategory, act.actionType, act.scriptCommand, detail)
	}
}
