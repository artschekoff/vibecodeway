package slackstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UpsertSlackDMConfig creates or updates a DM load configuration for a context.
// Uniqueness is keyed on (context_id, workspace).
func (r *Repo) UpsertSlackDMConfig(ctx context.Context, contextID, workspace, startFrom string) (*SlackDMConfig, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM slack_dm_configs WHERE context_id = ? AND workspace = ?",
		contextID, workspace,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(ctx,
			"UPDATE slack_dm_configs SET start_from = ?, updated_at = ? WHERE id = ?",
			startFrom, now, existing,
		)
		if err != nil {
			return nil, fmt.Errorf("updating slack dm config: %w", err)
		}
		return r.getSlackDMConfig(ctx, existing)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying slack dm config: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO slack_dm_configs (id, context_id, workspace, start_from, content, created_at, updated_at, is_active)
		VALUES (?, ?, ?, ?, '', ?, ?, 1)
	`, id, contextID, workspace, startFrom, now, now)
	if err != nil {
		return nil, fmt.Errorf("inserting slack dm config: %w", err)
	}
	return r.getSlackDMConfig(ctx, id)
}

// UpdateSlackDMConfigContent stores the fetched DM content for a given config ID.
func (r *Repo) UpdateSlackDMConfigContent(ctx context.Context, id, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		"UPDATE slack_dm_configs SET content = ?, updated_at = ? WHERE id = ?",
		content, now, id,
	)
	if err != nil {
		return fmt.Errorf("updating slack dm config content: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSlackDMConfigs returns all DM configs for a context ordered by workspace.
// When activeOnly is true only rows with is_active = 1 are returned.
func (r *Repo) ListSlackDMConfigs(ctx context.Context, contextID string, activeOnly bool) ([]SlackDMConfig, error) {
	q := `
		SELECT id, context_id, workspace, start_from, content, created_at, updated_at, is_active
		FROM slack_dm_configs
		WHERE context_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY workspace"
	rows, err := r.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing slack dm configs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var configs []SlackDMConfig
	for rows.Next() {
		var c SlackDMConfig
		var isActive int
		if err := rows.Scan(&c.ID, &c.ContextID, &c.Workspace, &c.StartFrom, &c.Content, &c.CreatedAt, &c.UpdatedAt, &isActive); err != nil {
			return nil, fmt.Errorf("scanning slack dm config: %w", err)
		}
		c.IsActive = isActive == 1
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// DeleteSlackDMConfig removes a DM config by its primary key ID.
// Returns ErrNotFound if no row with that ID exists.
func (r *Repo) DeleteSlackDMConfig(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM slack_dm_configs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting slack dm config: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ToggleSlackDMConfigActive flips the is_active flag for the given DM config and returns the updated record.
func (r *Repo) ToggleSlackDMConfigActive(ctx context.Context, id string) (*SlackDMConfig, error) {
	res, err := r.db.ExecContext(ctx,
		"UPDATE slack_dm_configs SET is_active = NOT is_active, updated_at = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, fmt.Errorf("toggling slack dm config active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.getSlackDMConfig(ctx, id)
}

// RemoveSlackDMEntry removes a single DM conversation entry (by its entry ID)
// from the content of a DM config. Content may be a JSON array (legacy) or
// plain text with "# DM with ..." headers (produced by formatDMContent).
// For plain-text content the frontend generates synthetic IDs like "conv-0".
// Returns ErrNotFound if the config or entry does not exist.
func (r *Repo) RemoveSlackDMEntry(ctx context.Context, dmConfigID, entryID string) error {
	cfg, err := r.getSlackDMConfig(ctx, dmConfigID)
	if err != nil {
		return err
	}

	if cfg.Content == "" {
		return ErrNotFound
	}

	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(cfg.Content), &entries); err != nil {
		return r.removePlainTextDMEntry(ctx, cfg, entryID)
	}

	filtered := make([]json.RawMessage, 0, len(entries))
	found := false
	for _, raw := range entries {
		var stub struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &stub); err != nil {
			filtered = append(filtered, raw)
			continue
		}
		if stub.ID == entryID {
			found = true
			continue
		}
		filtered = append(filtered, raw)
	}

	if !found {
		return ErrNotFound
	}

	out, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("marshaling dm config content: %w", err)
	}

	return r.UpdateSlackDMConfigContent(ctx, dmConfigID, string(out))
}

// removePlainTextDMEntry handles the case where DM content is plain text with
// "# DM with ..." section headers. The entryID is expected to be "conv-N"
// where N is the 0-based index of the section to remove.
func (r *Repo) removePlainTextDMEntry(ctx context.Context, cfg *SlackDMConfig, entryID string) error {
	var targetIdx int
	if _, err := fmt.Sscanf(entryID, "conv-%d", &targetIdx); err != nil || targetIdx < 0 {
		return ErrNotFound
	}

	sections := splitDMSections(cfg.Content)
	if targetIdx >= len(sections) {
		return ErrNotFound
	}

	remaining := make([]string, 0, len(sections)-1)
	for i, sec := range sections {
		if i != targetIdx {
			remaining = append(remaining, sec)
		}
	}

	newContent := strings.TrimSpace(strings.Join(remaining, "\n"))
	return r.UpdateSlackDMConfigContent(ctx, cfg.ID, newContent)
}

// splitDMSections splits plain-text DM content into sections delimited by
// lines starting with "# DM with ".
func splitDMSections(content string) []string {
	var sections []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# DM with ") {
			if current.Len() > 0 {
				sections = append(sections, current.String())
				current.Reset()
			}
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}

func (r *Repo) getSlackDMConfig(ctx context.Context, id string) (*SlackDMConfig, error) {
	var c SlackDMConfig
	var isActive int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, workspace, start_from, content, created_at, updated_at, is_active
		FROM slack_dm_configs WHERE id = ?
	`, id).Scan(&c.ID, &c.ContextID, &c.Workspace, &c.StartFrom, &c.Content, &c.CreatedAt, &c.UpdatedAt, &isActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting slack dm config by id: %w", err)
	}
	c.IsActive = isActive == 1
	return &c, nil
}
