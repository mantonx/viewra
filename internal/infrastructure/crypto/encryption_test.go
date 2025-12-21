package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptor_RoundTrip(t *testing.T) {
	// Create encryptor with known key
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewEncryptorWithKey(key)
	if err != nil {
		t.Fatalf("NewEncryptorWithKey failed: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"short string", "test"},
		{"api key", "sk-abc123xyz789"},
		{"with special chars", "pass!@#$%^&*()"},
		{"unicode", "日本語テスト"},
		{"long text", "This is a much longer piece of text that contains multiple words and sentences."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := enc.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Empty string should return empty
			if tc.plaintext == "" {
				if ciphertext != "" {
					t.Errorf("Expected empty ciphertext for empty plaintext, got %q", ciphertext)
				}
				return
			}

			// Ciphertext should have prefix
			if !IsEncrypted(ciphertext) {
				t.Errorf("Ciphertext should have encryption prefix")
			}

			// Decrypt
			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("Round-trip failed: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncryptor_DifferentCiphertexts(t *testing.T) {
	// Each encryption should produce different ciphertext (due to random nonce)
	key := make([]byte, KeySize)
	enc, err := NewEncryptorWithKey(key)
	if err != nil {
		t.Fatalf("NewEncryptorWithKey failed: %v", err)
	}

	plaintext := "same-value"
	ciphertext1, _ := enc.Encrypt(plaintext)
	ciphertext2, _ := enc.Encrypt(plaintext)

	if ciphertext1 == ciphertext2 {
		t.Error("Encrypting same value twice should produce different ciphertexts")
	}

	// Both should decrypt to same value
	dec1, _ := enc.Decrypt(ciphertext1)
	dec2, _ := enc.Decrypt(ciphertext2)

	if dec1 != dec2 || dec1 != plaintext {
		t.Error("Both ciphertexts should decrypt to the same plaintext")
	}
}

func TestEncryptor_WrongKey(t *testing.T) {
	// Create two encryptors with different keys
	key1 := make([]byte, KeySize)
	key2 := make([]byte, KeySize)
	key2[0] = 1 // Different key

	enc1, _ := NewEncryptorWithKey(key1)
	enc2, _ := NewEncryptorWithKey(key2)

	plaintext := "secret"
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Decrypting with wrong key should fail
	_, err := enc2.Decrypt(ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("Expected ErrDecryptionFailed, got %v", err)
	}
}

func TestEncryptor_InvalidKey(t *testing.T) {
	// Key too short
	shortKey := make([]byte, 16)
	_, err := NewEncryptorWithKey(shortKey)
	if err == nil {
		t.Error("Expected error for short key")
	}

	// Key too long
	longKey := make([]byte, 64)
	_, err = NewEncryptorWithKey(longKey)
	if err == nil {
		t.Error("Expected error for long key")
	}
}

func TestEncryptor_DecryptNonEncrypted(t *testing.T) {
	key := make([]byte, KeySize)
	enc, _ := NewEncryptorWithKey(key)

	// Non-encrypted value should be returned as-is
	plainValue := "not-encrypted"
	result, err := enc.Decrypt(plainValue)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if result != plainValue {
		t.Errorf("Expected %q, got %q", plainValue, result)
	}
}

func TestEncryptor_CorruptedCiphertext(t *testing.T) {
	key := make([]byte, KeySize)
	enc, _ := NewEncryptorWithKey(key)

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{"too short", EncryptedPrefix + "abc"},
		{"invalid base64", EncryptedPrefix + "not-valid-base64!!!"},
		{"truncated", EncryptedPrefix + "YWJj"}, // valid base64, but too short
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := enc.Decrypt(tc.ciphertext)
			if err == nil {
				t.Error("Expected error for corrupted ciphertext")
			}
		})
	}
}

func TestNewEncryptor_KeyPersistence(t *testing.T) {
	// Create temp directory for key
	tmpDir := t.TempDir()

	// Create first encryptor (generates key)
	enc1, err := NewEncryptor(tmpDir)
	if err != nil {
		t.Fatalf("First NewEncryptor failed: %v", err)
	}

	plaintext := "test-value"
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Create second encryptor (loads existing key)
	enc2, err := NewEncryptor(tmpDir)
	if err != nil {
		t.Fatalf("Second NewEncryptor failed: %v", err)
	}

	// Should be able to decrypt with second encryptor
	decrypted, err := enc2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt with second encryptor failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %q, got %q", plaintext, decrypted)
	}

	// Verify key file exists
	keyPath := filepath.Join(tmpDir, "encryption.key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file should exist")
	}
}

func TestIsEncrypted(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"", false},
		{"plain-text", false},
		{"enc:", false},
		{"enc:v1", false},
		{EncryptedPrefix, false}, // Just prefix, no data
		{EncryptedPrefix + "data", true},
	}

	for _, tc := range testCases {
		result := IsEncrypted(tc.value)
		if result != tc.expected {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tc.value, result, tc.expected)
		}
	}
}

func TestMask(t *testing.T) {
	testCases := []struct {
		value    string
		expected string
	}{
		{"", ""},
		{"short", "••••••••"},
		{"12345678", "••••••••"},
		{"123456789", "1234••••6789"},
		{"sk-1234567890abcdef", "sk-1••••cdef"},
		{EncryptedPrefix + "data", "••••••••"},
	}

	for _, tc := range testCases {
		result := Mask(tc.value)
		if result != tc.expected {
			t.Errorf("Mask(%q) = %q, want %q", tc.value, result, tc.expected)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	testCases := []struct {
		value    string
		expected string
	}{
		{"", ""},
		{"short", "••••••••"},
		{"12345678", "••••••••"},
		{"sk-123456789", "sk-1••••••••"},
		{EncryptedPrefix + "data", "••••••••••••"},
	}

	for _, tc := range testCases {
		result := MaskAPIKey(tc.value)
		if result != tc.expected {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tc.value, result, tc.expected)
		}
	}
}
