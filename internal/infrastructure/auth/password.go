package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params holds the parameters for Argon2id hashing.
// Based on OWASP recommendations.
type Argon2Params struct {
	Memory      uint32 // Memory in KiB
	Iterations  uint32 // Number of iterations
	Parallelism uint8  // Number of parallel threads
	SaltLength  uint32 // Salt length in bytes
	KeyLength   uint32 // Output key length in bytes
}

// DefaultArgon2Params returns OWASP-recommended Argon2id parameters.
// These provide a good balance of security and performance.
func DefaultArgon2Params() *Argon2Params {
	return &Argon2Params{
		Memory:      64 * 1024, // 64 MB
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// PasswordHasher handles password hashing and verification using Argon2id.
type PasswordHasher struct {
	params *Argon2Params
}

// NewPasswordHasher creates a new password hasher with the given parameters.
// If params is nil, default parameters are used.
func NewPasswordHasher(params *Argon2Params) *PasswordHasher {
	if params == nil {
		params = DefaultArgon2Params()
	}
	return &PasswordHasher{params: params}
}

// Hash generates an Argon2id hash of the password.
// Returns a string in the format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
func (h *PasswordHasher) Hash(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate the hash
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		h.params.Iterations,
		h.params.Memory,
		h.params.Parallelism,
		h.params.KeyLength,
	)

	// Encode to standard format
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory,
		h.params.Iterations,
		h.params.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// Verify checks if the password matches the hash.
// Returns nil if the password is correct, ErrPasswordMismatch otherwise.
func (h *PasswordHasher) Verify(password, encodedHash string) error {
	// Parse the encoded hash
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return err
	}

	// Generate hash with the same parameters
	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(hash, otherHash) != 1 {
		return ErrPasswordMismatch
	}

	return nil
}

// ErrPasswordMismatch is returned when password verification fails.
var ErrPasswordMismatch = errors.New("password does not match")

// ErrInvalidHash is returned when the hash format is invalid.
var ErrInvalidHash = errors.New("invalid hash format")

// decodeHash parses an encoded Argon2id hash string.
func decodeHash(encodedHash string) (*Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	params := &Argon2Params{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.SaltLength = uint32(len(salt))

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	params.KeyLength = uint32(len(hash))

	return params, salt, hash, nil
}
