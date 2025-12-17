package media

import (
	"context"
	"strings"
)

// UpsertCallbacks defines the operations needed to upsert media with cache and race condition handling.
// This enables a single implementation of the complex cache-check → create → handle-race pattern
// used across movie, TV, and music processing.
//
// Usage:
//
//	callbacks := media.UpsertCallbacks{
//	    GetMediaID: func() int64 { return entity.ID },
//	    SetMediaID: func(id int64) { entity.ID = id },
//	    Update: func(ctx context.Context) error { return repo.Update(ctx, entity) },
//	    Create: func(ctx context.Context) error { return repo.Create(ctx, entity) },
//	    PostSave: func(ctx context.Context) { extractImages(ctx, entity) },
//	    EventMeta: &media.EventMetadata{Type: "movie", Title: movie.Title},
//	}
type UpsertCallbacks struct {
	// GetMediaID returns the ID from the media entity (used after cache hit or DB fetch)
	GetMediaID func() int64
	// SetMediaID sets the ID on the media entity (called when ID is retrieved from cache/DB)
	SetMediaID func(id int64)
	// Update performs the update operations on an existing media entity
	Update func(ctx context.Context) error
	// Create performs the create operation for a new media entity
	Create func(ctx context.Context) error
	// PostSave performs post-save operations like image extraction and track persistence
	PostSave func(ctx context.Context)
	// EventMeta provides optional metadata for event publishing (media.discovered/updated)
	EventMeta *EventMetadata
}

// EventMetadata holds media metadata for event publishing.
type EventMetadata struct {
	Type  string // "movie", "tv", "music"
	Title string // Media title for event data
}

// IsConstraintError checks if an error is a UNIQUE constraint violation (SQLite or PostgreSQL).
// Used during race condition handling when concurrent workers try to create the same media.
func IsConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "duplicate key")
}
