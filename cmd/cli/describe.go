package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type flagDoc struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

type commandDoc struct {
	Name        string    `json:"name"`
	Short       string    `json:"short"`
	Description string    `json:"description,omitempty"`
	Input       string    `json:"input,omitempty"`
	Output      string    `json:"output,omitempty"`
	Example     string    `json:"example,omitempty"`
	Long        string    `json:"long,omitempty"`
	Flags       []flagDoc `json:"flags"`
}

type interfaceDoc struct {
	Binary   string       `json:"binary"`
	Version  string       `json:"version"`
	Commands []commandDoc `json:"commands"`
}

// setAgentDocs attaches machine-readable fields for `describe`.
// description / input / output appear in both Long help and the JSON schema.
func setAgentDocs(cmd *cobra.Command, description, input, output, example string) *cobra.Command {
	long := strings.Builder{}
	long.WriteString("Description:\n  ")
	long.WriteString(strings.ReplaceAll(description, "\n", "\n  "))
	long.WriteString("\n\nInput:\n")
	if strings.TrimSpace(input) == "" {
		long.WriteString("  (none)\n")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(input), "\n") {
			long.WriteString("  ")
			long.WriteString(line)
			long.WriteString("\n")
		}
	}
	long.WriteString("\nOutput:\n  ")
	long.WriteString(strings.ReplaceAll(output, "\n", "\n  "))
	cmd.Long = strings.TrimRight(long.String(), "\n")
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["description"] = description
	cmd.Annotations["input"] = input
	cmd.Annotations["output"] = output
	cmd.Annotations["example"] = example
	if example != "" {
		cmd.Example = "  " + strings.ReplaceAll(example, "\n", "\n  ")
	}
	return cmd
}

// collect walks the tree, appending a doc for every runnable (leaf) command.
func collect(cmd *cobra.Command, prefix string, out *[]commandDoc) {
	name := strings.TrimSpace(prefix + " " + cmd.Name())
	if cmd.Runnable() && cmd.Name() != "help" {
		cd := commandDoc{
			Name:        name,
			Short:       cmd.Short,
			Description: cmd.Annotations["description"],
			Input:       cmd.Annotations["input"],
			Output:      cmd.Annotations["output"],
			Example:     cmd.Annotations["example"],
			Long:        cmd.Long,
		}
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			cd.Flags = append(cd.Flags, flagDoc{
				Name:        f.Name,
				Shorthand:   f.Shorthand,
				Type:        f.Value.Type(),
				Default:     f.DefValue,
				Description: f.Usage,
			})
		})
		*out = append(*out, cd)
	}
	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}
		collect(c, name, out)
	}
}

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Emit the full CLI interface as JSON for machine consumption",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			doc := interfaceDoc{Binary: root.Name(), Version: root.Version}
			for _, c := range root.Commands() {
				if c.Hidden {
					continue
				}
				collect(c, "", &doc.Commands)
			}
			out, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	return setAgentDocs(cmd,
		"Prints a JSON document describing every command: name, short help, description, input, output JSON shape, flags, and an example. Consumed by coding agents that need a schema of the CLI without parsing text help.",
		"(none)",
		`{"binary":"vibecodeway-cli","version":"string","commands":[{"name":"string","short":"string","description":"string","input":"string","output":"json shape string","example":"string","flags":[{"name":"string","type":"string","default":"string","description":"string"}]}]}`,
		`vibecodeway-cli describe | jq '.commands[].name'`,
	)
}
