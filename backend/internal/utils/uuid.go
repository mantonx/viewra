package utils

import (
	"github.com/google/uuid"
)

// GenerateUUID generates a new UUID v4 string
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateUUIDv1 generates a UUID v1 (timestamp-based) string
// Use this when you need UUIDs that are sortable by creation time
func GenerateUUIDv1() string {
	return uuid.Must(uuid.NewUUID()).String()
}

// IsValidUUID checks if a string is a valid UUID
func IsValidUUID(uuidStr string) bool {
	_, err := uuid.Parse(uuidStr)
	return err == nil
}

// ParseUUID parses a UUID string and returns a UUID object
func ParseUUID(uuidStr string) (uuid.UUID, error) {
	return uuid.Parse(uuidStr)
}

// MustParseUUID parses a UUID string and panics if invalid
// Only use this when you're certain the UUID is valid
func MustParseUUID(uuidStr string) uuid.UUID {
	return uuid.MustParse(uuidStr)
}

// GenerateShortUUID generates a shorter UUID (first 8 characters)
// WARNING: This has higher collision probability, only use for non-critical IDs
func GenerateShortUUID() string {
	return uuid.New().String()[:8]
}

// GenerateNamespaceUUID generates a UUID v5 based on a namespace and name
// This produces deterministic UUIDs for the same namespace+name combination
func GenerateNamespaceUUID(namespace uuid.UUID, name string) string {
	return uuid.NewSHA1(namespace, []byte(name)).String()
}

// Common namespace UUIDs for different entity types
var (
	// NamespaceMovies is the namespace UUID for movie entities
	NamespaceMovies = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceTVShows is the namespace UUID for TV show entities  
	NamespaceTVShows = uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceSeasons is the namespace UUID for season entities
	NamespaceSeasons = uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceEpisodes is the namespace UUID for episode entities
	NamespaceEpisodes = uuid.MustParse("6ba7b813-9dad-11d1-80b4-00c04fd430c8")
	// NamespaceMediaFiles is the namespace UUID for media file entities
	NamespaceMediaFiles = uuid.MustParse("6ba7b814-9dad-11d1-80b4-00c04fd430c8")
) 