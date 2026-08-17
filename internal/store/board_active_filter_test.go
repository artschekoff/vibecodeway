package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The board reflects active work: rows linked to an archived (deactivated)
// context must not surface. Rows with no context link are unaffected.

func TestListBoardPRsByAuthor_excludesArchivedContext(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES ('org-1', 'Org', 0, datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES ('ctx-1', 'archived-ctx', 'archived', 'org-1', datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO vcs_repos (id, platform, owner, name, created_at) VALUES ('repo-1', 'github', 'acme', 'api', datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, created_at) VALUES ('branch-1', 'ctx-1', 'repo-1', 'feat/x', datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO vcs_pull_requests (id, repo_id, branch_id, pr_number, author, state, fetched_at, pr_updated_at)
		 VALUES ('pr-1', 'repo-1', 'branch-1', 42, 'carol', 'open', datetime('now'), datetime('now'))`)
	require.NoError(t, err)

	prs, err := db.ListBoardPRsByAuthor(ctx, "carol")
	require.NoError(t, err)
	assert.Empty(t, prs, "PR on an archived context must not appear on the board")
}

func TestListBoardAssignments_excludesArchivedContext(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES ('org-1', 'Org', 0, datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES ('ctx-active', 'a', 'active', 'org-1', datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES ('ctx-arch', 'b', 'archived', 'org-1', datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO context_person_assignments (id, context_id, name, body, updated_at) VALUES
		 ('pa-active', 'ctx-active', 'bob', 'x', datetime('now')),
		 ('pa-arch', 'ctx-arch', 'bob', 'y', datetime('now'))`)
	require.NoError(t, err)

	got, err := db.ListBoardAssignments(ctx, "bob")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "pa-active", got[0].ID)
}

func TestListBoardJiraAssignments_excludesArchivedContextKeepsUnlinked(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	_, err := db.db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, sort_order, created_at, updated_at) VALUES ('org-1', 'Org', 0, datetime('now'), datetime('now'))`)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx,
		`INSERT INTO contexts (id, key, status, org_id, created_at, updated_at) VALUES ('ctx-arch', 'arch', 'archived', 'org-1', datetime('now'), datetime('now'))`)
	require.NoError(t, err)

	// JH-ARCH is linked (via jira_issues) to an archived context -> excluded.
	// JH-FREE has no context link -> kept.
	require.NoError(t, db.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{
		{Key: "JH-ARCH", Summary: "s", Status: "In Progress", Priority: "High"},
		{Key: "JH-FREE", Summary: "s", Status: "In Progress", Priority: "High"},
	}))
	insertJiraIssue(t, db, "ji-arch", "ctx-arch", "JH-ARCH", "In Progress")

	got, err := db.ListBoardJiraAssignments(ctx, "priority")
	require.NoError(t, err)
	keys := make([]string, 0, len(got))
	for _, a := range got {
		keys = append(keys, a.Key)
	}
	assert.Contains(t, keys, "JH-FREE", "unlinked assignment must remain")
	assert.NotContains(t, keys, "JH-ARCH", "assignment linked to an archived context must be dropped")
}
