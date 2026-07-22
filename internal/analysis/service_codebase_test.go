package analysis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/openai"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeCodebaseAI is a scripted chatClient double. It returns a fixed selection
// JSON for the codebase_file_select call and a fixed assignment body for the
// person_assignment call, while capturing every prompt it was sent so the test
// can assert on prompt content.
type fakeCodebaseAI struct {
	selectResponse     string // returned for the file-select call
	assignmentResponse string // returned for the assignment call
	prompts            []string
}

func (f *fakeCodebaseAI) ChatCompletion(_ context.Context, messages []openai.Message) (*openai.Response, error) {
	prompt := messages[0].Content
	f.prompts = append(f.prompts, prompt)

	// Distinguish the file-select call from the assignment call by content,
	// since a run with no linked repo never issues a file-select call at all.
	if strings.Contains(prompt, "selecting which repository files") {
		return &openai.Response{Content: f.selectResponse}, nil
	}
	return &openai.Response{Content: f.assignmentResponse}, nil
}

func (f *fakeCodebaseAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}

func (f *fakeCodebaseAI) EmbedText(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

// assignmentPrompt returns the (non file-select) assignment prompt sent to the fake.
func (f *fakeCodebaseAI) assignmentPrompt() string {
	for _, p := range f.prompts {
		if !strings.Contains(p, "selecting which repository files") {
			return p
		}
	}
	return ""
}

// newCodebaseTestService builds a Service backed by a temp SQLite DB with one
// org, one user, and one context, ready for GeneratePersonAssignments.
func newCodebaseTestService(t *testing.T, ai *fakeCodebaseAI) (*Service, string) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	_, err = s.CreateUser(ctx, appstore.CreateUserArgs{OrgID: org.ID, DisplayName: "Ada Lovelace"})
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	jiraSvc := jiraservice.NewContextService(s, nil, jiraRepo, nil, nil, zap.NewNop())

	svc := &Service{
		store:   s,
		jiraSvc: jiraSvc,
		ai:      ai,
		logger:  zap.NewNop(),
	}
	return svc, ctxRow.ID
}

// linkRepo creates a VCS repo pointing at localPath and links it to contextID,
// then attaches the vcs store to svc so contextRepoPath can resolve it.
func linkRepo(t *testing.T, svc *Service, contextID, localPath string) {
	t.Helper()
	vs := vcsstore.NewStore(svc.store.DB())
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "widgets")
	require.NoError(t, err)
	require.NoError(t, vs.Repos.UpdateLocalPath(ctx, repo.ID, localPath))

	_, err = vs.Branches.AddBranch(ctx, contextID, repo.ID, "main", "")
	require.NoError(t, err)

	svc.SetVCSStore(vs)
}

func TestGeneratePersonAssignments_withLinkedRepo_injectsCodebaseContext(t *testing.T) {
	repoDir := t.TempDir()
	authGo := "package auth\n\nfunc Login() error { return nil }\n"
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "auth.go"), []byte(authGo), 0o644))

	ai := &fakeCodebaseAI{
		selectResponse:     `["auth.go"]`,
		assignmentResponse: `[{"name":"Ada Lovelace","body":"## Ada Lovelace\n\nassigned work"}]`,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	linkRepo(t, svc, contextID, repoDir)

	_, err := svc.GeneratePersonAssignments(context.Background(), contextID)
	require.NoError(t, err)

	require.Len(t, ai.prompts, 2, "expected one file-select call and one assignment call")
	assignmentPrompt := ai.assignmentPrompt()
	require.Contains(t, assignmentPrompt, "## Codebase context")
	require.Contains(t, assignmentPrompt, "auth.go")
	require.Contains(t, assignmentPrompt, "func Login() error")
}

func TestGeneratePersonAssignments_malformedSelectResponse_fallsBackToBaseTemplate(t *testing.T) {
	repoDir := t.TempDir()
	authGo := "package auth\n\nfunc Login() error { return nil }\n"
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "auth.go"), []byte(authGo), 0o644))

	ai := &fakeCodebaseAI{
		selectResponse:     "not json",
		assignmentResponse: `[{"name":"Ada Lovelace","body":"## Ada Lovelace\n\nassigned work"}]`,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	linkRepo(t, svc, contextID, repoDir)

	_, err := svc.GeneratePersonAssignments(context.Background(), contextID)
	require.NoError(t, err, "malformed codebase_file_select output must not fail the whole assignment flow")

	require.Len(t, ai.prompts, 2, "expected one file-select call and one assignment call")
	assignmentPrompt := ai.assignmentPrompt()
	require.NotContains(t, assignmentPrompt, "## Codebase context", "should fall back to the base template when selection is malformed")
	require.NotContains(t, assignmentPrompt, "func Login() error", "no file content should be injected when selection is malformed")
}

func TestGeneratePersonAssignments_noLinkedRepo_usesBaseTemplate(t *testing.T) {
	ai := &fakeCodebaseAI{
		assignmentResponse: `[{"name":"Ada Lovelace","body":"## Ada Lovelace\n\nassigned work"}]`,
	}
	svc, contextID := newCodebaseTestService(t, ai)
	// No linkRepo call: svc.vcsStore stays nil, so contextRepoPath returns "".

	_, err := svc.GeneratePersonAssignments(context.Background(), contextID)
	require.NoError(t, err)

	require.Len(t, ai.prompts, 1, "expected only the assignment call — no file-select call without a linked repo")
	assignmentPrompt := ai.assignmentPrompt()
	require.NotContains(t, assignmentPrompt, "## Codebase context")
}
