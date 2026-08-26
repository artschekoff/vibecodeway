package slackstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertSlackChannel creates or updates a Slack channel configuration for a context.
// Uniqueness is keyed on (context_id, workspace, channel_name).
func (r *Repo) UpsertSlackChannel(ctx context.Context, contextID, workspace, channelID, channelName, startFrom string) (*SlackChannel, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT id FROM slack_channels WHERE context_id = ? AND workspace = ? AND channel_name = ?",
		contextID, workspace, channelName,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(
			ctx,
			"UPDATE slack_channels SET channel_id = ?, start_from = ? WHERE id = ?",
			channelID, startFrom, existing,
		)
		if err != nil {
			return nil, fmt.Errorf("updating slack channel: %w", err)
		}
		return r.getSlackChannel(ctx, existing)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying slack channel: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO slack_channels (id, context_id, workspace, channel_id, channel_name, start_from, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, contextID, workspace, channelID, channelName, startFrom, now)
	if err != nil {
		return nil, fmt.Errorf("inserting slack channel: %w", err)
	}
	return r.getSlackChannel(ctx, id)
}

// ListSlackChannels returns channels for the given context, ordered by channel_name.
// When activeOnly is true only rows with is_active = 1 are returned.
func (r *Repo) ListSlackChannels(ctx context.Context, contextID string, activeOnly bool) ([]SlackChannel, error) {
	q := `
		SELECT id, context_id, workspace, channel_id, channel_name, start_from, created_at, is_active
		FROM slack_channels
		WHERE context_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY channel_name"
	rows, err := r.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing slack channels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var channels []SlackChannel
	for rows.Next() {
		var ch SlackChannel
		if err := rows.Scan(&ch.ID, &ch.ContextID, &ch.Workspace, &ch.ChannelID, &ch.ChannelName, &ch.StartFrom, &ch.CreatedAt, &ch.IsActive); err != nil {
			return nil, fmt.Errorf("scanning slack channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// ToggleSlackChannelActive flips the is_active flag for the given channel and returns the updated record.
// Returns ErrNotFound if no channel with that ID exists.
func (r *Repo) ToggleSlackChannelActive(ctx context.Context, id string) (*SlackChannel, error) {
	res, err := r.db.ExecContext(ctx, "UPDATE slack_channels SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling slack channel active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.getSlackChannel(ctx, id)
}

// RemoveSlackChannel deletes a channel record by its primary key ID and cascade-deletes
// all slack_threads that belong to the same context + channel (matched by channel_id).
// Returns ErrNotFound if no row with that ID exists.
func (r *Repo) RemoveSlackChannel(ctx context.Context, id string) error {
	// Look up context_id + channel_id before deleting so we can cascade threads.
	var contextID, channelID string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT context_id, channel_id FROM slack_channels WHERE id = ?", id,
	).Scan(&contextID, &channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("querying slack channel for removal: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		"DELETE FROM slack_threads WHERE context_id = ? AND channel_id = ?",
		contextID, channelID,
	); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("deleting slack threads for channel: %w", err)
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM slack_channels WHERE id = ?", id)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("deleting slack channel: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		tx.Rollback() //nolint:errcheck
		return ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (r *Repo) getSlackChannel(ctx context.Context, id string) (*SlackChannel, error) {
	var ch SlackChannel
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, workspace, channel_id, channel_name, start_from, created_at, is_active
		FROM slack_channels WHERE id = ?
	`, id).Scan(&ch.ID, &ch.ContextID, &ch.Workspace, &ch.ChannelID, &ch.ChannelName, &ch.StartFrom, &ch.CreatedAt, &ch.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting slack channel by id: %w", err)
	}
	return &ch, nil
}
