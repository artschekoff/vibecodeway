package store

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExternalizedPromptsSeeded verifies that prompts which previously existed
// only as built-in fallbacks (and the new Ask AI prompt) are seeded into the DB
// so they appear in Settings → Prompts and are editable.
func TestExternalizedPromptsSeeded(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"ask_ai", "person_assignment_summary", "person_assignment_judge"} {
		p, err := s.GetAIPrompt(ctx, id)
		require.NoErrorf(t, err, "prompt %q should be seeded", id)
		require.NotEmpty(t, strings.TrimSpace(p.Body), "prompt %q body should not be empty", id)
		require.Equal(t, p.Body, p.DefaultBody, "seeded body and default should match for %q", id)
	}

	// They must also show up in the list backing the Prompts settings page.
	all, err := s.ListAIPrompts(ctx)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, p := range all {
		ids[p.ID] = true
	}
	require.True(t, ids["ask_ai"], "ask_ai should be listed")
	require.True(t, ids["person_assignment_judge"], "person_assignment_judge should be listed")
	require.True(t, ids["person_assignment_summary"], "person_assignment_summary should be listed")
}

// TestAssignmentPromptRequestsTaskList verifies that the person_assignment and
// person_assignment_single prompts instruct the model to write actionable
// steps as a GFM task list, so downstream rendering can turn them into an
// interactive checklist.
func TestAssignmentPromptRequestsTaskList(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	p, err := store.GetAIPrompt(ctx, "person_assignment")
	require.NoError(t, err)
	assert.Contains(t, p.Body, "- [ ] ")
	// The example rows themselves must be unindented (column 0) so they match
	// the frontend parser's anchored regex `^- \[( |x)\] ?(.*)$` — a leading
	// indent would parse as plain text and the model may mirror it.
	assert.Contains(t, p.Body, "\n- [ ] ", "person_assignment task-list example must be unindented, not \"\\n  - [ ]\"")
	assert.NotContains(t, p.Body, "\n  - [ ] ")

	single, err := store.GetAIPrompt(ctx, "person_assignment_single")
	require.NoError(t, err)
	assert.Contains(t, single.Body, "- [ ] ")
}

// TestAssignmentPromptTaskListExemptFromPathProhibition verifies that the
// blanket "no file paths" prohibitions surviving in person_assignment and
// person_assignment_single no longer contradict the task-list instruction:
// the old unscoped sentences must be gone, and each prompt must now carry an
// explicit exemption allowing an inline `path:line` reference inside the
// actionable-steps task list.
func TestAssignmentPromptTaskListExemptFromPathProhibition(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	p, err := store.GetAIPrompt(ctx, "person_assignment")
	require.NoError(t, err)
	assert.Contains(t, p.Body, "- [ ] ")
	assert.NotContains(t, p.Body, "Do NOT include specific file paths, directory paths, function names, class names, or component paths. Describe work at the feature and behavior level.")
	assert.Contains(t, p.Body, "except inside the Next concrete actions task list")
	assert.Equal(t, p.Body, p.DefaultBody, "default_body should match the scoped body")

	single, err := store.GetAIPrompt(ctx, "person_assignment_single")
	require.NoError(t, err)
	assert.Contains(t, single.Body, "- [ ] ")
	assert.NotContains(t, single.Body, "but must NOT contain specific file paths, function names, class names, or directory structures — the agent resolves those from the codebase itself.")
	assert.NotContains(t, single.Body, "- Do NOT name specific file paths, directory paths, function names, class names, or module paths\n")
	assert.Contains(t, single.Body, "Inside the Implementation Steps task list, you may include an inline `path:line` reference")
	assert.Contains(t, single.Body, "except as an inline `path:line` reference inside the Implementation Steps task list")
	assert.Equal(t, single.Body, single.DefaultBody, "default_body should match the scoped body")
}

// TestCodebaseReadPromptsSeeded verifies that the two prompts backing
// "Codebase Read Mode" are seeded: person_assignment_with_code (the
// person_assignment body plus a trailing codebase-context section) and
// codebase_file_select (the AI file-selection step).
func TestCodebaseReadPromptsSeeded(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	wc, err := store.GetAIPrompt(ctx, "person_assignment_with_code")
	require.NoError(t, err)
	assert.Contains(t, wc.Body, "{{codebase_context}}")
	sel, err := store.GetAIPrompt(ctx, "codebase_file_select")
	require.NoError(t, err)
	assert.Contains(t, sel.Body, "{{file_tree}}")
}
