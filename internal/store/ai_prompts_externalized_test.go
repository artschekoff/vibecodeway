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

	single, err := store.GetAIPrompt(ctx, "person_assignment_single")
	require.NoError(t, err)
	assert.Contains(t, single.Body, "- [ ] ")
}
