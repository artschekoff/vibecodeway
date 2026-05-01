package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrAIKeyNotSet is returned by GetAIOpenAIKey when no encrypted key has been stored.
var ErrAIKeyNotSet = errors.New("openai key not set in database")

const aiSettingsSingletonID = 1

// SetAISettingsStoreOpts holds all fields that can be updated in ai_settings.
// Empty string fields are ignored (preserve existing values) except where noted.
// Passphrase is required when OpenAIKey or AnthropicKey is non-empty.
type SetAISettingsStoreOpts struct {
	Provider     string // "openai" | "anthropic" | "local"; empty = preserve existing
	OpenAIKey    string // empty = preserve existing ciphertext
	AnthropicKey string // empty = preserve existing ciphertext
	Model        string
	BaseURL      string
	Passphrase   string
}

// GetAISettings returns the current AI settings metadata (no secrets).
// Returns zero-value settings with Provider="openai" and HasKey=false when not yet created.
func (s *Store) GetAISettings(ctx context.Context) (*AISettings, error) {
	var (
		keyEnc      []byte
		keyNonce    []byte
		antKeyEnc   []byte
		antKeyNonce []byte
		provider    string
		model       string
		baseURL     string
		updatedAt   string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT openai_key_enc, openai_key_nonce,
		        anthropic_key_enc, anthropic_key_nonce,
		        provider, model, base_url, updated_at
		 FROM ai_settings WHERE id = ?`,
		aiSettingsSingletonID,
	).Scan(&keyEnc, &keyNonce, &antKeyEnc, &antKeyNonce, &provider, &model, &baseURL, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &AISettings{Provider: "openai"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading ai settings: %w", err)
	}
	hasOpenAI := len(keyEnc) > 0 && len(keyNonce) > 0
	hasAnthropic := len(antKeyEnc) > 0 && len(antKeyNonce) > 0
	hasKey := (provider == ProviderAnthropic && hasAnthropic) ||
		(provider != ProviderAnthropic && (hasOpenAI || provider == ProviderLocal))
	if provider == "" {
		provider = ProviderOpenAI
	}
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

// GetAIOpenAIKey returns the decrypted OpenAI API key stored in ai_settings.
// Returns ErrAIKeyNotSet when no key has been stored yet. The passphrase must be the
// same Slack passphrase used when the key was saved.
func (s *Store) GetAIOpenAIKey(ctx context.Context, passphrase string) (string, error) {
	var (
		keyEnc   []byte
		keyNonce []byte
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT openai_key_enc, openai_key_nonce FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID,
	).Scan(&keyEnc, &keyNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAIKeyNotSet
	}
	if err != nil {
		return "", fmt.Errorf("loading ai key: %w", err)
	}
	if len(keyEnc) == 0 || len(keyNonce) == 0 {
		return "", ErrAIKeyNotSet
	}
	plain, err := Decrypt(keyEnc, keyNonce, passphrase)
	if err != nil {
		return "", fmt.Errorf("decrypting ai key: %w", err)
	}
	return string(plain), nil
}

// GetAIAnthropicKey returns the decrypted Anthropic API key stored in ai_settings.
// Returns ErrAIKeyNotSet when no key has been stored yet.
func (s *Store) GetAIAnthropicKey(ctx context.Context, passphrase string) (string, error) {
	var (
		keyEnc   []byte
		keyNonce []byte
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT anthropic_key_enc, anthropic_key_nonce FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID,
	).Scan(&keyEnc, &keyNonce)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAIKeyNotSet
	}
	if err != nil {
		return "", fmt.Errorf("loading anthropic key: %w", err)
	}
	if len(keyEnc) == 0 || len(keyNonce) == 0 {
		return "", ErrAIKeyNotSet
	}
	plain, err := Decrypt(keyEnc, keyNonce, passphrase)
	if err != nil {
		return "", fmt.Errorf("decrypting anthropic key: %w", err)
	}
	return string(plain), nil
}

// SetAISettings upserts the singleton ai_settings row using the provided opts.
// Key fields (OpenAIKey / AnthropicKey) are encrypted when non-empty; empty = preserve.
// Passphrase is only required when a key field is non-empty.
func (s *Store) SetAISettings(ctx context.Context, opts SetAISettingsStoreOpts) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var openAIEnc, openAINonce, antEnc, antNonce []byte
	if opts.OpenAIKey != "" {
		ct, nc, err := Encrypt([]byte(opts.OpenAIKey), opts.Passphrase)
		if err != nil {
			return fmt.Errorf("encrypting openai key: %w", err)
		}
		openAIEnc, openAINonce = ct, nc
	}
	if opts.AnthropicKey != "" {
		ct, nc, err := Encrypt([]byte(opts.AnthropicKey), opts.Passphrase)
		if err != nil {
			return fmt.Errorf("encrypting anthropic key: %w", err)
		}
		antEnc, antNonce = ct, nc
	}

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
		_, err = tx.ExecContext(ctx,
			`INSERT INTO ai_settings
			 (id, openai_key_enc, openai_key_nonce, anthropic_key_enc, anthropic_key_nonce,
			  provider, model, base_url, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			aiSettingsSingletonID,
			openAIEnc, openAINonce, antEnc, antNonce,
			provider, opts.Model, opts.BaseURL, now,
		)
		if err != nil {
			return fmt.Errorf("inserting ai settings: %w", err)
		}
	} else {
		setClauses := []string{"model = ?", "base_url = ?", "updated_at = ?"}
		args := []any{opts.Model, opts.BaseURL, now}
		if opts.Provider != "" {
			setClauses = append(setClauses, "provider = ?")
			args = append(args, opts.Provider)
		}
		if opts.OpenAIKey != "" {
			setClauses = append(setClauses, "openai_key_enc = ?", "openai_key_nonce = ?")
			args = append(args, openAIEnc, openAINonce)
		}
		if opts.AnthropicKey != "" {
			setClauses = append(setClauses, "anthropic_key_enc = ?", "anthropic_key_nonce = ?")
			args = append(args, antEnc, antNonce)
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

// ClearAIOpenAIKey removes the stored encrypted OpenAI key, preserving model/baseURL.
func (s *Store) ClearAIOpenAIKey(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE ai_settings SET openai_key_enc = NULL, openai_key_nonce = NULL, updated_at = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339), aiSettingsSingletonID,
	)
	if err != nil {
		return fmt.Errorf("clearing ai key: %w", err)
	}
	return nil
}
