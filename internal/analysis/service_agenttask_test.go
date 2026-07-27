package analysis

import (
	"context"
	"path/filepath"
	"testing"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newAgentTaskTestService builds a Service backed by a temp SQLite DB, seeded
// with one org, one context, one Jira issue (JH-1: "slow sql"), and one VCS
// branch linked to a repo whose LocalPath is "/tmp/repoA".
func newAgentTaskTestService(t *testing.T) (*Service, string) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	jiraSvc := jiraservice.NewContextService(s, nil, jiraRepo, nil, nil, zap.NewNop())

	_, err = jiraRepo.UpsertIssue(ctx, ctxRow.ID,
		"JH-1",
		"slow sql",
		"In Progress",
		"alice",
		"Bug",
		"",
		"",
		"",
		"",
	)
	require.NoError(t, err)

	svc := &Service{
		store:   s,
		jiraSvc: jiraSvc,
		logger:  zap.NewNop(),
	}

	vs := vcsstore.NewStore(s.DB())
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "repoA")
	require.NoError(t, err)
	require.NoError(t, vs.Repos.UpdateLocalPath(ctx, repo.ID, "/tmp/repoA"))

	_, err = vs.Branches.AddBranch(ctx, ctxRow.ID, repo.ID, "main", "")
	require.NoError(t, err)

	svc.SetVCSStore(vs)

	return svc, ctxRow.ID
}

func TestBuildAgentTask(t *testing.T) {
	svc, contextID := newAgentTaskTestService(t)
	out, err := svc.BuildAgentTask(context.Background(), contextID, "реализовать")
	require.NoError(t, err)
	assert.Contains(t, out, "реализовать") // the user task
	assert.Contains(t, out, "JH-1")        // ticket key
	assert.Contains(t, out, "slow sql")    // ticket summary
	assert.Contains(t, out, "/tmp/repoA")  // attached repo path
}

// TestBuildAgentTask_multiRepo proves the attached-repos loop appends every
// qualifying repo rather than stopping at (or returning) the first match —
// unlike contextRepoPath, which intentionally stops at the first repo.
func TestBuildAgentTask_multiRepo(t *testing.T) {
	svc, contextID := newAgentTaskTestService(t)

	vs := svc.vcsStore
	repoB, err := vs.Repos.EnsureRepo(context.Background(), "github", "acme", "repoB")
	require.NoError(t, err)
	require.NoError(t, vs.Repos.UpdateLocalPath(context.Background(), repoB.ID, "/tmp/repoB"))
	_, err = vs.Branches.AddBranch(context.Background(), contextID, repoB.ID, "develop", "")
	require.NoError(t, err)

	out, err := svc.BuildAgentTask(context.Background(), contextID, "реализовать")
	require.NoError(t, err)
	assert.Contains(t, out, "/tmp/repoA") // first repo's path
	assert.Contains(t, out, "/tmp/repoB") // second repo's path
}

// The real app registers paths in vcs_repo_paths, not the legacy local_path
// column. BuildAgentTask must surface those paths in the instruction.
func TestBuildAgentTask_usesRepoPaths(t *testing.T) {
	svc, contextID := newAgentTaskTestService(t)

	vs := svc.vcsStore
	repoC, err := vs.Repos.EnsureRepo(context.Background(), "github", "acme", "repoC")
	require.NoError(t, err)
	// Path only in vcs_repo_paths; local_path left empty (as in prod).
	_, err = vs.RepoPaths.Add(context.Background(), repoC.ID, "/tmp/repoC")
	require.NoError(t, err)
	_, err = vs.Branches.AddBranch(context.Background(), contextID, repoC.ID, "main", "")
	require.NoError(t, err)

	out, err := svc.BuildAgentTask(context.Background(), contextID, "реализовать")
	require.NoError(t, err)
	assert.Contains(t, out, "/tmp/repoC")
}

func TestBuildAgentTask_prefersWorktreePath(t *testing.T) {
	svc, contextID := newAgentTaskTestService(t)

	// Find repoA's id via its branch, then record a workspace (worktree) for it.
	branches, err := svc.vcsStore.Branches.ListBranches(context.Background(), contextID)
	require.NoError(t, err)
	require.NotEmpty(t, branches)
	// vcs_workspaces.repo_path_id is a NOT NULL FK into vcs_repo_paths, so a
	// path row must exist before a workspace can reference it.
	repoPath, err := svc.vcsStore.RepoPaths.Add(context.Background(), branches[0].RepoID, "/tmp/repoA")
	require.NoError(t, err)
	err = svc.vcsStore.Workspaces.Create(context.Background(), &vcsstore.VCSWorkspace{
		RepoPathID:   repoPath.ID,
		RepoID:       branches[0].RepoID,
		Branch:       "main",
		WorktreePath: "/tmp/repoA/.worktrees/main",
		Status:       "active",
	})
	require.NoError(t, err)

	out, err := svc.BuildAgentTask(context.Background(), contextID, "реализовать")
	require.NoError(t, err)
	assert.Contains(t, out, "/tmp/repoA/.worktrees/main") // worktree path, not the bare main path
}

func TestBuildReviewTask(t *testing.T) {
	svc, contextID := newAgentTaskTestService(t)
	out, err := svc.BuildReviewTask(context.Background(), contextID, 349, "Scribe note detail")
	require.NoError(t, err)
	assert.Contains(t, out, "349")                // PR number
	assert.Contains(t, out, "Scribe note detail") // PR title
	assert.Contains(t, out, "/tmp/repoA")         // attached repo/worktree path
	assert.Contains(t, out, "reviewer")           // review-oriented wording from the template
}

func TestAgentTaskPrompt_migrationUpgradesSeededBody(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	p, err := s.GetAIPrompt(context.Background(), "agent_task")
	require.NoError(t, err)
	assert.Contains(t, p.Body, "/writing-plans")
	assert.Contains(t, p.Body, "/arts-load-context {{context_key}}")
}
