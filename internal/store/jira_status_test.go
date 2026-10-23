package store

import (
	"context"
	"testing"

	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTerminalTicketStatus(t *testing.T) {
	assert.True(t, IsTerminalTicketStatus("Done"))
	assert.True(t, IsTerminalTicketStatus(" closed "))
	assert.True(t, IsTerminalTicketStatus("WON'T DO"))
	assert.False(t, IsTerminalTicketStatus("In Progress"))
	assert.False(t, IsTerminalTicketStatus("Ready for QA")) // downstream, not terminal
	assert.False(t, IsTerminalTicketStatus(""))
}

func TestContextTicketState(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	// Context A: mixed -> not all done, 1 open ticket.
	a, err := s.CreateContext(ctx, "CTX-A", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "a1", a.ID, "JH-1", "Done")
	insertJiraIssue(t, s, "a2", a.ID, "JH-2", "In Progress")

	// Context B: all terminal -> all done, 0 open.
	b, err := s.CreateContext(ctx, "CTX-B", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "b1", b.ID, "JH-3", "Done")
	insertJiraIssue(t, s, "b2", b.ID, "JH-4", "Closed")

	// Context C: zero tickets -> never "all done" (vacuous guard).
	c, err := s.CreateContext(ctx, "CTX-C", org.ID)
	require.NoError(t, err)

	openA, err := s.CountOpenTickets(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, openA)
	doneA, err := s.ContextAllTicketsDone(ctx, a.ID)
	require.NoError(t, err)
	assert.False(t, doneA)

	openB, err := s.CountOpenTickets(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, openB)
	doneB, err := s.ContextAllTicketsDone(ctx, b.ID)
	require.NoError(t, err)
	assert.True(t, doneB)

	openC, err := s.CountOpenTickets(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, openC)
	doneC, err := s.ContextAllTicketsDone(ctx, c.ID)
	require.NoError(t, err)
	assert.False(t, doneC)
}

func TestListKnownJiraStatuses(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	// Statuses spread over two contexts: the list is workflow-wide, not per-context.
	a, err := s.CreateContext(ctx, "CTX-A", org.ID)
	require.NoError(t, err)
	b, err := s.CreateContext(ctx, "CTX-B", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "a1", a.ID, "JH-1", "To Do")
	insertJiraIssue(t, s, "a2", a.ID, "JH-2", "In Code Review")
	insertJiraIssue(t, s, "a3", a.ID, "JH-3", "To Do") // duplicate collapses
	insertJiraIssue(t, s, "b1", b.ID, "JH-4", "Ready for QA")
	insertJiraIssue(t, s, "b2", b.ID, "JH-5", "") // blank is not a status

	got, err := s.ListKnownJiraStatuses(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"In Code Review", "Ready for QA", "To Do"}, got)
}

func TestListKnownJiraStatuses_emptyWhenNoIssues(t *testing.T) {
	s := newTodoTestStore(t)
	got, err := s.ListKnownJiraStatuses(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestContextWorkDone(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	newCtx := func(key string) string {
		c, err := s.CreateContext(ctx, key, org.ID)
		require.NoError(t, err)
		return c.ID
	}
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO vcs_repos (id, platform, owner, name, local_path, label, description, created_at)
		 VALUES ('r1', 'github', 'acme', 'api', '', '', '', datetime('now'))`)
	require.NoError(t, err)
	prNum := 0
	insertPR := func(id, ctxID, state string) {
		t.Helper()
		prNum++
		_, err := s.DB().ExecContext(ctx,
			`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, label, created_at, autodetect_pr)
			 VALUES (?, ?, 'r1', ?, '', datetime('now'), 1)`, "b-"+id, ctxID, "br-"+id)
		require.NoError(t, err)
		_, err = s.DB().ExecContext(ctx,
			`INSERT INTO vcs_pull_requests (id, repo_id, branch_id, pr_number, title, author, state, is_draft,
			   review_status, url, head_branch, mergeable, merge_state_status, ci_status, pr_created_at, pr_updated_at, fetched_at)
			 VALUES (?, 'r1', ?, ?, 't', 'a', ?, 0, '', '', '', '', '', '', '', '', datetime('now'))`,
			id, "b-"+id, prNum, state)
		require.NoError(t, err)
	}
	done := func(ctxID string) bool {
		t.Helper()
		d, err := s.ContextWorkDone(ctx, ctxID)
		require.NoError(t, err)
		return d
	}

	// Nothing tracked: never done (vacuous guard).
	assert.False(t, done(newCtx("empty")))

	// Tickets only: done iff all terminal (existing behavior preserved).
	tOpen := newCtx("t-open")
	insertJiraIssue(t, s, "t1", tOpen, "JH-1", "In Progress")
	assert.False(t, done(tOpen))
	tDone := newCtx("t-done")
	insertJiraIssue(t, s, "t2", tDone, "JH-2", "Done")
	assert.True(t, done(tDone))

	// PRs only (review contexts): done iff every PR is closed/merged.
	pOpen := newCtx("p-open")
	insertPR("p1", pOpen, "open")
	assert.False(t, done(pOpen))
	pDone := newCtx("p-done")
	insertPR("p2", pDone, "merged")
	assert.True(t, done(pDone))

	// Mixed: all tickets terminal but a PR still open keeps it active.
	mixed := newCtx("mixed")
	insertJiraIssue(t, s, "m1", mixed, "JH-3", "Done")
	insertPR("m2", mixed, "open")
	assert.False(t, done(mixed))

	// Mixed, everything finished.
	finished := newCtx("finished")
	insertJiraIssue(t, s, "f1", finished, "JH-4", "Closed")
	insertPR("f2", finished, "closed")
	assert.True(t, done(finished))
}

func insertJiraIssue(t *testing.T, s *Store, id, ctxID, key, status string) {
	t.Helper()
	_, err := s.DB().ExecContext(context.Background(),
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES (?, ?, ?, 's', ?, '', datetime('now'))`, id, ctxID, key, status)
	require.NoError(t, err)
}

func TestListContextIDsWithOpenTickets(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "o")
	require.NoError(t, err)
	open, err := s.CreateContext(ctx, "open-work", org.ID)
	require.NoError(t, err)
	closed, err := s.CreateContext(ctx, "done-work", org.ID)
	require.NoError(t, err)

	jiraRepo := jirastore.NewRepo(s.DB())
	_, err = jiraRepo.UpsertIssue(ctx, open.ID, "JH-1", "wip", "In Progress", "", "task", "", "", "", "")
	require.NoError(t, err)
	_, err = jiraRepo.UpsertIssue(ctx, closed.ID, "JH-2", "shipped", "Done", "", "task", "", "", "", "")
	require.NoError(t, err)

	ids, err := s.ListContextIDsWithOpenTickets(ctx)
	require.NoError(t, err)
	assert.Contains(t, ids, open.ID)
	assert.NotContains(t, ids, closed.ID)
}
