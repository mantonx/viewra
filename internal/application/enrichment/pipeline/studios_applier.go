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

	// Find or create studios
	var studios []*media.Studio
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
		studios = append(studios, studio)
	}

	// No studios to apply after filtering
	if len(studios) == 0 {
		return nil
	}

	// Replace existing studios with new ones
	if err := a.studioRepo.ReplaceStudiosForEntity(entityType, mediaID, studios); err != nil {
		return err
	}

	a.logger.Debug("applied studios",
		slog.Int64("media_id", mediaID),
		slog.String("media_type", string(mediaType)),
		slog.Int("studio_count", len(studios)))

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
