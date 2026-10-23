package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runDescribe(t *testing.T) map[string]any {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"describe"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("describe output is not valid JSON: %v\n%s", err, buf.String())
	}
	return got
}

func TestDescribe_EmitsBinaryAndCommands(t *testing.T) {
	got := runDescribe(t)
	if got["binary"] != "vibecodeway-cli" {
		t.Fatalf("binary field wrong: %v", got["binary"])
	}
	cmds, ok := got["commands"].([]any)
	if !ok || len(cmds) == 0 {
		t.Fatalf("commands array missing or empty")
	}
}

func TestDescribe_IncludesCoreCommands(t *testing.T) {
	got := runDescribe(t)
	names := map[string]bool{}
	for _, c := range got["commands"].([]any) {
		m := c.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, want := range []string{"describe", "context-list", "context-get", "add-text", "jira-search", "slack-list-workspaces", "repo-list"} {
		if !names[want] {
			t.Errorf("expected %q in commands, got %v", want, names)
		}
	}
}

func TestDescribe_EveryCommandHasAgentDocs(t *testing.T) {
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"describe"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Input       string `json:"input"`
			Output      string `json:"output"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	for _, c := range doc.Commands {
		if strings.HasPrefix(c.Name, "completion") {
			t.Errorf("shell-completion command %q leaked into the schema", c.Name)
		}
		if strings.TrimSpace(c.Description) == "" {
			t.Errorf("command %q missing description", c.Name)
		}
		if strings.TrimSpace(c.Input) == "" {
			t.Errorf("command %q missing input", c.Name)
		}
		if strings.TrimSpace(c.Output) == "" {
			t.Errorf("command %q missing output schema", c.Name)
		}
	}
}

func TestDescribe_LongHelpHasSections(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"context-list"})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	for _, section := range []string{"Description:", "Input:", "Output:"} {
		if !strings.Contains(cmd.Long, section) {
			t.Errorf("context-list Long missing %q:\n%s", section, cmd.Long)
		}
	}
}

func TestDescribe_IncludesAddToContextCommands(t *testing.T) {
	got := runDescribe(t)
	names := map[string]bool{}
	for _, c := range got["commands"].([]any) {
		names[c.(map[string]any)["name"].(string)] = true
	}
	// The /vibecodeway-add-to-context skill drives exactly these verbs.
	for _, want := range []string{"branch-add", "event-add", "org-list", "jira-add", "add-text", "context-create"} {
		if !names[want] {
			t.Errorf("expected %q in commands, got %v", want, names)
		}
	}
}

func TestDescribe_JiraAddAcceptsContextKey(t *testing.T) {
	got := runDescribe(t)
	for _, c := range got["commands"].([]any) {
		m := c.(map[string]any)
		if m["name"] != "jira-add" {
			continue
		}
		for _, f := range m["flags"].([]any) {
			if f.(map[string]any)["name"] == "context-key" {
				return
			}
		}
		t.Fatalf("jira-add must expose --context-key, flags: %v", m["flags"])
	}
	t.Fatal("jira-add not found in describe output")
}

func TestDescribe_ContextGetOutputMatchesNestedShape(t *testing.T) {
	got := runDescribe(t)
	for _, c := range got["commands"].([]any) {
		m := c.(map[string]any)
		if m["name"] != "context-get" {
			continue
		}
		out, _ := m["output"].(string)
		// Agents historically invented flat keys (jiraConfig, .repos, .texts).
		// The live payload nests metadata under .context and uses vcsBranches / contextTexts.
		for _, want := range []string{`"context"`, `vcsBranches`, `contextTexts`, `pullRequests`, `issues`} {
			if !strings.Contains(out, want) {
				t.Errorf("context-get output schema missing %q:\n%s", want, out)
			}
		}
		for _, stale := range []string{`jiraConfig`, `slackConfig`} {
			if strings.Contains(out, stale) {
				t.Errorf("context-get output schema still documents stale key %q:\n%s", stale, out)
			}
		}
		return
	}
	t.Fatal("context-get not found in describe output")
}

func TestDescribe_TextListRejectsContextKey(t *testing.T) {
	got := runDescribe(t)
	for _, c := range got["commands"].([]any) {
		m := c.(map[string]any)
		if m["name"] != "text-list" {
			continue
		}
		input, _ := m["input"].(string)
		if !strings.Contains(input, "--context-id") {
			t.Fatalf("text-list input must require --context-id, got %q", input)
		}
		if strings.Contains(input, "--context-key") {
			t.Fatalf("text-list must not advertise --context-key, got %q", input)
		}
		if flags, ok := m["flags"].([]any); ok {
			for _, f := range flags {
				if f.(map[string]any)["name"] == "context-key" {
					t.Fatal("text-list must not expose --context-key flag")
				}
			}
		}
		return
	}
	t.Fatal("text-list not found in describe output")
}
