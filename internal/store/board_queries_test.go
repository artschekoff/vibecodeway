package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBoardBlockers_empty(t *testing.T) {
	db := openTestStore(t)
	blockers, err := db.ListBoardBlockers(context.Background())
	require.NoError(t, err)
	require.Empty(t, blockers)
}

func TestListBoardAssignments_empty(t *testing.T) {
	db := openTestStore(t)
	assignments, err := db.ListBoardAssignments(context.Background(), "alice")
	require.NoError(t, err)
	require.Empty(t, assignments)
}

func TestListBoardPRsByAuthor_empty(t *testing.T) {
	db := openTestStore(t)
	prs, err := db.ListBoardPRsByAuthor(context.Background(), "alice")
	require.NoError(t, err)
	require.Empty(t, prs)
}

// --- Seeded data tests ---

func TestListBoardBlockers_withData(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	// Seed org + context
	orgID := "org-blocker-1"
	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES (?, 'Test Org', 0, datetime('now'), datetime('now'))`,
		orgID,
	)
	require.NoError(t, err)

	ctxID := "ctx-blocker-1"
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES (?, 'blocker-ctx', 'active', ?, datetime('now'), datetime('now'))`,
		ctxID, orgID,
	)
	require.NoError(t, err)

	// Seed a blocker_identified event
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO event_log (id, context_id, type, timestamp, source_type, author, summary, created_at)
		 VALUES ('evt-1', ?, 'blocker_identified', datetime('now'), 'jira', 'alice', 'Pipeline is broken', datetime('now'))`,
		ctxID,
	)
	require.NoError(t, err)

	blockers, err := db.ListBoardBlockers(ctx)
	require.NoError(t, err)
	require.Len(t, blockers, 1)
	assert.Equal(t, "evt-1", blockers[0].EventID)
	assert.Equal(t, ctxID, blockers[0].ContextID)
	assert.Equal(t, "blocker-ctx", blockers[0].ContextKey)
	assert.Equal(t, "alice", blockers[0].Author)
	assert.Equal(t, "Pipeline is broken", blockers[0].Summary)
}

func TestListBoardAssignments_withData(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	// Seed org + context
	orgID := "org-assign-1"
	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES (?, 'Test Org', 0, datetime('now'), datetime('now'))`,
		orgID,
	)
	require.NoError(t, err)

	ctxID := "ctx-assign-1"
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES (?, 'assign-ctx', 'active', ?, datetime('now'), datetime('now'))`,
		ctxID, orgID,
	)
	require.NoError(t, err)

	// Seed a person assignment for "bob"
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO context_person_assignments (id, context_id, name, body, updated_at) VALUES ('pa-1', ?, 'bob', '- [ ] finish PR', datetime('now'))`,
		ctxID,
	)
	require.NoError(t, err)

	// Filter by exact name should return the row
	assignments, err := db.ListBoardAssignments(ctx, "bob")
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, "pa-1", assignments[0].ID)
	assert.Equal(t, "bob", assignments[0].Name)
	assert.Equal(t, "assign-ctx", assignments[0].ContextKey)

	// Empty name should also return all assignments
	all, err := db.ListBoardAssignments(ctx, "")
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "pa-1", all[0].ID)

	// Non-matching name should return nothing
	none, err := db.ListBoardAssignments(ctx, "alice")
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestListBoardPRsByAuthor_withData(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	// Seed org + context
	orgID := "org-pr-1"
	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES (?, 'Test Org', 0, datetime('now'), datetime('now'))`,
		orgID,
	)
	require.NoError(t, err)

	ctxID := "ctx-pr-1"
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES (?, 'pr-ctx', 'active', ?, datetime('now'), datetime('now'))`,
		ctxID, orgID,
	)
	require.NoError(t, err)

	// Seed a vcs_repo
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO vcs_repos (id, platform, owner, name, created_at) VALUES ('repo-1', 'github', 'acme', 'api', datetime('now'))`,
	)
	require.NoError(t, err)

	// Seed a context_vcs_branch
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, created_at) VALUES ('branch-1', ?, 'repo-1', 'feat/thing', datetime('now'))`,
		ctxID,
	)
	require.NoError(t, err)

	// Seed a pull request authored by "carol"
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO vcs_pull_requests (id, repo_id, branch_id, pr_number, title, author, state, fetched_at, pr_updated_at)
		 VALUES ('pr-1', 'repo-1', 'branch-1', 42, 'Add feature', 'carol', 'open', datetime('now'), datetime('now'))`,
	)
	require.NoError(t, err)

	// Two comments on the PR: one unresolved, one resolved. Only the unresolved
	// one counts toward UnresolvedCount.
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO vcs_pr_comments (id, pr_id, external_id, is_resolved) VALUES
		 ('cm-1', 'pr-1', 'e1', 0), ('cm-2', 'pr-1', 'e2', 1)`,
	)
	require.NoError(t, err)

	prs, err := db.ListBoardPRsByAuthor(ctx, "carol")
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "pr-1", prs[0].ID)
	assert.Equal(t, ctxID, prs[0].ContextID)
	assert.Equal(t, "pr-ctx", prs[0].ContextKey)
	assert.Equal(t, "api", prs[0].RepoName)
	assert.Equal(t, 42, prs[0].PRNumber)
	assert.Equal(t, "Add feature", prs[0].Title)
	assert.Equal(t, "open", prs[0].State)
	assert.False(t, prs[0].IsDraft)
	assert.Equal(t, 1, prs[0].UnresolvedCount)

	// Non-matching author returns nothing
	none, err := db.ListBoardPRsByAuthor(ctx, "alice")
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestListBoardRecentEventsFiltersAndOrders(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	// Seed org + context
	org, err := db.CreateOrganization(ctx, "acme")
	require.NoError(t, err)
	c, err := db.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	// activity event (should appear) + AI semantic event (should NOT appear)
	_, err = db.AddEvent(ctx, Event{ContextID: c.ID, Type: EventTypeCommentAdded, Summary: "New comment"})
	require.NoError(t, err)
	_, err = db.AddEvent(ctx, Event{ContextID: c.ID, Type: EventTypeRequirementStated, Summary: "req"})
	require.NoError(t, err)

	evs, err := db.ListBoardRecentEvents(ctx)
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, EventTypeCommentAdded, evs[0].Type)
	assert.Equal(t, "New comment", evs[0].Summary)
}
