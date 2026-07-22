package analysis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/stretchr/testify/require"
)

// seedPersonAssignmentForPlan inserts a single person assignment for contextID
// via ReplacePersonAssignments (the same store method GeneratePersonAssignments
// uses) and returns the resulting assignment's ID.
func seedPersonAssignmentForPlan(t *testing.T, svc *Service, contextID string) string {
	t.Helper()
	result, err := svc.store.ReplacePersonAssignments(context.Background(), contextID, []appstore.PersonAssignment{
		{ContextID: contextID, Name: "Ada Lovelace", Body: "## Ada Lovelace\n\nassigned work"},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	return result[0].ID
}

func TestGeneratePlan_persistsAIOutput(t *testing.T) {
	canned := "## Plan\n\n- [ ] Step one\n- [ ] Step two"
	ai := &fakeCodebaseAI{
		assignmentResponse: canned,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	personAssignmentID := seedPersonAssignmentForPlan(t, svc, contextID)

	plan, err := svc.GeneratePlan(context.Background(), contextID, personAssignmentID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, canned, plan.Body)
	require.Equal(t, contextID, plan.ContextID)
	require.Equal(t, personAssignmentID, plan.PersonAssignmentID)

	fetched, err := svc.store.GetContextPlan(context.Background(), plan.ID)
	require.NoError(t, err)
	require.Equal(t, canned, fetched.Body)
}

func TestGeneratePlan_withLinkedRepo_injectsCodebaseContext(t *testing.T) {
	repoDir := t.TempDir()
	authGo := "package auth\n\nfunc Login() error { return nil }\n"
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "auth.go"), []byte(authGo), 0o644))

	canned := "## Plan\n\n- [ ] Step one"
	ai := &fakeCodebaseAI{
		selectResponse:     `["auth.go"]`,
		assignmentResponse: canned,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	linkRepo(t, svc, contextID, repoDir)
	personAssignmentID := seedPersonAssignmentForPlan(t, svc, contextID)

	plan, err := svc.GeneratePlan(context.Background(), contextID, personAssignmentID)
	require.NoError(t, err)
	require.Equal(t, canned, plan.Body)

	planPrompt := ai.assignmentPrompt()
	require.Contains(t, planPrompt, "## Codebase context")
	require.Contains(t, planPrompt, "auth.go")
	require.Contains(t, planPrompt, "func Login() error")
}

func TestGeneratePlan_noLinkedRepo_omitsCodebaseContext(t *testing.T) {
	canned := "## Plan\n\n- [ ] Step one"
	ai := &fakeCodebaseAI{
		assignmentResponse: canned,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	// No linkRepo call: svc.vcsStore stays nil, so contextRepoPath returns "".
	personAssignmentID := seedPersonAssignmentForPlan(t, svc, contextID)

	plan, err := svc.GeneratePlan(context.Background(), contextID, personAssignmentID)
	require.NoError(t, err)
	require.Equal(t, canned, plan.Body)

	require.Len(t, ai.prompts, 1, "no linked repo means no file-select call")
	planPrompt := ai.prompts[0]
	require.NotContains(t, planPrompt, "## Codebase context")
	require.False(t, strings.Contains(planPrompt, "func Login() error"))
}

func TestGeneratePlan_missingPersonAssignment_returnsErrorNotPanic(t *testing.T) {
	ai := &fakeCodebaseAI{
		assignmentResponse: "## Plan\n\n- [ ] Step one",
	}
	svc, contextID := newCodebaseTestService(t, ai)

	plan, err := svc.GeneratePlan(context.Background(), contextID, "does-not-exist")
	require.Error(t, err)
	require.Nil(t, plan)
}
