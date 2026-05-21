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
			    search_query = ?, content = ?, parent_thread_ts = ?
			WHERE id = ?
		`, nullableString(args.SlackChannelID), args.Workspace, args.ChannelName, args.Permalink,
			args.SearchQuery, args.Content, args.ParentThreadTS, existing)
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
			 parent_thread_ts, channel_name, permalink, search_query, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, contextID, nullableString(args.SlackChannelID), args.Workspace,
		args.ChannelID, args.ThreadTS, args.ParentThreadTS, args.ChannelName, args.Permalink,
		args.SearchQuery, args.Content, now)
	if err != nil {
		return nil, fmt.Errorf("inserting slack thread: %w", err)
	}
	return r.getSlackThread(ctx, id)
}

// ListSlackThreads returns Slack threads for a context, ordered by created_at.
// When activeOnly is true only rows with is_active = 1 are returned.
func (r *Repo) ListSlackThreads(ctx context.Context, contextID string, activeOnly bool) ([]SlackThread, error) {
	q := `
		SELECT id, context_id, COALESCE(slack_channel_id, ''), workspace, channel_id, thread_ts,
		       parent_thread_ts, channel_name, permalink, search_query, content, created_at, is_active, viewed_at, ai_processed_at
		FROM slack_threads
		WHERE context_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY created_at ASC"
	rows, err := r.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing slack threads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var threads []SlackThread
	for rows.Next() {
		var t SlackThread
		if err := rows.Scan(
			&t.ID, &t.ContextID, &t.SlackChannelID, &t.Workspace, &t.ChannelID, &t.ThreadTS,
			&t.ParentThreadTS, &t.ChannelName, &t.Permalink, &t.SearchQuery, &t.Content, &t.CreatedAt, &t.IsActive, &t.ViewedAt, &t.AiProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning slack thread: %w", err)
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

// ToggleSlackThreadActive flips the is_active flag for the given thread and returns the updated record.
// Returns ErrNotFound if no thread with that ID exists.
func (r *Repo) ToggleSlackThreadActive(ctx context.Context, id string) (*SlackThread, error) {
	res, err := r.db.ExecContext(ctx, "UPDATE slack_threads SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling slack thread active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.getSlackThread(ctx, id)
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
		       parent_thread_ts, channel_name, permalink, search_query, content, created_at, is_active, viewed_at, ai_processed_at
		FROM slack_threads WHERE id = ?
	`, id).Scan(
		&t.ID, &t.ContextID, &t.SlackChannelID, &t.Workspace, &t.ChannelID, &t.ThreadTS,
		&t.ParentThreadTS, &t.ChannelName, &t.Permalink, &t.SearchQuery, &t.Content, &t.CreatedAt, &t.IsActive, &t.ViewedAt, &t.AiProcessedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting slack thread: %w", err)
	}
	return &t, nil
}

// MarkSlackThreadViewed sets viewed_at = datetime('now') for the given thread if not already viewed.
func (r *Repo) MarkSlackThreadViewed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE slack_threads SET viewed_at = datetime('now') WHERE id = ? AND viewed_at IS NULL", id)
	return err
}

// MarkSlackChannelAllViewed marks all threads under the given slack_channel_id as viewed.
func (r *Repo) MarkSlackChannelAllViewed(ctx context.Context, channelID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE slack_threads SET viewed_at = datetime('now') WHERE slack_channel_id = ? AND viewed_at IS NULL", channelID)
	return err
}

// nullableString converts an empty string to nil so SQLite stores NULL
// for optional FK columns instead of an empty string that would violate the FK.
// DeleteChannelBlobs removes whole-channel snapshot records (thread_ts="") for the given
// channel. Called during Refresh after per-message records have been successfully upserted
// so stale blobs from the old single-record format do not accumulate.
func (r *Repo) DeleteChannelBlobs(ctx context.Context, contextID, channelID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM slack_threads WHERE context_id = ? AND channel_id = ? AND thread_ts = ''",
		contextID, channelID,
	)
	if err != nil {
		return fmt.Errorf("deleting channel blobs: %w", err)
	}
	return nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// SetThreadMessageID sets the message_id FK on a slack_threads row.
func (r *Repo) SetThreadMessageID(ctx context.Context, threadID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE slack_threads SET message_id = ? WHERE id = ?", messageID, threadID)
	return err
}
