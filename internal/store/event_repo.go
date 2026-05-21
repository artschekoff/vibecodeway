package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AddEvent appends one event to the event log.
func (s *Store) AddEvent(ctx context.Context, e Event) (*Event, error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if e.Timestamp == "" {
		e.Timestamp = now
	}
	if e.CreatedAt == "" {
		e.CreatedAt = now
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO event_log
		  (id, context_id, sync_session_id, type, timestamp, source_type, source_id, author, summary, raw_reference, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.ContextID, e.SyncSessionID, e.Type, e.Timestamp, e.SourceType,
		e.SourceID, e.Author, e.Summary, e.RawReference, e.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add event: %w", err)
	}
	return &e, nil
}

// AddEvents inserts multiple events in a single transaction.
func (s *Store) AddEvents(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin add events tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO event_log
		  (id, context_id, sync_session_id, type, timestamp, source_type, source_id, author, summary, raw_reference, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare add events: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range events {
		e := &events[i]
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		if e.Timestamp == "" {
			e.Timestamp = now
		}
		if e.CreatedAt == "" {
			e.CreatedAt = now
		}
		if _, err := stmt.ExecContext(ctx, e.ID, e.ContextID, e.SyncSessionID, e.Type, e.Timestamp,
			e.SourceType, e.SourceID, e.Author, e.Summary, e.RawReference, e.CreatedAt); err != nil {
			return fmt.Errorf("insert event %s: %w", e.ID, err)
		}
	}
	return tx.Commit()
}

// ListEvents returns all events for a context ordered by original timestamp ascending.
func (s *Store) ListEvents(ctx context.Context, contextID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, sync_session_id, type, timestamp, source_type, source_id, author, summary, raw_reference, created_at
		FROM event_log WHERE context_id = ? ORDER BY timestamp ASC, created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanEvents(rows)
}

// ListEventsSince returns events from sessions that started after lastSessionID (exclusive),
// plus all user-sourced events (sync_session_id IS NULL) that were added after lastSessionID was created.
// If lastSessionID is empty, all events are returned.
func (s *Store) ListEventsSince(ctx context.Context, contextID, lastSessionID string) ([]Event, error) {
	var rows *sql.Rows
	var err error
	if lastSessionID == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, context_id, sync_session_id, type, timestamp, source_type, source_id, author, summary, raw_reference, created_at
			FROM event_log WHERE context_id = ? ORDER BY timestamp ASC, created_at ASC
		`, contextID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT el.id, el.context_id, el.sync_session_id, el.type, el.timestamp,
			       el.source_type, el.source_id, el.author, el.summary, el.raw_reference, el.created_at
			FROM event_log el
			LEFT JOIN sync_sessions ss ON ss.id = el.sync_session_id
			WHERE el.context_id = ?
			  AND (
			    -- user events created after the last analysis session started
			    (el.sync_session_id IS NULL AND el.created_at > (SELECT started_at FROM sync_sessions WHERE id = ?))
			    OR
			    -- session events from sessions newer than last analyzed
			    (ss.started_at > (SELECT started_at FROM sync_sessions WHERE id = ?))
			  )
			ORDER BY el.timestamp ASC, el.created_at ASC
		`, contextID, lastSessionID, lastSessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("list events since: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanEvents(rows)
}

// DeleteEvent removes a single event from the log.
func (s *Store) DeleteEvent(ctx context.Context, eventID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM event_log WHERE id = ? AND source_type = 'user'`, eventID)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}

// UpdateEvent updates the type and summary of a user-authored event.
func (s *Store) UpdateEvent(ctx context.Context, eventID, eventType, summary string) (*Event, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE event_log SET type = ?, summary = ? WHERE id = ? AND source_type = 'user'`,
		eventType, summary, eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}
	var e Event
	row := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, sync_session_id, type, timestamp, source_type, source_id, author, summary, raw_reference, created_at
		FROM event_log WHERE id = ?
	`, eventID)
	if err := row.Scan(&e.ID, &e.ContextID, &e.SyncSessionID, &e.Type, &e.Timestamp,
		&e.SourceType, &e.SourceID, &e.Author, &e.Summary, &e.RawReference, &e.CreatedAt); err != nil {
		return nil, fmt.Errorf("fetch updated event: %w", err)
	}
	return &e, nil
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var result []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID, &e.ContextID, &e.SyncSessionID, &e.Type, &e.Timestamp,
			&e.SourceType, &e.SourceID, &e.Author, &e.Summary, &e.RawReference, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
