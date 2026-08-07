package agent

// ToolInfo is a user-facing description of a tool the agent can call.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Write       bool   `json:"write"`
}

// ToolCatalogue returns the tools available to the agent, for display in the UI.
// It reads from the shared registry so it always matches what the model sees.
func (a *Agent) ToolCatalogue() []ToolInfo {
	tools := a.reg.List()
	out := make([]ToolInfo, 0, len(tools))
	for _, t := range tools {
		out = append(out, ToolInfo{Name: t.Name, Description: t.Description, Write: t.Write})
	}
	return out
}
