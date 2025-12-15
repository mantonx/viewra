package media

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// ProcessMediaWithCache handles the common pattern of cache-based media upsert with race condition handling.
// This eliminates ~100 lines of duplicated code across movie, TV, and music processors.
//
// The pattern handled is:
//  1. Check cache for existing media ID → if found, update and return
//  2. Try to create new media → if successful, add to cache and return
//  3. On UNIQUE constraint error (race condition):
//     a. Check cache again (another worker may have added it)
//     b. If still not in cache, fetch from database
//     c. Update the existing record
//     d. Add to cache for future lookups
func ProcessMediaWithCache(
	ctx context.Context,
	deps *Deps,
	libraryID int64,
	filePath string,
	cache *sync.Map,
	callbacks UpsertCallbacks,
) (*int64, error) {
	// Check if media already exists using in-memory cache (major performance optimization!)
	// This eliminates individual database SELECTs for every file
	if value, found := cache.Load(filePath); found {
		callbacks.SetMediaID(value.(int64))
		if err := callbacks.Update(ctx); err != nil {
			return nil, err
		}
		callbacks.PostSave(ctx)
		id := callbacks.GetMediaID()
		return &id, nil
	}

	// Try to create new entry
	if err := callbacks.Create(ctx); err != nil {
		// Handle race condition: Another worker may have created this between our check and insert
		if !IsConstraintError(err) {
			return nil, err
		}

		// Check cache again (another worker may have just added it)
		if value, found := cache.Load(filePath); found {
			callbacks.SetMediaID(value.(int64))
		} else {
			// Cache miss - fetch from database (race condition: created after our initial cache load)
			existing, fetchErr := deps.MediaRepos.Media.GetByFilePath(ctx, libraryID, filePath)
			if fetchErr != nil || existing == nil {
				return nil, fmt.Errorf("failed to fetch existing media after collision: %w", fetchErr)
			}
			callbacks.SetMediaID(existing.ID)
			// Add to cache for future lookups
			cache.Store(filePath, existing.ID)
		}

		// Update the existing record
		if updateErr := callbacks.Update(ctx); updateErr != nil {
			return nil, updateErr
		}
		callbacks.PostSave(ctx)
		id := callbacks.GetMediaID()
		return &id, nil
	}

	// Success: add newly created media to cache so other workers don't try to create it again
	cache.Store(filePath, callbacks.GetMediaID())
	callbacks.PostSave(ctx)
	id := callbacks.GetMediaID()
	return &id, nil
}

// enqueueForEnrichment fires off enrichment for newly saved/updated media.
// This is fire-and-forget - errors are logged but don't fail the scan.
func enqueueForEnrichment(ctx context.Context, deps *Deps, mediaID int64, mediaType enrichment.MediaType) {
	if deps.EnrichmentEnqueuer == nil {
		return // Enrichment not configured
	}

	// Use background context to avoid cancellation from scan context
	go func() {
		if err := deps.EnrichmentEnqueuer.EnqueueFirstStage(context.Background(), mediaID, mediaType); err != nil {
			deps.Logger.Warn("failed to enqueue for enrichment",
				slog.Int64("media_id", mediaID),
				slog.String("media_type", string(mediaType)),
				slog.Any("error", err))
		}
	}()
}
