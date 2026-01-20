package pipeline

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
)

// SimilarTitlesApplier handles applying similar titles from enrichment to the database.
type SimilarTitlesApplier struct {
	similarTitlesRepo media.SimilarTitlesRepository
	logger            *slog.Logger
}

// NewSimilarTitlesApplier creates a new SimilarTitlesApplier.
func NewSimilarTitlesApplier(similarTitlesRepo media.SimilarTitlesRepository, logger *slog.Logger) *SimilarTitlesApplier {
	return &SimilarTitlesApplier{
		similarTitlesRepo: similarTitlesRepo,
		logger:            logger,
	}
}

// Apply applies similar titles from enriched metadata to the database.
func (a *SimilarTitlesApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error {
	if a.similarTitlesRepo == nil {
		return nil // No similar titles repo configured
	}

	// Only movies and TV shows have similar titles
	entityType := similarTitlesMediaTypeToEntityType(mediaType)
	if entityType == "" {
		return nil // Not a supported type for similar titles
	}

	// No similar titles to apply
	if len(metadata.SimilarTitles) == 0 {
		return nil
	}

	// Replace all similar titles for this entity
	if err := a.similarTitlesRepo.ReplaceSimilarTitles(entityType, mediaID, metadata.SimilarTitles); err != nil {
		a.logger.Warn("failed to apply similar titles",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)),
			slog.Any("error", err))
		return err
	}

	a.logger.Debug("applied similar titles",
		slog.Int64("media_id", mediaID),
		slog.String("media_type", string(mediaType)),
		slog.Int("similar_titles_count", len(metadata.SimilarTitles)))

	return nil
}

// similarTitlesMediaTypeToEntityType converts enrichment.MediaType to similar titles entity type.
func similarTitlesMediaTypeToEntityType(mt enrichment.MediaType) string {
	switch mt {
	case enrichment.MediaTypeMovie:
		return "movie"
	case enrichment.MediaTypeTVShow:
		return "tv_show"
	default:
		return ""
	}
}
