package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateSyncSession inserts a new sync session and returns it.
func (s *Store) CreateSyncSession(ctx context.Context, contextID string, sources []string) (*SyncSession, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	sourcesJSON := `["` + joinStrings(sources, `","`) + `"]`
	if len(sources) == 0 {
		sourcesJSON = "[]"
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO sync_sessions (id, context_id, started_at, sources) VALUES (?, ?, ?, ?)`,
		id, contextID, now, sourcesJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("create sync session: %w", err)
	}
	return &SyncSession{ID: id, ContextID: contextID, StartedAt: now, Sources: sourcesJSON}, nil
}

// CompleteSyncSession marks a session as finished and records item count.
func (s *Store) CompleteSyncSession(ctx context.Context, id string, itemsFetched int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE sync_sessions SET finished_at = ?, items_fetched = ? WHERE id = ?`,
		now, itemsFetched, id,
	)
	if err != nil {
		return fmt.Errorf("complete sync session: %w", err)
	}
	return nil
}

// GetSyncSession returns a sync session by ID.
func (s *Store) GetSyncSession(ctx context.Context, id string) (*SyncSession, error) {
	var ss SyncSession
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, context_id, started_at, finished_at, sources, items_fetched FROM sync_sessions WHERE id = ?`, id,
	).Scan(&ss.ID, &ss.ContextID, &ss.StartedAt, &ss.FinishedAt, &ss.Sources, &ss.ItemsFetched)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sync session: %w", err)
	}
	return &ss, nil
}

// ListSyncSessions returns all finished sessions for a context, newest first.
func (s *Store) ListSyncSessions(ctx context.Context, contextID string) ([]SyncSession, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, context_id, started_at, finished_at, sources, items_fetched
		 FROM sync_sessions WHERE context_id = ? AND finished_at IS NOT NULL
		 ORDER BY started_at DESC`,
		contextID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sync sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []SyncSession
	for rows.Next() {
		var ss SyncSession
		if err := rows.Scan(&ss.ID, &ss.ContextID, &ss.StartedAt, &ss.FinishedAt, &ss.Sources, &ss.ItemsFetched); err != nil {
			return nil, fmt.Errorf("scan sync session: %w", err)
		}
		result = append(result, ss)
	}
	return result, rows.Err()
}

// TagSessionItems bulk-sets sync_session_id on newly inserted items (those that have no
// session yet and were created at or after startedAt). Called after each provider refresh.
func (s *Store) TagSessionItems(ctx context.Context, contextID, sessionID, startedAt string) error {
	queries := []string{
		`UPDATE jira_issues SET sync_session_id = ? WHERE context_id = ? AND sync_session_id IS NULL AND created_at >= ?`,
		`UPDATE slack_threads SET sync_session_id = ? WHERE context_id = ? AND sync_session_id IS NULL AND created_at >= ?`,
		`UPDATE vcs_pull_requests SET sync_session_id = ?
		 WHERE branch_id IN (SELECT id FROM context_vcs_branches WHERE context_id = ?)
		   AND sync_session_id IS NULL AND created_at >= ?`,
	}
	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q, sessionID, contextID, startedAt); err != nil {
			return fmt.Errorf("tag session items: %w", err)
		}
	}
	return nil
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
