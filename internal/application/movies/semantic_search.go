package movies

import (
	"context"
)

// SemanticSearchResult represents a single result from semantic search.
type SemanticSearchResult struct {
	EntityID   int64
	EntityType string
	Similarity float32
}

// SemanticSearchProvider defines the interface for semantic search capabilities.
// This allows the movies use case to optionally use semantic search when available.
type SemanticSearchProvider interface {
	// Search performs semantic search and returns matching entity IDs with similarity scores.
	// Returns nil (not error) if semantic search is not available or fails gracefully.
	Search(ctx context.Context, query string, entityTypes []string, limit int) ([]SemanticSearchResult, error)

	// IsAvailable returns true if semantic search is currently available.
	IsAvailable() bool
}
