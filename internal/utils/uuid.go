// Package utils provides file system utilities and media file handling functions.
// This package contains optimized file operations, hashing utilities, and media type detection
// designed for high-performance media scanning and processing.
package utils

import (
	"github.com/google/uuid"
)

// GenerateUUID generates a new UUID v4 string.
// UUID v4 uses random data and is the most common UUID type for general use.
// The probability of generating duplicate UUIDs is negligible.
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateUUIDv1 generates a UUID v1 (timestamp-based) string.
// Use this when you need UUIDs that are sortable by creation time.
// UUID v1 includes a timestamp component, making them naturally ordered
// when sorted lexicographically.
func GenerateUUIDv1() string {
	return uuid.Must(uuid.NewUUID()).String()
}

// IsValidUUID checks if a string is a valid UUID.
// Returns true if the string can be parsed as any valid UUID format
// (with or without hyphens).
func IsValidUUID(uuidStr string) bool {
	_, err := uuid.Parse(uuidStr)
	return err == nil
}

// ParseUUID parses a UUID string and returns a UUID object.
// Accepts UUID strings with or without hyphens.
// Returns an error if the string is not a valid UUID.
func ParseUUID(uuidStr string) (uuid.UUID, error) {
	return uuid.Parse(uuidStr)
}

// MustParseUUID parses a UUID string and panics if invalid.
// Only use this when you're certain the UUID is valid, such as
// with hardcoded constants or previously validated UUIDs.
// For user input or external data, use ParseUUID instead.
func MustParseUUID(uuidStr string) uuid.UUID {
	return uuid.MustParse(uuidStr)
}

// GenerateShortUUID generates a shorter UUID (first 8 characters).
// WARNING: This has higher collision probability, only use for non-critical IDs
// such as temporary identifiers, UI elements, or logging correlation IDs.
// Do not use for database primary keys or security-sensitive identifiers.
func GenerateShortUUID() string {
	return uuid.New().String()[:8]
}

// GenerateNamespaceUUID generates a UUID v5 based on a namespace and name.
// This produces deterministic UUIDs for the same namespace+name combination,
// useful for creating stable identifiers from external data sources.
// For example, generating consistent UUIDs for movies based on their IMDB ID.
func GenerateNamespaceUUID(namespace uuid.UUID, name string) string {
	return uuid.NewSHA1(namespace, []byte(name)).String()
}

// Common namespace UUIDs for different entity types.
// These namespaces are used with GenerateNamespaceUUID to create
// deterministic UUIDs for different types of media entities.
var (
	// NamespaceMovies is the namespace UUID for movie entities.
	// Use with movie identifiers (e.g., IMDB ID) to generate consistent UUIDs.
	NamespaceMovies = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceTVShows is the namespace UUID for TV show entities.
	// Use with TV show identifiers (e.g., TVDB ID) to generate consistent UUIDs.
	NamespaceTVShows = uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceSeasons is the namespace UUID for season entities.
	// Use with season identifiers (e.g., "show-id:season-number") for consistent UUIDs.
	NamespaceSeasons = uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceEpisodes is the namespace UUID for episode entities.
	// Use with episode identifiers (e.g., "show-id:s01e01") for consistent UUIDs.
	NamespaceEpisodes = uuid.MustParse("6ba7b813-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceMediaFiles is the namespace UUID for media file entities.
	// Use with file paths or content hashes to generate consistent file UUIDs.
	NamespaceMediaFiles = uuid.MustParse("6ba7b814-9dad-11d1-80b4-00c04fd430c8")
)
