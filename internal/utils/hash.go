// Package utils provides shared utilities for all modules.
// This file contains content hashing utilities using SHA256 for
// content-addressable storage and deduplication.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Resolution represents video resolution
type Resolution struct {
	Width  int
	Height int
}

// GenerateContentHash creates a unique content hash for transcoded media.
// The hash is based on media ID, container format, quality settings, and resolution.
// Returns a full 64-character SHA256 hash for content-addressable storage.
func GenerateContentHash(mediaID, container string, quality int, resolution *Resolution) string {
	// IMPORTANT: mediaID must not be empty to ensure uniqueness
	if mediaID == "" {
		// Generate a random component to ensure uniqueness when mediaID is missing
		// This prevents hash collisions between different media files
		mediaID = fmt.Sprintf("unknown-%d", time.Now().UnixNano())
	}
	
	// Build hash input from all relevant parameters
	hashInput := fmt.Sprintf("%s-%s-%d", mediaID, container, quality)

	// Add resolution if specified
	if resolution != nil {
		hashInput = fmt.Sprintf("%s-%dx%d", hashInput, resolution.Width, resolution.Height)
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(hashInput))

	// Return full 64-character SHA256 hash
	return hex.EncodeToString(hash[:])
}

// GenerateSessionHash creates a hash for a transcoding session.
// This is used for temporary identification before content hash is available.
func GenerateSessionHash(sessionID, provider string) string {
	hashInput := fmt.Sprintf("session-%s-%s", sessionID, provider)
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

// ValidateHash checks if a hash string is a valid SHA256 hash.
func ValidateHash(hash string) bool {
	// SHA256 produces 64 character hex strings
	if len(hash) != 64 {
		return false
	}

	// Check if all characters are valid hex
	_, err := hex.DecodeString(hash)
	return err == nil
}

// TruncateHash returns a truncated version of the hash for display purposes.
// This should NOT be used for storage or lookups, only for logging.
func TruncateHash(hash string, length int) string {
	if len(hash) <= length {
		return hash
	}
	return hash[:length] + "..."
}
