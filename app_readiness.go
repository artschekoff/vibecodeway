package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/hotkey"
	"github.com/artschekoff/vibecodeway/internal/perm"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Setup checklist shown at startup: permissions, credentials, tools and
// optional extras, each with a one-click way to fix it. Actions are opaque
// strings the frontend hands back to ResolveReadiness:
//   settings-pane:<anchor>  macOS Privacy & Security pane
//   open-path:<path>        reveal a file / open a folder in Finder
//   url:<https://…>         docs in the browser
//   app-settings:<tab>      a tab of the in-app Settings (handled by the frontend)

// ReadinessCheck is one row of the checklist.
type ReadinessCheck struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	Status      string `json:"status"` // ok | warn | fail | info
	Required    bool   `json:"required"`
	Action      string `json:"action,omitempty"`
	ActionLabel string `json:"actionLabel,omitempty"`
}

const (
	readyOK   = "ok"
	readyWarn = "warn"
	readyFail = "fail"
	readyInfo = "info"
)

// RunReadinessChecks evaluates every check. Each is quick (no network except
// acli's cached auth status).
func (a *App) RunReadinessChecks() []ReadinessCheck {
	settings, _ := a.db.GetAppSettings(a.ctx)
	var out []ReadinessCheck
	add := func(c ReadinessCheck) { out = append(out, c) }

	// --- macOS permissions ---
	if hotkey.CheckAccessibility() {
		add(ReadinessCheck{ID: "accessibility", Title: "Accessibility permission", Status: readyOK, Required: true,
			Detail: "Global hotkey ⌘⇧C can read the selected text from other apps."})
	} else {
		add(ReadinessCheck{ID: "accessibility", Title: "Accessibility permission", Status: readyFail, Required: true,
			Detail: "Needed for the global quick-attach hotkey. Add VibeCodeWay under Privacy & Security → Accessibility.",
			Action: "settings-pane:Privacy_Accessibility", ActionLabel: "Open System Settings"})
	}
	switch perm.MicrophoneStatus() {
	case "authorized":
		add(ReadinessCheck{ID: "microphone", Title: "Microphone access", Status: readyOK, Detail: "Granted."})
	case "denied", "restricted":
		add(ReadinessCheck{ID: "microphone", Title: "Microphone access", Status: readyWarn,
			Detail: "Denied. Only needed for voice input; enable under Privacy & Security → Microphone.",
			Action: "settings-pane:Privacy_Microphone", ActionLabel: "Open System Settings"})
	default:
		add(ReadinessCheck{ID: "microphone", Title: "Microphone access", Status: readyInfo,
			Detail: "Not requested yet. macOS asks the first time a feature uses the microphone.",
			Action: "settings-pane:Privacy_Microphone", ActionLabel: "Open System Settings"})
	}

	// --- tools & credentials ---
	if p := resolveAgentPath("acli"); !filepath.IsAbs(p) {
		add(ReadinessCheck{ID: "acli", Title: "Atlassian CLI (acli)", Status: readyFail, Required: true,
			Detail: "acli is not on PATH. Jira sync, My Board and the task graph need it.",
			Action: "url:https://developer.atlassian.com/cloud/acli/guides/install-acli/", ActionLabel: "Install guide"})
	} else if st := a.AuthStatus(); strings.HasPrefix(st, "error") || !strings.Contains(strings.ToLower(st), "site") {
		add(ReadinessCheck{ID: "acli", Title: "Atlassian CLI (acli)", Status: readyFail, Required: true,
			Detail: "Installed but not signed in. Run `acli jira auth login` in a terminal.",
			Action: "url:https://developer.atlassian.com/cloud/acli/guides/authentication/", ActionLabel: "How to sign in"})
	} else {
		add(ReadinessCheck{ID: "acli", Title: "Atlassian CLI (acli)", Status: readyOK, Required: true, Detail: strings.TrimSpace(st)})
	}
	if auth, err := a.GetVCSAuth("github"); err == nil && auth != nil && auth.Username != "" {
		add(ReadinessCheck{ID: "github", Title: "GitHub", Status: readyOK, Detail: "Signed in as " + auth.Username + "."})
	} else {
		add(ReadinessCheck{ID: "github", Title: "GitHub", Status: readyWarn,
			Detail: "Not connected. PR tracking, code reviews and branch linking stay off.",
			Action: "app-settings:accounts", ActionLabel: "Connect"})
	}
	if a.HasSlackPassphrase() {
		add(ReadinessCheck{ID: "slack", Title: "Slack passphrase (Keychain)", Status: readyOK, Detail: "Passphrase loaded; Slack threads can be fetched."})
	} else {
		add(ReadinessCheck{ID: "slack", Title: "Slack passphrase (Keychain)", Status: readyWarn,
			Detail: "Not entered this session. You will be asked when a context needs Slack; set SLACK_MCP_PASSPHRASE to skip the prompt.",
			Action: "app-settings:accounts", ActionLabel: "Open Accounts"})
	}
	if ai, err := a.GetAISettings(); err == nil && ai != nil && ai.HasKey {
		add(ReadinessCheck{ID: "ai", Title: "AI provider key", Status: readyOK, Detail: fmt.Sprintf("%s · %s", ai.Provider, ai.Model)})
	} else {
		add(ReadinessCheck{ID: "ai", Title: "AI provider key", Status: readyWarn,
			Detail: "No API key. Ask AI, Todo Next and AI matching are disabled.",
			Action: "app-settings:ai", ActionLabel: "Add key"})
	}

	// --- coding agents & terminal ---
	var agents []string
	for _, bin := range []string{"claude", "codex"} {
		if p := resolveAgentPath(bin); filepath.IsAbs(p) {
			agents = append(agents, bin)
		}
	}
	if len(agents) > 0 {
		add(ReadinessCheck{ID: "agents", Title: "Coding agents", Status: readyOK,
			Detail: "Found: " + strings.Join(agents, ", ") + "."})
	} else {
		add(ReadinessCheck{ID: "agents", Title: "Coding agents", Status: readyWarn,
			Detail: "Neither claude nor codex found on PATH. Terminals still work; agent launch buttons will fail.",
			Action: "url:https://docs.anthropic.com/en/docs/claude-code/setup", ActionLabel: "Install Claude Code"})
	}
	st := a.AgentNotificationsStatus()
	switch {
	case st.ClaudeBell && st.CodexNotifications:
		add(ReadinessCheck{ID: "agent-notify", Title: "Agent notifications", Status: readyOK, Detail: "Claude Code bell and Codex notifications are on."})
	default:
		add(ReadinessCheck{ID: "agent-notify", Title: "Agent notifications", Status: readyInfo,
			Detail: "Enable so the app can tell when an agent waits for input (Claude Code: " + onOff(st.ClaudeBell) + ", Codex: " + onOff(st.CodexNotifications) + ").",
			Action: "app-settings:terminal", ActionLabel: "Enable"})
	}
	if settings != nil && settings.TerminalAppPath == "" && settings.CodingAgentApp == "" {
		add(ReadinessCheck{ID: "terminal", Title: "External terminal / agent app", Status: readyInfo,
			Detail: "Nothing configured; checkouts open in the in-app terminal only.",
			Action: "app-settings:terminal", ActionLabel: "Configure"})
	} else if settings != nil {
		add(ReadinessCheck{ID: "terminal", Title: "External terminal / agent app", Status: readyOK,
			Detail: strings.TrimSpace(settings.TerminalAppPath + " " + settings.CodingAgentApp)})
	}
	if settings != nil && settings.JiraOrgField == "" {
		add(ReadinessCheck{ID: "jira-scope", Title: "Jira scope field", Status: readyInfo,
			Detail: "Optional: a custom field that groups My Board by service.",
			Action: "app-settings:sync", ActionLabel: "Set"})
	} else if settings != nil {
		add(ReadinessCheck{ID: "jira-scope", Title: "Jira scope field", Status: readyOK, Detail: settings.JiraOrgField})
	}

	// --- optional extras ---
	dataDir := filepath.Dir(appstore.DefaultDBPath())
	licPath := filepath.Join(dataDir, "license.json")
	if _, err := os.Stat(licPath); err == nil {
		add(ReadinessCheck{ID: "license", Title: "License", Status: readyOK, Detail: licPath, Action: "open-path:" + licPath, ActionLabel: "Show in Finder"})
	} else {
		add(ReadinessCheck{ID: "license", Title: "License", Status: readyInfo,
			Detail: "No license file. This build does not require one.", Action: "open-path:" + dataDir, ActionLabel: "Open data folder"})
	}
	return out
}

// ResolveReadiness performs the fix action for a check (see the prefixes above).
// app-settings:* is a no-op here — the frontend routes it.
func (a *App) ResolveReadiness(action string) error {
	kind, arg, ok := strings.Cut(action, ":")
	if !ok {
		return fmt.Errorf("bad action %q", action)
	}
	switch kind {
	case "settings-pane":
		if arg == "Privacy_Accessibility" {
			a.OpenAccessibilityPreferences() // opens the pane and polls until granted
			return nil
		}
		return exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?"+arg).Run()
	case "open-path":
		info, err := os.Stat(arg)
		switch {
		case err != nil:
			return exec.Command("open", filepath.Dir(arg)).Run()
		case info.IsDir():
			return exec.Command("open", arg).Run()
		default:
			return exec.Command("open", "-R", arg).Run()
		}
	case "url":
		runtime.BrowserOpenURL(a.ctx, arg)
		return nil
	case "app-settings":
		return nil
	}
	return fmt.Errorf("unknown action %q", action)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
