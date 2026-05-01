package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListOrganizations returns all organizations ordered by sort_order ASC, then created_at ASC.
func (s *Store) ListOrganizations(ctx context.Context) ([]Organization, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, sort_order, created_at, updated_at
		FROM organizations
		ORDER BY sort_order ASC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	defer rows.Close()

	var orgs []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.SortOrder, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning organization: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// CreateOrganization inserts a new organization and returns it.
func (s *Store) CreateOrganization(ctx context.Context, name string) (*Organization, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()

	var maxOrder int
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order),0) FROM organizations`).Scan(&maxOrder)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO organizations (id, name, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, name, maxOrder+1, now, now)
	if err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}
	return &Organization{ID: id, Name: name, SortOrder: maxOrder + 1, CreatedAt: now, UpdatedAt: now}, nil
}

// UpdateOrganization renames an organization and returns the updated record.
func (s *Store) UpdateOrganization(ctx context.Context, id, name string) (*Organization, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `UPDATE organizations SET name=?, updated_at=? WHERE id=?`, name, now, id)
	if err != nil {
		return nil, fmt.Errorf("updating organization: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	var o Organization
	err = s.db.QueryRowContext(ctx, `
		SELECT id, name, sort_order, created_at, updated_at FROM organizations WHERE id=?
	`, id).Scan(&o.ID, &o.Name, &o.SortOrder, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetching updated organization: %w", err)
	}
	return &o, nil
}

// DeleteOrganization removes an organization. Contexts with this org_id are set to NULL via FK ON DELETE SET NULL.
func (s *Store) DeleteOrganization(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM organizations WHERE id=?`, id)
	return err
}

// SetOrganizationsOrder sets sort_order for an ordered list of organization IDs.
func (s *Store) SetOrganizationsOrder(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `UPDATE organizations SET sort_order=? WHERE id=?`)
	if err != nil {
		return fmt.Errorf("prepare set org order: %w", err)
	}
	defer stmt.Close()

	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, i, id); err != nil {
			return fmt.Errorf("setting org order for %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// SetContextOrg assigns a context to an organization. Pass empty orgID to clear the assignment.
func (s *Store) SetContextOrg(ctx context.Context, contextID, orgID string) error {
	if orgID == "" {
		_, err := s.db.ExecContext(ctx, `UPDATE contexts SET org_id=NULL WHERE id=?`, contextID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE contexts SET org_id=? WHERE id=?`, orgID, contextID)
	return err
}

// ListContextsByOrg returns all contexts (including archived) for a given organization.
func (s *Store) ListContextsByOrg(ctx context.Context, orgID string) ([]Context, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.key, c.status, c.created_at, c.updated_at,
		       COUNT(DISTINCT i.id),
		       COALESCE((
		           SELECT MAX(ts) FROM (
		               SELECT MAX(i2.created_at) AS ts FROM jira_issues i2 WHERE i2.context_id = c.id
		               UNION ALL
		               SELECT MAX(jc.created_at) FROM jira_comments jc
		                   JOIN jira_issues ji ON ji.id = jc.issue_id WHERE ji.context_id = c.id
		               UNION ALL
		               SELECT MAX(st.created_at) FROM slack_threads st WHERE st.context_id = c.id
		           )
		       ), ''),
		       (SELECT COUNT(*) FROM jira_issues WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM jira_comments jc2
		            JOIN jira_issues ji2 ON jc2.issue_id = ji2.id
		            WHERE ji2.context_id = c.id AND jc2.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM slack_threads WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM slack_dm_configs WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM vcs_pull_requests vpr
		            JOIN context_vcs_branches b ON vpr.branch_id = b.id
		            WHERE b.context_id = c.id AND vpr.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM vcs_pr_comments vpc
		            JOIN vcs_pull_requests vpr2 ON vpc.pr_id = vpr2.id
		            JOIN context_vcs_branches b2 ON vpr2.branch_id = b2.id
		            WHERE b2.context_id = c.id AND vpc.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM notifications n WHERE n.context_id = c.id AND n.read_at IS NULL),
		       EXISTS(SELECT 1 FROM jira_provider WHERE context_id = c.id),
		       (EXISTS(SELECT 1 FROM slack_channels WHERE context_id = c.id)
		        OR EXISTS(SELECT 1 FROM slack_dm_configs WHERE context_id = c.id)),
		       EXISTS(SELECT 1 FROM context_vcs_branches WHERE context_id = c.id),
		       (EXISTS(SELECT 1 FROM context_texts WHERE context_id = c.id)
		        OR EXISTS(SELECT 1 FROM context_uploaded_files WHERE context_id = c.id)),
		       c.ai_proposed_status, c.ai_proposed_comment, c.ai_proposed_next_step, c.ai_proposed_at,
		       c.user_status_override, c.user_comment_override, c.user_next_step_override, c.user_override_updated_at
		FROM contexts c
		LEFT JOIN jira_issues i ON i.context_id = c.id
		WHERE c.org_id = ?
		GROUP BY c.id
		ORDER BY c.sort_order ASC NULLS LAST, c.updated_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing contexts by org: %w", err)
	}
	defer rows.Close()
	return scanContextRows(rows)
}
