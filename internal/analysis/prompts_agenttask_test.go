package analysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAgentTaskPrompt_startsWithSkillHeader(t *testing.T) {
	// Bare skill names, no leading slash — Codex does not resolve slash commands.
	assert.True(t, strings.HasPrefix(defaultAgentTaskPrompt, "superpowers:writing-plans\nvibecodeway-load-context {{context_key}}\n"),
		"prompt must start with the two skill lines (no slashes)")
	assert.Contains(t, defaultAgentTaskPrompt, "{{repos}}")
	assert.Contains(t, defaultAgentTaskPrompt, "{{user_prompt}}")
	assert.Contains(t, defaultAgentTaskPrompt, "{{tickets}}")
}
