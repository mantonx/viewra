package mediaassetmodule

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
)

// HashCalculator provides hashing functionality for media assets
type HashCalculator struct{}

// NewHashCalculator creates a new hash calculator instance
func NewHashCalculator() *HashCalculator {
	return &HashCalculator{}
}

// CalculateDataHash calculates the SHA-256 hash of data
func (hc *HashCalculator) CalculateDataHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// CalculateFileHash calculates the SHA-256 hash of a file
func (hc *HashCalculator) CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w", err)
	}

	hash := hasher.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}

// ValidateHash checks if a hash string is a valid SHA-256 hash
func (hc *HashCalculator) ValidateHash(hash string) bool {
	// SHA-256 hash should be 64 hexadecimal characters
	if len(hash) != 64 {
		return false
	}
	
	// Check if it contains only hexadecimal characters
	matched, _ := regexp.MatchString("^[a-f0-9]{64}$", hash)
	return matched
}

// GetSubfolder returns the subfolder name based on hash
func (hc *HashCalculator) GetSubfolder(hash string) string {
	if len(hash) < 2 {
		return "00"
	}
	return hash[:2]
}

// Global hash calculator instance
var defaultHashCalculator *HashCalculator

// GetDefaultHashCalculator returns the default hash calculator instance
func GetDefaultHashCalculator() *HashCalculator {
	if defaultHashCalculator == nil {
		defaultHashCalculator = NewHashCalculator()
	}
	return defaultHashCalculator
}

// Convenience functions using the default hash calculator

// CalculateDataHash calculates the SHA-256 hash of data using the default calculator
func CalculateDataHash(data []byte) string {
	return GetDefaultHashCalculator().CalculateDataHash(data)
}

// CalculateFileHash calculates the SHA-256 hash of a file using the default calculator
func CalculateFileHash(filePath string) (string, error) {
	return GetDefaultHashCalculator().CalculateFileHash(filePath)
}

// ValidateHash checks if a hash string is valid using the default calculator
func ValidateHash(hash string) bool {
	return GetDefaultHashCalculator().ValidateHash(hash)
}

// GetSubfolder returns the subfolder name based on hash using the default calculator
func GetSubfolder(hash string) string {
	return GetDefaultHashCalculator().GetSubfolder(hash)
} 