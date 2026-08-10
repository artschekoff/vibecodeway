package tooling

import (
	"fmt"
	"sort"
	"strings"
)

// RenderMarkdown produces docs/TOOLING.md from the registry. Deterministic:
// tools in registration order, params sorted by name.
func RenderMarkdown(r *Registry) string {
	var b strings.Builder
	b.WriteString("# Agent Tooling\n\n")
	b.WriteString("Generated from the tool registry — do not edit by hand. Regenerate with `make tooling-doc`.\n\n")
	b.WriteString("These are the actions the Ask AI agent can take. The CLI mirrors them:\n")
	b.WriteString("`vibecodeway-cli agent-tool list` and `... agent-tool call --name <tool> --args '{...}'`.\n")
	b.WriteString("Tools marked **Write: yes** mutate state and require confirmation (Ask AI) or `--yes` (CLI).\n\n")

	for _, t := range r.List() {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", t.Name, t.Description)
		if t.Write {
			b.WriteString("**Write:** yes\n\n")
		} else {
			b.WriteString("**Write:** no\n\n")
		}
		writeParams(&b, t.Params)
		fmt.Fprintf(&b, "```sh\nvibecodeway-cli agent-tool call --name %s --args '{}'\n```\n\n", t.Name)
	}
	return b.String()
}

func writeParams(b *strings.Builder, params map[string]any) {
	props, _ := params["properties"].(map[string]any)
	if len(props) == 0 {
		b.WriteString("_No parameters._\n\n")
		return
	}
	required := map[string]bool{}
	if req, ok := params["required"].([]string); ok {
		for _, r := range req {
			required[r] = true
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	b.WriteString("| Param | Type | Required | Description |\n|---|---|---|---|\n")
	for _, name := range names {
		p, _ := props[name].(map[string]any)
		typ, _ := p["type"].(string)
		desc, _ := p["description"].(string)
		req := "no"
		if required[name] {
			req = "yes"
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", name, typ, req, desc)
	}
	b.WriteString("\n")
}
