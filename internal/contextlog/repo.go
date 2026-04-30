package contextlog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a targeted row does not exist.
var ErrNotFound = errors.New("context log record not found")

// Repo provides access to context_logs / consolidated_context_logs / context_log_chat_messages.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a new repository over the shared *sql.DB.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// InsertLog persists a new ContextLog. If log.ID is empty a fresh UUID is assigned.
// If log.CreatedAt is empty the current UTC time is used. The stored record is returned.
func (r *Repo) InsertLog(ctx context.Context, log ContextLog) (*ContextLog, error) {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	if log.CreatedAt == "" {
		log.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO context_logs
			(id, context_id, trigger, source, model, prompt_tokens, completion_tokens,
			 diff_json, summary_md, is_manual, edited_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.ContextID, log.Trigger, log.Source, log.Model,
		log.PromptTokens, log.CompletionTokens, log.DiffJSON, log.SummaryMd,
		boolToInt(log.IsManual), nullableString(log.EditedAt), log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting context log: %w", err)
	}
	return &log, nil
}

// UpdateLogSummary rewrites summary_md and marks the record as user-edited.
// Returns the updated record or ErrNotFound.
func (r *Repo) UpdateLogSummary(ctx context.Context, id, summaryMd string) (*ContextLog, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		"UPDATE context_logs SET summary_md = ?, edited_at = ? WHERE id = ?",
		summaryMd, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating context log: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.GetLog(ctx, id)
}

// GetLog fetches a single log entry by id.
func (r *Repo) GetLog(ctx context.Context, id string) (*ContextLog, error) {
	row := r.db.QueryRowContext(ctx, selectLogColumns+" FROM context_logs WHERE id = ?", id)
	log, err := scanLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading context log: %w", err)
	}
	return log, nil
}

// DeleteLog removes a single log entry by id.
func (r *Repo) DeleteLog(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM context_logs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting context log: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListLogs returns all logs for contextID sorted from newest to oldest.
func (r *Repo) ListLogs(ctx context.Context, contextID string) ([]ContextLog, error) {
	rows, err := r.db.QueryContext(ctx,
		selectLogColumns+" FROM context_logs WHERE context_id = ? ORDER BY created_at DESC",
		contextID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing context logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ContextLog
	for rows.Next() {
		log, err := scanLog(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning context log: %w", err)
		}
		out = append(out, *log)
	}
	return out, rows.Err()
}

// GetConsolidated returns the consolidated log for contextID or nil when absent.
func (r *Repo) GetConsolidated(ctx context.Context, contextID string) (*ConsolidatedLog, error) {
	var c ConsolidatedLog
	err := r.db.QueryRowContext(ctx,
		"SELECT context_id, content_md, version, updated_by, updated_at FROM consolidated_context_logs WHERE context_id = ?",
		contextID,
	).Scan(&c.ContextID, &c.ContentMd, &c.Version, &c.UpdatedBy, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading consolidated log: %w", err)
	}
	return &c, nil
}

// UpsertConsolidated inserts or replaces the consolidated log for contextID.
// Version is incremented on every update.
func (r *Repo) UpsertConsolidated(ctx context.Context, contextID, contentMd, updatedBy string) (*ConsolidatedLog, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin consolidated tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	var currentVersion int
	err = tx.QueryRowContext(ctx,
		"SELECT version FROM consolidated_context_logs WHERE context_id = ?",
		contextID,
	).Scan(&currentVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		existing = 0
	case err != nil:
		return nil, fmt.Errorf("checking consolidated log: %w", err)
	default:
		existing = 1
	}

	newVersion := 1
	if existing == 1 {
		newVersion = currentVersion + 1
		_, err := tx.ExecContext(ctx,
			"UPDATE consolidated_context_logs SET content_md = ?, version = ?, updated_by = ?, updated_at = ? WHERE context_id = ?",
			contentMd, newVersion, updatedBy, now, contextID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating consolidated log: %w", err)
		}
	} else {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO consolidated_context_logs (context_id, content_md, version, updated_by, updated_at) VALUES (?, ?, ?, ?, ?)",
			contextID, contentMd, newVersion, updatedBy, now,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting consolidated log: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing consolidated log: %w", err)
	}

	return &ConsolidatedLog{
		ContextID: contextID,
		ContentMd: contentMd,
		Version:   newVersion,
		UpdatedBy: updatedBy,
		UpdatedAt: now,
	}, nil
}

// InsertChatMessage stores a new chat message and returns the created record.
func (r *Repo) InsertChatMessage(ctx context.Context, msg ChatMessage) (*ChatMessage, error) {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.CreatedAt == "" {
		msg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO context_log_chat_messages
			(id, context_id, role, content, proposed_consolidated_md, applied_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ContextID, msg.Role, msg.Content,
		nullableString(msg.ProposedConsolidatedMd), nullableString(msg.AppliedAt), msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting chat message: %w", err)
	}
	return &msg, nil
}

// MarkChatMessageApplied sets applied_at to now on the specified message.
func (r *Repo) MarkChatMessageApplied(ctx context.Context, id string) (*ChatMessage, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		"UPDATE context_log_chat_messages SET applied_at = ? WHERE id = ?",
		now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("marking chat message applied: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.GetChatMessage(ctx, id)
}

// GetChatMessage fetches a single chat message by id.
func (r *Repo) GetChatMessage(ctx context.Context, id string) (*ChatMessage, error) {
	row := r.db.QueryRowContext(ctx, selectChatColumns+" FROM context_log_chat_messages WHERE id = ?", id)
	msg, err := scanChatMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading chat message: %w", err)
	}
	return msg, nil
}

// ListChatMessages returns all chat messages for contextID ordered by created_at ASC.
// limit <= 0 means "no limit". When limit > 0, the N most recent messages are returned
// in chronological order.
func (r *Repo) ListChatMessages(ctx context.Context, contextID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		rows, err := r.db.QueryContext(ctx,
			selectChatColumns+" FROM context_log_chat_messages WHERE context_id = ? ORDER BY created_at ASC",
			contextID,
		)
		if err != nil {
			return nil, fmt.Errorf("listing chat messages: %w", err)
		}
		defer func() { _ = rows.Close() }()
		return scanChatRows(rows)
	}

	rows, err := r.db.QueryContext(ctx,
		selectChatColumns+" FROM context_log_chat_messages WHERE context_id = ? ORDER BY created_at DESC LIMIT ?",
		contextID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing chat messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	recent, err := scanChatRows(rows)
	if err != nil {
		return nil, err
	}
	// Reverse into ascending order.
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}
	return recent, nil
}

// --- internal helpers ---

const selectLogColumns = `SELECT id, context_id, trigger, source, model, prompt_tokens, completion_tokens,
	diff_json, summary_md, is_manual, COALESCE(edited_at, ''), created_at`

const selectChatColumns = `SELECT id, context_id, role, content,
	COALESCE(proposed_consolidated_md, ''), COALESCE(applied_at, ''), created_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanLog(s scanner) (*ContextLog, error) {
	var (
		log      ContextLog
		isManual int
	)
	err := s.Scan(
		&log.ID, &log.ContextID, &log.Trigger, &log.Source, &log.Model,
		&log.PromptTokens, &log.CompletionTokens, &log.DiffJSON, &log.SummaryMd,
		&isManual, &log.EditedAt, &log.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	log.IsManual = isManual != 0
	return &log, nil
}

func scanChatMessage(s scanner) (*ChatMessage, error) {
	var msg ChatMessage
	err := s.Scan(
		&msg.ID, &msg.ContextID, &msg.Role, &msg.Content,
		&msg.ProposedConsolidatedMd, &msg.AppliedAt, &msg.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func scanChatRows(rows *sql.Rows) ([]ChatMessage, error) {
	var out []ChatMessage
	for rows.Next() {
		msg, err := scanChatMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning chat message: %w", err)
		}
		out = append(out, *msg)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
