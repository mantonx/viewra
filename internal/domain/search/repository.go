package search

import "context"

// Repository defines the interface for search data access operations.
// This provides global search across all media types and libraries.
type Repository interface {
	// Search performs a global text search across all media.
	// It searches movies, TV shows, and optionally other media types.
	// The query is matched against titles, original titles, and plots.
	Search(ctx context.Context, req *Request) ([]Result, error)
}
