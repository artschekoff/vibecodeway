package slackstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertSlackThread inserts or updates a Slack thread (dedup on context_id + channel_id + thread_ts).
func (r *Repo) UpsertSlackThread(ctx context.Context, contextID string, args UpsertSlackThreadArgs) (*SlackThread, error) {
	var existing string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM slack_threads WHERE context_id = ? AND channel_id = ? AND thread_ts = ?",
		contextID, args.ChannelID, args.ThreadTS,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(ctx, `
			UPDATE slack_threads
			SET slack_channel_id = ?, workspace = ?, channel_name = ?, permalink = ?,
			    search_query = ?, content = ?
			WHERE id = ?
		`, nullableString(args.SlackChannelID), args.Workspace, args.ChannelName, args.Permalink,
			args.SearchQuery, args.Content, existing)
		if err != nil {
			return nil, fmt.Errorf("updating slack thread: %w", err)
		}
		return r.getSlackThread(ctx, existing)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying slack thread: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO slack_threads
			(id, context_id, slack_channel_id, workspace, channel_id, thread_ts,
			 channel_name, permalink, search_query, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, contextID, nullableString(args.SlackChannelID), args.Workspace,
		args.ChannelID, args.ThreadTS, args.ChannelName, args.Permalink,
		args.SearchQuery, args.Content, now)
	if err != nil {
		return nil, fmt.Errorf("inserting slack thread: %w", err)
	}
	return r.getSlackThread(ctx, id)
}

// ListSlackThreads returns all Slack threads for a context, ordered by created_at.
func (r *Repo) ListSlackThreads(ctx context.Context, contextID string) ([]SlackThread, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, context_id, COALESCE(slack_channel_id, ''), workspace, channel_id, thread_ts,
		       channel_name, permalink, search_query, content, created_at
		FROM slack_threads
		WHERE context_id = ?
		ORDER BY created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing slack threads: %w", err)
	}
	defer rows.Close()

	var threads []SlackThread
	for rows.Next() {
		var t SlackThread
		if err := rows.Scan(
			&t.ID, &t.ContextID, &t.SlackChannelID, &t.Workspace, &t.ChannelID, &t.ThreadTS,
			&t.ChannelName, &t.Permalink, &t.SearchQuery, &t.Content, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning slack thread: %w", err)
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

// DeleteSlackThread removes a single Slack thread by its internal ID.
// Returns ErrNotFound if no row with that ID exists.
func (r *Repo) DeleteSlackThread(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM slack_threads WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting slack thread: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) getSlackThread(ctx context.Context, id string) (*SlackThread, error) {
	var t SlackThread
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, COALESCE(slack_channel_id, ''), workspace, channel_id, thread_ts,
		       channel_name, permalink, search_query, content, created_at
		FROM slack_threads WHERE id = ?
	`, id).Scan(
		&t.ID, &t.ContextID, &t.SlackChannelID, &t.Workspace, &t.ChannelID, &t.ThreadTS,
		&t.ChannelName, &t.Permalink, &t.SearchQuery, &t.Content, &t.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting slack thread: %w", err)
	}
	return &t, nil
}

// nullableString converts an empty string to nil so SQLite stores NULL
// for optional FK columns instead of an empty string that would violate the FK.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
