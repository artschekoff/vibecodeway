package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrAIKeyNotSet is returned by GetAIOpenAIKey and GetAIAnthropicKey when no key has been stored.
var ErrAIKeyNotSet = errors.New("ai key not set in database")

const aiSettingsSingletonID = 1

// SetAISettingsStoreOpts holds all fields that can be updated in ai_settings.
// Empty string fields are ignored (preserve existing values) except where noted.
type SetAISettingsStoreOpts struct {
	Provider     string // "openai" | "anthropic" | "local"; empty = preserve existing
	OpenAIKey    string // empty = preserve existing
	AnthropicKey string // empty = preserve existing
	Model        string
	// BaseURL to set; empty string clears the stored base URL (unlike key fields which preserve on empty).
	BaseURL string
}

// GetAISettings returns the current AI settings metadata (no secrets).
// Returns zero-value settings with Provider="openai" and HasKey=false when not yet created.
func (s *Store) GetAISettings(ctx context.Context) (*AISettings, error) {
	var (
		keyPlain    sql.NullString
		antKeyPlain sql.NullString
		// legacy encrypted columns — kept for schema compatibility
		keyEnc      []byte
		keyNonce    []byte
		antKeyEnc   []byte
		antKeyNonce []byte
		provider    string
		model       string
		baseURL     string
		updatedAt   string
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT openai_key, anthropic_key,
		        openai_key_enc, openai_key_nonce,
		        anthropic_key_enc, anthropic_key_nonce,
		        provider, model, base_url, updated_at
		 FROM ai_settings WHERE id = ?`,
		aiSettingsSingletonID,
	).Scan(&keyPlain, &antKeyPlain, &keyEnc, &keyNonce, &antKeyEnc, &antKeyNonce, &provider, &model, &baseURL, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &AISettings{Provider: "openai"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading ai settings: %w", err)
	}
	if provider == "" {
		provider = ProviderOpenAI
	}
	hasOpenAI := (keyPlain.Valid && keyPlain.String != "") || (len(keyEnc) > 0 && len(keyNonce) > 0)
	hasAnthropic := (antKeyPlain.Valid && antKeyPlain.String != "") || (len(antKeyEnc) > 0 && len(antKeyNonce) > 0)
	hasKey := (provider == ProviderAnthropic && hasAnthropic) ||
		(provider != ProviderAnthropic && (hasOpenAI || provider == ProviderLocal))
	return &AISettings{
		Provider:        provider,
		HasKey:          hasKey,
		HasOpenAIKey:    hasOpenAI,
		HasAnthropicKey: hasAnthropic,
		Model:           model,
		BaseURL:         baseURL,
		UpdatedAt:       updatedAt,
	}, nil
}

// GetAIOpenAIKey returns the stored OpenAI API key.
// Returns ErrAIKeyNotSet when no key has been stored yet.
func (s *Store) GetAIOpenAIKey(ctx context.Context) (string, error) {
	var key sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		"SELECT openai_key FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID,
	).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAIKeyNotSet
	}
	if err != nil {
		return "", fmt.Errorf("loading ai key: %w", err)
	}
	if !key.Valid || key.String == "" {
		return "", ErrAIKeyNotSet
	}
	return key.String, nil
}

// GetAIAnthropicKey returns the stored Anthropic API key.
// Returns ErrAIKeyNotSet when no key has been stored yet.
func (s *Store) GetAIAnthropicKey(ctx context.Context) (string, error) {
	var key sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		"SELECT anthropic_key FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID,
	).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAIKeyNotSet
	}
	if err != nil {
		return "", fmt.Errorf("loading anthropic key: %w", err)
	}
	if !key.Valid || key.String == "" {
		return "", ErrAIKeyNotSet
	}
	return key.String, nil
}

// SetAISettings upserts the singleton ai_settings row using the provided opts.
// Empty key fields preserve the existing stored key.
func (s *Store) SetAISettings(ctx context.Context, opts SetAISettingsStoreOpts) error {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning ai settings tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID).Scan(&existing); err != nil {
		return fmt.Errorf("checking ai settings row: %w", err)
	}

	if existing == 0 {
		provider := opts.Provider
		if provider == "" {
			provider = ProviderOpenAI
		}
		var openAIKey, antKey *string
		if opts.OpenAIKey != "" {
			openAIKey = &opts.OpenAIKey
		}
		if opts.AnthropicKey != "" {
			antKey = &opts.AnthropicKey
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO ai_settings
			 (id, openai_key, anthropic_key,
			  openai_key_enc, openai_key_nonce, anthropic_key_enc, anthropic_key_nonce,
			  provider, model, base_url, updated_at)
			 VALUES (?, ?, ?, NULL, NULL, NULL, NULL, ?, ?, ?, ?)`,
			aiSettingsSingletonID,
			openAIKey, antKey,
			provider, opts.Model, opts.BaseURL, now,
		)
		if err != nil {
			return fmt.Errorf("inserting ai settings: %w", err)
		}
	} else {
		setClauses := []string{}
		args := []any{}
		if opts.Provider != "" {
			setClauses = append(setClauses, "provider = ?")
			args = append(args, opts.Provider)
		}
		if opts.Model != "" {
			setClauses = append(setClauses, "model = ?")
			args = append(args, opts.Model)
		}
		setClauses = append(setClauses, "base_url = ?", "updated_at = ?")
		args = append(args, opts.BaseURL, now)
		if opts.OpenAIKey != "" {
			setClauses = append(setClauses, "openai_key = ?")
			args = append(args, opts.OpenAIKey)
		}
		if opts.AnthropicKey != "" {
			setClauses = append(setClauses, "anthropic_key = ?")
			args = append(args, opts.AnthropicKey)
		}
		args = append(args, aiSettingsSingletonID)
		query := "UPDATE ai_settings SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("updating ai settings: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing ai settings: %w", err)
	}
	return nil
}

// ClearAIOpenAIKey removes the stored OpenAI key, preserving model/baseURL.
func (s *Store) ClearAIOpenAIKey(ctx context.Context) error {
	_, err := s.db.ExecContext(
		ctx,
		"UPDATE ai_settings SET openai_key = NULL, openai_key_enc = NULL, openai_key_nonce = NULL, updated_at = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339), aiSettingsSingletonID,
	)
	if err != nil {
		return fmt.Errorf("clearing ai key: %w", err)
	}
	return nil
}
