package main

import (
	"os"
	"os/exec"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// scriptOutputTailLimit caps how much combined stdout+stderr of a user script
// is kept for the app log — enough to diagnose a failing hook without letting
// a chatty script (npm install…) flood the log store.
const scriptOutputTailLimit = 4 * 1024

// tailString returns at most the last limit bytes of s.
func tailString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return "…" + s[len(s)-limit:]
}

// runScriptHook runs a user shell command for a hook. The context key is passed
// both as $1 and via CONTEXT_KEY; CONTEXT_ID and EVENT_CATEGORY are also exported.
// Synchronous; returns the script's combined stdout+stderr and the command error.
func runScriptHook(command, contextKey, contextID, eventCategory string) (string, error) {
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
	out, err := cmd.CombinedOutput()
	return string(out), err
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
		// Both outcomes land in the app log with the script's output tail, so a
		// hook that "silently misbehaves" is diagnosable from the Logs page.
		out, err := runScriptHook(scriptCommand, c.Key, c.ID, eventCategory)
		if err != nil {
			a.logger.Warn("hooks: script failed",
				zap.String("contextID", c.ID), zap.String("category", eventCategory),
				zap.String("command", scriptCommand),
				zap.String("output", tailString(out, scriptOutputTailLimit)),
				zap.Error(err))
		} else {
			a.logger.Info("hooks: script ok",
				zap.String("contextID", c.ID), zap.String("category", eventCategory),
				zap.String("command", scriptCommand),
				zap.String("output", tailString(out, scriptOutputTailLimit)))
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
