package tooling

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	r := &Registry{}
	r.Register(Tool{Name: "search_contexts", Description: "Find contexts.", Params: map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "keywords"}},
		"required": []string{"query"},
	}})
	r.Register(Tool{Name: "edit_context", Description: "Edit a context.", Write: true, Params: map[string]any{
		"type": "object", "properties": map[string]any{"contextId": map[string]any{"type": "string", "description": "the id"}},
		"required": []string{"contextId"},
	}})

	md := RenderMarkdown(r)
	for _, want := range []string{"# Agent Tooling", "## search_contexts", "## edit_context", "`query`", "`contextId`", "**Write:** yes"} {
		if !strings.Contains(md, want) {
			t.Fatalf("RenderMarkdown missing %q\n---\n%s", want, md)
		}
	}
	if strings.Index(md, "## search_contexts") > strings.Index(md, "## edit_context") {
		t.Fatal("tools must render in registration order")
	}
}
