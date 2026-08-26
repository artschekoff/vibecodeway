package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AppLog is a single structured log entry stored in the database.
type AppLog struct {
	ID        int64   `json:"id"`
	Level     string  `json:"level"`
	Ts        float64 `json:"ts"`
	Caller    string  `json:"caller"`
	Msg       string  `json:"msg"`
	Error     string  `json:"error,omitempty"`
	Fields    string  `json:"fields,omitempty"`
	CreatedAt string  `json:"createdAt"`
}

// AppLogRepo handles persistence for app_logs rows.
type AppLogRepo struct {
	db *sql.DB
}

// NewAppLogRepo creates an AppLogRepo backed by db.
func NewAppLogRepo(db *sql.DB) *AppLogRepo { return &AppLogRepo{db: db} }

// Add inserts a single log entry.
func (r *AppLogRepo) Add(ctx context.Context, level string, ts float64, caller, msg, errMsg, fields string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO app_logs (level, ts, caller, msg, error, fields, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		level, ts, caller, msg, errMsg, fields, now,
	)
	if err != nil {
		return fmt.Errorf("insert app_log: %w", err)
	}
	return nil
}

// List returns the last `limit` warn/error entries, ordered oldest-first.
func (r *AppLogRepo) List(ctx context.Context, limit int) ([]AppLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, level, ts, caller, msg, error, fields, created_at
		   FROM app_logs
		  ORDER BY id DESC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list app_logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []AppLog
	for rows.Next() {
		var e AppLog
		if err := rows.Scan(&e.ID, &e.Level, &e.Ts, &e.Caller, &e.Msg, &e.Error, &e.Fields, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan app_log: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to chronological order.
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	return result, nil
}

// Clear removes all app_logs entries.
func (r *AppLogRepo) Clear(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM app_logs`)
	return err
}
