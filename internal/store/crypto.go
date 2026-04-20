package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

// ErrNoPassphrase is returned by Encrypt when an empty passphrase is supplied.
var ErrNoPassphrase = errors.New("passphrase is empty")

// kdfSalt is a fixed salt used to derive the AES key from the user's Slack passphrase.
// Using a static salt is acceptable here because the passphrase itself acts as the per-user
// secret and the ciphertext never leaves the user's machine; the rotating GCM nonce provides
// the per-record randomness required for AES-GCM.
var kdfSalt = []byte("context-consolidator/ai-settings/v1")

const (
	scryptN      = 1 << 15 // 32768
	scryptR      = 8
	scryptP      = 1
	aesKeyLen    = 32 // AES-256
	gcmNonceSize = 12
)

// Encrypt AES-256-GCM encrypts plaintext with a key derived from passphrase via scrypt.
// Returns (ciphertext, nonce, error). The returned ciphertext includes the GCM auth tag.
// Passing an empty passphrase returns ErrNoPassphrase.
func Encrypt(plaintext []byte, passphrase string) (ciphertext, nonce []byte, err error) {
	if passphrase == "" {
		return nil, nil, ErrNoPassphrase
	}
	key, err := deriveKey(passphrase)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating gcm: %w", err)
	}
	nonce = make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt reverses Encrypt. Returns ErrNoPassphrase when passphrase is empty.
func Decrypt(ciphertext, nonce []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, ErrNoPassphrase
	}
	if len(nonce) != gcmNonceSize {
		return nil, fmt.Errorf("invalid nonce length: got %d, want %d", len(nonce), gcmNonceSize)
	}
	key, err := deriveKey(passphrase)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return plaintext, nil
}

func deriveKey(passphrase string) ([]byte, error) {
	key, err := scrypt.Key([]byte(passphrase), kdfSalt, scryptN, scryptR, scryptP, aesKeyLen)
	if err != nil {
		return nil, fmt.Errorf("deriving key: %w", err)
	}
	return key, nil
}
