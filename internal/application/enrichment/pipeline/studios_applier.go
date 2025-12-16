package pipeline

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
)

// StudiosApplier handles applying studios from enrichment to the database.
type StudiosApplier struct {
	studioRepo media.StudioRepository
	logger     *slog.Logger
}

// NewStudiosApplier creates a new StudiosApplier.
func NewStudiosApplier(studioRepo media.StudioRepository, logger *slog.Logger) *StudiosApplier {
	return &StudiosApplier{
		studioRepo: studioRepo,
		logger:     logger,
	}
}

// Apply applies studios from enriched metadata to the database.
func (a *StudiosApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error {
	if a.studioRepo == nil {
		return nil // No studio repo configured
	}

	// Only movies and TV shows have studios
	entityType := studioMediaTypeToEntityType(mediaType)
	if entityType == "" {
		return nil // Not a supported type for studios
	}

	// No studios to apply
	if len(metadata.Studios) == 0 {
		return nil
	}

	// IMPORTANT: Clear existing studios FIRST before finding/creating studios.
	// The orphan cleanup trigger (tr_media_studios_cleanup_orphan_studios) deletes studios
	// who have no remaining associations. If we find/create studios before clearing,
	// those studio IDs may become invalid after clearing.
	if err := a.studioRepo.ClearStudiosForEntity(entityType, mediaID); err != nil {
		return err
	}

	// Now find or create studios and add new associations
	var count int
	for _, studioName := range metadata.Studios {
		if studioName == "" {
			continue
		}

		studio, err := a.studioRepo.FindOrCreateStudio(studioName, 0) // No TMDb ID in proto yet
		if err != nil {
			a.logger.Warn("failed to find/create studio",
				slog.String("name", studioName),
				slog.Any("error", err))
			continue
		}

		if err := a.studioRepo.AddStudioToEntity(entityType, mediaID, studio.ID); err != nil {
			a.logger.Warn("failed to add studio to entity",
				slog.String("name", studioName),
				slog.Any("error", err))
			continue
		}
		count++
	}

	a.logger.Debug("applied studios",
		slog.Int64("media_id", mediaID),
		slog.String("media_type", string(mediaType)),
		slog.Int("studio_count", count))

	return nil
}

// studioMediaTypeToEntityType converts enrichment.MediaType to studios entity type.
func studioMediaTypeToEntityType(mt enrichment.MediaType) string {
	switch mt {
	case enrichment.MediaTypeMovie:
		return "movie"
	case enrichment.MediaTypeTVShow:
		return "tv_show"
	default:
		return ""
	}
}
