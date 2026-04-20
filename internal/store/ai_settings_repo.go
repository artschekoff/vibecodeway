package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrAIKeyNotSet is returned by GetAIOpenAIKey when no encrypted key has been stored.
var ErrAIKeyNotSet = errors.New("openai key not set in database")

const aiSettingsSingletonID = 1

// GetAISettings returns the current AI settings metadata (no secrets).
// Returns zero-value settings with HasKey=false when the row has not been created yet.
func (s *Store) GetAISettings(ctx context.Context) (*AISettings, error) {
	var (
		keyEnc    []byte
		keyNonce  []byte
		model     string
		baseURL   string
		updatedAt string
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT openai_key_enc, openai_key_nonce, model, base_url, updated_at FROM ai_settings WHERE id = ?",
		aiSettingsSingletonID,
	).Scan(&keyEnc, &keyNonce, &model, &baseURL, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &AISettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading ai settings: %w", err)
	}
	return &AISettings{
		HasKey:    len(keyEnc) > 0 && len(keyNonce) > 0,
		Model:     model,
		BaseURL:   baseURL,
		UpdatedAt: updatedAt,
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

// SetAISettings upserts the singleton ai_settings row.
// When apiKey is non-empty it is encrypted with the provided passphrase and stored.
// When apiKey is empty the previously stored ciphertext is preserved (only model/baseURL update).
// Passing a passphrase is only required when apiKey is non-empty; otherwise it may be empty.
func (s *Store) SetAISettings(ctx context.Context, apiKey, model, baseURL, passphrase string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var (
		keyEnc   []byte
		keyNonce []byte
	)
	if apiKey != "" {
		ct, nc, err := Encrypt([]byte(apiKey), passphrase)
		if err != nil {
			return fmt.Errorf("encrypting ai key: %w", err)
		}
		keyEnc = ct
		keyNonce = nc
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning ai settings tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_settings WHERE id = ?", aiSettingsSingletonID).Scan(&existing)
	if err != nil {
		return fmt.Errorf("checking ai settings row: %w", err)
	}

	if existing == 0 {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO ai_settings (id, openai_key_enc, openai_key_nonce, model, base_url, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			aiSettingsSingletonID, keyEnc, keyNonce, model, baseURL, now,
		)
		if err != nil {
			return fmt.Errorf("inserting ai settings: %w", err)
		}
	} else if apiKey != "" {
		_, err := tx.ExecContext(ctx,
			"UPDATE ai_settings SET openai_key_enc = ?, openai_key_nonce = ?, model = ?, base_url = ?, updated_at = ? WHERE id = ?",
			keyEnc, keyNonce, model, baseURL, now, aiSettingsSingletonID,
		)
		if err != nil {
			return fmt.Errorf("updating ai settings: %w", err)
		}
	} else {
		_, err := tx.ExecContext(ctx,
			"UPDATE ai_settings SET model = ?, base_url = ?, updated_at = ? WHERE id = ?",
			model, baseURL, now, aiSettingsSingletonID,
		)
		if err != nil {
			return fmt.Errorf("updating ai settings metadata: %w", err)
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
