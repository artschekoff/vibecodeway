package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Agent "waiting for input" notifications. The in-app terminal listens for
// the signals coding agents already know how to send — BEL (Claude Code with
// preferredNotifChannel=terminal_bell) and OSC 9 (Codex with
// tui.notifications) — so enabling the feature means flipping those two
// settings in the agents' own config files.

// AgentNotifyStatus reports which agents are configured to signal.
type AgentNotifyStatus struct {
	ClaudeBell         bool   `json:"claudeBell"`
	CodexNotifications bool   `json:"codexNotifications"`
	ClaudePath         string `json:"claudePath"`
	CodexPath          string `json:"codexPath"`
}

func agentConfigPaths() (claude, codex string) {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".codex", "config.toml")
}

// AgentNotificationsStatus reads both configs (missing file = not enabled).
func (a *App) AgentNotificationsStatus() AgentNotifyStatus {
	cp, xp := agentConfigPaths()
	st := AgentNotifyStatus{ClaudePath: cp, CodexPath: xp}
	if b, err := os.ReadFile(cp); err == nil {
		st.ClaudeBell = claudeBellEnabled(string(b))
	}
	if b, err := os.ReadFile(xp); err == nil {
		st.CodexNotifications = codexNotificationsEnabled(string(b))
	}
	return st
}

// EnableAgentNotifications patches ~/.claude/settings.json and
// ~/.codex/config.toml in place (text edits, nothing else reformatted) and
// returns the resulting status. A missing config file is created.
func (a *App) EnableAgentNotifications() (AgentNotifyStatus, error) {
	cp, xp := agentConfigPaths()
	if err := patchFile(cp, patchClaudeSettings); err != nil {
		return a.AgentNotificationsStatus(), fmt.Errorf("claude settings: %w", err)
	}
	if err := patchFile(xp, patchCodexConfig); err != nil {
		return a.AgentNotificationsStatus(), fmt.Errorf("codex config: %w", err)
	}
	return a.AgentNotificationsStatus(), nil
}

func patchFile(path string, patch func(string) (string, error)) error {
	src := ""
	if b, err := os.ReadFile(path); err == nil {
		src = string(b)
	} else if !os.IsNotExist(err) {
		return err
	}
	out, err := patch(src)
	if err != nil {
		return err
	}
	if out == src {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

var claudeNotifRe = regexp.MustCompile(`"preferredNotifChannel"\s*:\s*"[^"]*"`)

func claudeBellEnabled(src string) bool {
	m := claudeNotifRe.FindString(src)
	return strings.Contains(m, `"terminal_bell"`) || strings.Contains(m, `"iterm2_with_bell"`)
}

// patchClaudeSettings sets preferredNotifChannel to terminal_bell, editing the
// text so key order and formatting of the user's file survive.
func patchClaudeSettings(src string) (string, error) {
	if claudeBellEnabled(src) {
		return src, nil
	}
	out := src
	if claudeNotifRe.MatchString(src) {
		out = claudeNotifRe.ReplaceAllString(src, `"preferredNotifChannel": "terminal_bell"`)
	} else if i := strings.Index(src, "{"); i >= 0 {
		rest := strings.TrimLeft(src[i+1:], " \t\r\n")
		sep := ","
		if strings.HasPrefix(rest, "}") {
			sep = ""
		}
		out = src[:i+1] + "\n  \"preferredNotifChannel\": \"terminal_bell\"" + sep + "\n" + src[i+1:]
	} else {
		out = "{\n  \"preferredNotifChannel\": \"terminal_bell\"\n}\n"
	}
	if !json.Valid([]byte(out)) {
		return "", fmt.Errorf("refusing to write invalid JSON")
	}
	return out, nil
}

var (
	tomlSectionRe   = regexp.MustCompile(`(?m)^[ \t]*\[([^\]]+)\][ \t]*$`)
	codexNotifLine  = regexp.MustCompile(`(?m)^[ \t]*notifications[ \t]*=.*$`)
	codexNotifValue = "notifications = true"
)

// tuiSection returns the [start, end) byte range of the [tui] section body
// (after the header line), or -1,-1 when absent.
func tuiSection(src string) (int, int) {
	locs := tomlSectionRe.FindAllStringSubmatchIndex(src, -1)
	for i, l := range locs {
		if strings.TrimSpace(src[l[2]:l[3]]) != "tui" {
			continue
		}
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		return l[1], end
	}
	return -1, -1
}

func codexNotificationsEnabled(src string) bool {
	s, e := tuiSection(src)
	if s < 0 {
		return false
	}
	m := codexNotifLine.FindString(src[s:e])
	if m == "" {
		return false
	}
	v := strings.TrimSpace(strings.SplitN(m, "=", 2)[1])
	return v == "true" || strings.HasPrefix(v, "[")
}

// patchCodexConfig sets notifications = true inside [tui], creating the
// section when missing. Other sections are untouched.
func patchCodexConfig(src string) (string, error) {
	if codexNotificationsEnabled(src) {
		return src, nil
	}
	s, e := tuiSection(src)
	if s < 0 {
		sep := "\n"
		if src == "" || strings.HasSuffix(src, "\n\n") {
			sep = ""
		} else if strings.HasSuffix(src, "\n") {
			sep = "\n"
		} else {
			sep = "\n\n"
		}
		return src + sep + "[tui]\n" + codexNotifValue + "\n", nil
	}
	body := src[s:e]
	if codexNotifLine.MatchString(body) {
		return src[:s] + codexNotifLine.ReplaceAllString(body, codexNotifValue) + src[e:], nil
	}
	// s sits at the end of the "[tui]" header line: insert right below it.
	return src[:s] + "\n" + codexNotifValue + src[s:], nil
}

// NotifyNative shows a macOS notification (osascript; no extra dependency).
func (a *App) NotifyNative(title, body string) error {
	esc := func(s string) string {
		return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
	}
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, esc(body), esc(title))
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	return nil
}
