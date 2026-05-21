package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertMessage creates or updates a message identified by (source_id, external_id, context_id).
// Returns the internal message ID.
func (s *Store) UpsertMessage(ctx context.Context, args UpsertMessageArgs) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var contextIDPtr, orgIDPtr, parentIDPtr, authorIDPtr *string
	if args.ContextID != "" {
		contextIDPtr = &args.ContextID
	}
	if args.OrganizationID != "" {
		orgIDPtr = &args.OrganizationID
	}
	if args.ParentID != "" {
		parentIDPtr = &args.ParentID
	}
	if args.AuthorID != "" {
		authorIDPtr = &args.AuthorID
	}

	createdAt := args.CreatedAt
	if createdAt == "" {
		createdAt = now
	}
	updatedAt := args.UpdatedAt
	if updatedAt == "" {
		updatedAt = now
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM messages WHERE source_id = ? AND external_id = ? AND context_id IS ?`,
		args.SourceID, args.ExternalID, contextIDPtr,
	).Scan(&id)

	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("lookup message: %w", err)
		}
		id = uuid.New().String()
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO messages
			  (id, author_id, raw_author, context_id, organization_id, parent_id,
			   title, body, source_id, external_id, created_at, updated_at, inserted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, authorIDPtr, args.RawAuthor, contextIDPtr, orgIDPtr, parentIDPtr,
			args.Title, args.Body, args.SourceID, args.ExternalID,
			createdAt, updatedAt, now)
		if err != nil {
			return "", fmt.Errorf("insert message: %w", err)
		}
		return id, nil
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE messages
		SET author_id = ?, raw_author = ?, organization_id = ?, parent_id = ?,
		    title = ?, body = ?, updated_at = ?
		WHERE id = ?
	`, authorIDPtr, args.RawAuthor, orgIDPtr, parentIDPtr,
		args.Title, args.Body, updatedAt, id)
	if err != nil {
		return "", fmt.Errorf("update message: %w", err)
	}
	return id, nil
}

// ListMessagesSince returns messages for contextID on or after since,
// joined with users for AuthorDisplayName and sources for SourceLabel.
// Pass sourceIDs to filter by provider (empty = all). Ordered by created_at ASC.
func (s *Store) ListMessagesSince(ctx context.Context, contextID string, since time.Time, sourceIDs ...string) ([]Message, error) {
	q := `
		SELECT m.id, m.author_id, COALESCE(u.display_name, ''), m.raw_author,
		       m.context_id, m.organization_id, m.parent_id,
		       m.title, m.body, m.source_id, COALESCE(src.label, m.source_id),
		       m.external_id, m.created_at, m.updated_at, m.inserted_at
		FROM messages m
		LEFT JOIN users u     ON u.id = m.author_id
		LEFT JOIN sources src ON src.id = m.source_id
		WHERE m.context_id = ?`
	args := []any{contextID}

	if !since.IsZero() {
		q += " AND m.created_at >= ?"
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	if len(sourceIDs) > 0 {
		ph := ""
		for i, src := range sourceIDs {
			if i > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, src)
		}
		q += " AND m.source_id IN (" + ph + ")"
	}
	q += " ORDER BY m.created_at ASC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.AuthorID, &m.AuthorDisplayName, &m.RawAuthor,
			&m.ContextID, &m.OrganizationID, &m.ParentID,
			&m.Title, &m.Body, &m.SourceID, &m.SourceLabel,
			&m.ExternalID, &m.CreatedAt, &m.UpdatedAt, &m.InsertedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// BackfillMessages creates message records for provider items with message_id IS NULL.
// Idempotent — safe to call at startup.
func (s *Store) BackfillMessages(ctx context.Context) error {
	if err := s.backfillJiraIssues(ctx); err != nil {
		return fmt.Errorf("backfill jira issues: %w", err)
	}
	if err := s.backfillJiraComments(ctx); err != nil {
		return fmt.Errorf("backfill jira comments: %w", err)
	}
	if err := s.backfillSlackThreads(ctx); err != nil {
		return fmt.Errorf("backfill slack threads: %w", err)
	}
	if err := s.backfillVCSPRs(ctx); err != nil {
		return fmt.Errorf("backfill vcs prs: %w", err)
	}
	if err := s.backfillVCSPRComments(ctx); err != nil {
		return fmt.Errorf("backfill vcs pr comments: %w", err)
	}
	if err := s.backfillContextTexts(ctx); err != nil {
		return fmt.Errorf("backfill context texts: %w", err)
	}
	if err := s.backfillUploadedFiles(ctx); err != nil {
		return fmt.Errorf("backfill uploaded files: %w", err)
	}
	return nil
}

func (s *Store) backfillJiraIssues(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, jira_key, summary, COALESCE(description,''),
		       COALESCE(assignee,''), COALESCE(jira_updated,''), created_at
		FROM jira_issues WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, jiraKey, summary, description, assignee, jiraUpdated, createdAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.jiraKey, &r.summary, &r.description, &r.assignee, &r.jiraUpdated, &r.createdAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		ts := r.jiraUpdated
		if ts == "" {
			ts = r.createdAt
		}
		authorID := s.ResolveAuthor(ctx, "jira", r.assignee)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "jira",
			ExternalID: r.jiraKey,
			Title:      r.summary,
			Body:       r.description,
			RawAuthor:  r.assignee,
			AuthorID:   authorID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  ts,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE jira_issues SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillJiraComments(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jc.id, ji.context_id, jc.jira_id, jc.body, jc.author,
		       COALESCE(jc.jira_created, jc.created_at), jc.created_at,
		       (SELECT m.id FROM messages m
		        WHERE m.source_id='jira' AND m.external_id=ji.jira_key
		          AND m.context_id=ji.context_id LIMIT 1)
		FROM jira_comments jc
		JOIN jira_issues ji ON jc.issue_id = ji.id
		WHERE jc.message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct {
		id, contextID, jiraID, body, author, jiraCreated, createdAt string
		parentMsgID                                                  *string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.jiraID, &r.body, &r.author, &r.jiraCreated, &r.createdAt, &r.parentMsgID); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "jira", r.author)
		args := UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "jira",
			ExternalID: r.jiraID,
			Body:       r.body,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			CreatedAt:  r.jiraCreated,
			UpdatedAt:  r.jiraCreated,
		}
		if r.parentMsgID != nil {
			args.ParentID = *r.parentMsgID
		}
		msgID, err := s.UpsertMessage(ctx, args)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE jira_comments SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillSlackThreads(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, channel_name, COALESCE(content,''), thread_ts, created_at
		FROM slack_threads WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, channelName, content, threadTS, createdAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.channelName, &r.content, &r.threadTS, &r.createdAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "slack",
			ExternalID: r.threadTS,
			Title:      r.channelName,
			Body:       r.content,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.createdAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE slack_threads SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillVCSPRs(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vpr.id, cvb.context_id,
		       r.owner || '/' || r.name || '#' || CAST(vpr.pr_number AS TEXT),
		       vpr.title, COALESCE(vpr.author,''),
		       vpr.pr_created_at, COALESCE(vpr.pr_updated_at, vpr.pr_created_at)
		FROM vcs_pull_requests vpr
		JOIN vcs_repos r ON vpr.repo_id = r.id
		JOIN context_vcs_branches cvb ON vpr.branch_id = cvb.id
		WHERE vpr.message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, externalID, title, author, createdAt, updatedAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.externalID, &r.title, &r.author, &r.createdAt, &r.updatedAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "github", r.author)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "github",
			ExternalID: r.externalID,
			Title:      r.title,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE vcs_pull_requests SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillVCSPRComments(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vpc.id, vpc.external_id, vpc.body, COALESCE(vpc.author,''),
		       vpc.comment_created_at, COALESCE(vpc.comment_updated_at, vpc.comment_created_at),
		       vpr.message_id, cvb.context_id
		FROM vcs_pr_comments vpc
		JOIN vcs_pull_requests vpr ON vpc.pr_id = vpr.id
		JOIN context_vcs_branches cvb ON vpr.branch_id = cvb.id
		WHERE vpc.message_id IS NULL AND vpr.message_id IS NOT NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct {
		id, externalID, body, author, createdAt, updatedAt string
		parentMsgID                                        string
		contextID                                          string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.externalID, &r.body, &r.author, &r.createdAt, &r.updatedAt, &r.parentMsgID, &r.contextID); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "github", r.author)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "github",
			ExternalID: r.externalID,
			Body:       r.body,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			ParentID:   r.parentMsgID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE vcs_pr_comments SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillContextTexts(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, COALESCE(name,''), content, created_at, updated_at
		FROM context_texts WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, name, content, createdAt, updatedAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.name, &r.content, &r.createdAt, &r.updatedAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "text",
			ExternalID: r.id,
			Title:      r.name,
			Body:       r.content,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE context_texts SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillUploadedFiles(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, original_name, COALESCE(extracted_text,''), created_at, updated_at
		FROM context_uploaded_files WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, originalName, extractedText, createdAt, updatedAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.originalName, &r.extractedText, &r.createdAt, &r.updatedAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "file",
			ExternalID: r.id,
			Title:      r.originalName,
			Body:       r.extractedText,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE context_uploaded_files SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}
