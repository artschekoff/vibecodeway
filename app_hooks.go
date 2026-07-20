package main

import (
	"os"
	"os/exec"

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
	cmd.Env = append(os.Environ(),
		"CONTEXT_KEY="+contextKey,
		"CONTEXT_ID="+contextID,
		"EVENT_CATEGORY="+eventCategory,
	)
	return cmd.Run()
}

// dispatchContextHooks runs every enabled hook for (contextID, eventCategory).
// Best-effort: it never returns an error and never blocks the caller on failure.
func (a *App) dispatchContextHooks(contextID, eventCategory string) {
	hooks, err := a.db.ListEnabledHooks(a.ctx, contextID, eventCategory)
	if err != nil {
		a.logger.Warn("hooks: list enabled failed",
			zap.String("contextID", contextID), zap.String("category", eventCategory), zap.Error(err))
		return
	}
	if len(hooks) == 0 {
		return
	}
	c, err := a.db.GetContext(a.ctx, contextID)
	if err != nil {
		a.logger.Warn("hooks: get context failed",
			zap.String("contextID", contextID), zap.Error(err))
		return
	}
	for _, h := range hooks {
		switch h.ActionType {
		case "os_notification":
			runtime.EventsEmit(a.ctx, "hook:notify", map[string]interface{}{
				"contextId":     c.ID,
				"contextKey":    c.Key,
				"eventCategory": eventCategory,
			})
		case "run_script":
			if err := runScriptHook(h.ScriptCommand, c.Key, c.ID, eventCategory); err != nil {
				a.logger.Warn("hooks: script failed",
					zap.String("contextID", contextID), zap.String("category", eventCategory),
					zap.Error(err))
			}
		}
	}
}
