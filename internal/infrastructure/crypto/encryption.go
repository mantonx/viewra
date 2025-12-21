// Package crypto provides encryption utilities for sensitive data at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	// KeySize is the size of the encryption key in bytes (256 bits for AES-256).
	KeySize = 32

	// NonceSize is the size of the GCM nonce in bytes.
	NonceSize = 12

	// EncryptedPrefix marks encrypted values for identification.
	EncryptedPrefix = "enc:v1:"
)

// ErrInvalidKey is returned when the encryption key is invalid.
var ErrInvalidKey = errors.New("invalid encryption key")

// ErrNotEncrypted is returned when trying to decrypt a non-encrypted value.
var ErrNotEncrypted = errors.New("value is not encrypted")

// ErrDecryptionFailed is returned when decryption fails (wrong key or corrupted data).
var ErrDecryptionFailed = errors.New("decryption failed")

// Encryptor provides AES-256-GCM encryption and decryption.
type Encryptor struct {
	mu     sync.RWMutex
	key    []byte
	gcm    cipher.AEAD
	keyDir string
}

// NewEncryptor creates a new encryptor, loading or generating the key from keyDir.
// The key file will be created at {keyDir}/encryption.key if it doesn't exist.
func NewEncryptor(keyDir string) (*Encryptor, error) {
	e := &Encryptor{keyDir: keyDir}

	// Load or generate the key
	if err := e.loadOrGenerateKey(); err != nil {
		return nil, fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	return e, nil
}

// NewEncryptorWithKey creates an encryptor with a provided key (for testing).
func NewEncryptorWithKey(key []byte) (*Encryptor, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidKey, KeySize, len(key))
	}

	e := &Encryptor{key: key}
	if err := e.initGCM(); err != nil {
		return nil, err
	}

	return e, nil
}

// loadOrGenerateKey loads the encryption key from disk or generates a new one.
func (e *Encryptor) loadOrGenerateKey() error {
	keyPath := filepath.Join(e.keyDir, "encryption.key")

	// Try to read existing key
	keyData, err := os.ReadFile(keyPath)
	if err == nil {
		// Decode base64 key
		key, decErr := base64.StdEncoding.DecodeString(string(keyData))
		if decErr != nil {
			return fmt.Errorf("failed to decode key file: %w", decErr)
		}
		if len(key) != KeySize {
			return fmt.Errorf("%w: key file has wrong size (%d bytes)", ErrInvalidKey, len(key))
		}
		e.key = key
		return e.initGCM()
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Generate new key
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(e.keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write key file with restricted permissions
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	e.key = key
	return e.initGCM()
}

// initGCM initializes the GCM cipher.
func (e *Encryptor) initGCM() error {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	e.gcm = gcm
	return nil
}

// Encrypt encrypts a plaintext string and returns a prefixed base64 encoded ciphertext.
// Returns the original value if it's empty.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt with GCM (includes authentication)
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode and prefix
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return EncryptedPrefix + encoded, nil
}

// Decrypt decrypts a prefixed base64 encoded ciphertext and returns the plaintext.
// Returns the original value if it's not encrypted (no prefix).
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Check for encryption prefix
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Remove prefix and decode
	encoded := ciphertext[len(EncryptedPrefix):]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	if len(data) < NonceSize {
		return "", ErrDecryptionFailed
	}

	// Extract nonce and ciphertext
	nonce := data[:NonceSize]
	encrypted := data[NonceSize:]

	// Decrypt
	plaintext, err := e.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value is encrypted (has the encryption prefix).
func IsEncrypted(value string) bool {
	return len(value) > len(EncryptedPrefix) && value[:len(EncryptedPrefix)] == EncryptedPrefix
}

// Mask returns a masked version of a sensitive value for display.
// Shows first 4 and last 4 characters with asterisks in between.
// For encrypted values, shows "••••••••" (8 bullets).
func Mask(value string) string {
	if value == "" {
		return ""
	}

	// If encrypted, just show bullets
	if IsEncrypted(value) {
		return "••••••••"
	}

	// For short values, mask entirely
	if len(value) <= 8 {
		return "••••••••"
	}

	// Show first 4 and last 4 with asterisks
	return value[:4] + "••••" + value[len(value)-4:]
}

// MaskAPIKey masks an API key for display, showing only first 4 chars.
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if IsEncrypted(key) {
		return "••••••••••••"
	}
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:4] + "••••••••"
}
