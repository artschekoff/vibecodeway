package analysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAgentTaskPrompt_startsWithSkillHeader(t *testing.T) {
	assert.True(t, strings.HasPrefix(defaultAgentTaskPrompt, "/writing-plans\n/arts-load-context {{context_key}}\n"),
		"prompt must start with the two skill-command lines")
	assert.Contains(t, defaultAgentTaskPrompt, "{{repos}}")
	assert.Contains(t, defaultAgentTaskPrompt, "{{user_prompt}}")
	assert.Contains(t, defaultAgentTaskPrompt, "{{tickets}}")
}
