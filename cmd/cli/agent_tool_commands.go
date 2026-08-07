package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artschekoff/context-consolidator/internal/tooling"
	"github.com/spf13/cobra"
)

func parseToolArgs(raw string) (map[string]any, error) {
	args := map[string]any{}
	if raw == "" {
		return args, nil
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("--args must be a JSON object: %w", err)
	}
	return args, nil
}

func guardWrite(isWrite, yes bool) error {
	if isWrite && !yes {
		return fmt.Errorf("this tool mutates state; re-run with --yes to confirm")
	}
	return nil
}

// toolingCtxFromEnv returns the default Ctx for CLI calls. The CLI is not
// org-scoped, so OrgID is empty (search across all orgs).
// ponytail: no org scoping on the CLI; add a --org flag if multi-org CLI use appears.
func toolingCtxFromEnv() tooling.Ctx { return tooling.Ctx{} }

func newAgentToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-tool",
		Short: "List and invoke the shared agent tools (mirrors the Ask AI tool registry)",
	}
	cmd.AddCommand(newAgentToolListCmd(), newAgentToolCallCmd(), newAgentToolDocCmd())
	return setAgentDocs(cmd,
		"Parent command for the agent tool registry. `list` prints the tools the Ask AI agent can call; `call` invokes one by name with JSON args; `doc` renders TOOLING.md.",
		"(none)",
		`see subcommands`,
		`context-consolidator-cli agent-tool list`)
}

func newAgentToolListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available agent tools as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			type toolView struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Write       bool           `json:"write"`
				Params      map[string]any `json:"params"`
			}
			var out []toolView
			for _, t := range env.toolRegistry().List() {
				out = append(out, toolView{t.Name, t.Description, t.Write, t.Params})
			}
			return writeJSON(out)
		},
	}
	return setAgentDocs(cmd,
		"Lists every agent tool: name, description, write flag, and JSON-schema params. This is the machine-readable capability list the Ask AI window also shows.",
		"(none)",
		`[{"name":"string","description":"string","write":"bool","params":"json schema"}]`,
		`context-consolidator-cli agent-tool list | jq '.[].name'`)
}

func newAgentToolCallCmd() *cobra.Command {
	var name, argsJSON string
	var yes bool
	cmd := &cobra.Command{
		Use:   "call",
		Short: "Invoke one agent tool by name with JSON args",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			args, err := parseToolArgs(argsJSON)
			if err != nil {
				return err
			}
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			reg := env.toolRegistry()
			tool, ok := reg.Get(name)
			if !ok {
				return fmt.Errorf("unknown tool %q (see `agent-tool list`)", name)
			}
			if err := guardWrite(tool.Write, yes); err != nil {
				return err
			}
			payload, err := reg.Call(context.Background(), name, args, toolingCtxFromEnv())
			if err != nil {
				return err
			}
			return writeJSON(payload)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (see `agent-tool list`)")
	cmd.Flags().StringVar(&argsJSON, "args", "", "Tool arguments as a JSON object")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm execution of a write (state-mutating) tool")
	return setAgentDocs(cmd,
		"Invokes one agent tool by name. --args is a JSON object matching the tool's params (see `agent-tool list`). Write tools require --yes.",
		"--name STRING (required)\n--args JSON (optional)\n--yes BOOL (required for write tools)",
		`the tool's JSON payload`,
		`context-consolidator-cli agent-tool call --name search_contexts --args '{"query":"sprint"}'`)
}

func newAgentToolDocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Render docs/TOOLING.md for the agent tool registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			fmt.Fprint(cmd.OutOrStdout(), tooling.RenderMarkdown(env.toolRegistry()))
			return nil
		},
	}
	return setAgentDocs(cmd,
		"Renders the Markdown reference for every agent tool. Piped to docs/TOOLING.md by `make tooling-doc`.",
		"(none)", "Markdown on stdout",
		`context-consolidator-cli agent-tool doc > docs/TOOLING.md`)
}
