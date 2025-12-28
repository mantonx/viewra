package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// ResponseApplier coordinates applying enrichment results to the database.
type ResponseApplier struct {
	deps            *Deps
	metadataApplier *MetadataApplier
	creditsApplier  *CreditsApplier
	studiosApplier  *StudiosApplier
	keywordsApplier *KeywordsApplier
	imageProcessor  *ImageProcessor
	logger          *slog.Logger
}

// NewResponseApplier creates a new ResponseApplier.
func NewResponseApplier(deps *Deps, metadataApplier *MetadataApplier, creditsApplier *CreditsApplier, studiosApplier *StudiosApplier, keywordsApplier *KeywordsApplier, imageProcessor *ImageProcessor, logger *slog.Logger) *ResponseApplier {
	return &ResponseApplier{
		deps:            deps,
		metadataApplier: metadataApplier,
		creditsApplier:  creditsApplier,
		studiosApplier:  studiosApplier,
		keywordsApplier: keywordsApplier,
		imageProcessor:  imageProcessor,
		logger:          logger,
	}
}

// Apply applies enrichment results to the database.
// Prioritizes metadata and images over external IDs - external ID failures are logged but don't block other updates.
func (a *ResponseApplier) Apply(ctx context.Context, job *enrichment.QueueJob, mediaType enrichment.MediaType, resp *pluginv1.EnrichResponse) error {
	mediaID := job.MediaID

	// Apply metadata updates first - this is the most important data
	if resp.Metadata != nil && a.metadataApplier != nil {
		if err := a.metadataApplier.Apply(ctx, mediaID, mediaType, resp.Metadata); err != nil {
			return fmt.Errorf("apply metadata: %w", err)
		}

		// After applying metadata, update priority based on actual release date.
		// This replaces the initial estimate (from filename year or file mtime)
		// with the authoritative date from the metadata provider.
		a.updatePriorityFromMetadata(ctx, mediaID, mediaType, resp.Metadata)
	}

	// Apply credits (cast, directors, writers, creators)
	if resp.Metadata != nil && a.creditsApplier != nil {
		if err := a.creditsApplier.Apply(ctx, mediaID, mediaType, resp.Metadata); err != nil {
			// Log but don't fail - credits are supplementary data
			a.logger.Warn("failed to apply credits",
				slog.Int64("media_id", mediaID),
				slog.String("media_type", string(mediaType)),
				slog.Any("error", err))
		}
	}

	// Apply studios (production companies)
	if resp.Metadata != nil && a.studiosApplier != nil {
		if err := a.studiosApplier.Apply(ctx, mediaID, mediaType, resp.Metadata); err != nil {
			// Log but don't fail - studios are supplementary data
			a.logger.Warn("failed to apply studios",
				slog.Int64("media_id", mediaID),
				slog.String("media_type", string(mediaType)),
				slog.Any("error", err))
		}
	}

	// Apply keywords (for location-based search and thematic tagging)
	if resp.Metadata != nil && a.keywordsApplier != nil {
		if err := a.keywordsApplier.Apply(ctx, mediaID, mediaType, resp.Metadata); err != nil {
			// Log but don't fail - keywords are supplementary data
			a.logger.Warn("failed to apply keywords",
				slog.Int64("media_id", mediaID),
				slog.String("media_type", string(mediaType)),
				slog.Any("error", err))
		}
	}

	// Process images (local paths and remote URLs)
	if len(resp.Images) > 0 && a.imageProcessor != nil {
		if err := a.imageProcessor.Process(ctx, mediaID, mediaType, resp.Images); err != nil {
			return fmt.Errorf("process images: %w", err)
		}
	}

	// Store discovered external IDs last - failures here shouldn't block metadata/images
	for provider, id := range resp.DiscoveredIds {
		// Determine if this entity has a media table entry
		// Only movie, tv (episode), and music (track) have media.id references
		var mediaIDPtr *int64
		if isMediaTableEntity(mediaType) {
			mediaIDPtr = &mediaID
		}

		extID := &enrichment.ExternalID{
			MediaID:    mediaIDPtr,
			MediaType:  convertToExternalIDMediaType(mediaType),
			EntityID:   mediaID, // The ID in the entity's own table
			Provider:   provider,
			ExternalID: id,
		}
		if err := a.deps.ExternalIDRepo.Upsert(ctx, extID); err != nil {
			// Log but don't fail - external IDs are supplementary data
			a.logger.Warn("failed to store external ID",
				slog.String("provider", provider),
				slog.String("media_type", string(mediaType)),
				slog.Int64("entity_id", mediaID),
				slog.Any("error", err))
		}
	}

	return nil
}

// updatePriorityFromMetadata recalculates and updates priority based on the actual
// release date discovered during enrichment. This replaces the initial estimate
// (from filename year or file mtime) with the authoritative date.
func (a *ResponseApplier) updatePriorityFromMetadata(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) {
	// Extract the appropriate date field based on media type
	var dateStr string
	switch mediaType {
	case enrichment.MediaTypeMovie:
		dateStr = metadata.GetReleaseDate()
	case enrichment.MediaTypeTVShow, enrichment.MediaTypeTV:
		// TV uses "premiered" (first_air_date) for the show/episode air date
		dateStr = metadata.GetPremiered()
	case enrichment.MediaTypeMusicAlbum:
		// Albums have release dates but they come through AlbumMetadata, not EnrichedMetadata
		// For now, skip album re-prioritization here; it would need separate handling
		return
	default:
		// Other types (tracks, artists, seasons) don't have release dates to update
		return
	}

	if dateStr == "" {
		return
	}

	// Parse the date string (expected format: YYYY-MM-DD)
	releaseDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		a.logger.Debug("could not parse release date for priority update",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)),
			slog.String("date_str", dateStr),
			slog.Any("error", err))
		return
	}

	// Calculate priority based on actual release date
	newPriority := CalculatePriority(releaseDate)

	// Update priority for all pending/processing jobs for this media item
	if err := a.deps.QueueRepo.UpdatePriorityByMedia(ctx, mediaID, mediaType, newPriority); err != nil {
		a.logger.Warn("failed to update priority after enrichment",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)),
			slog.Int("new_priority", newPriority),
			slog.Any("error", err))
	} else {
		a.logger.Debug("updated priority based on actual release date",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)),
			slog.String("release_date", dateStr),
			slog.Int("priority", newPriority))
	}
}
