package pipeline

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
)

// KeywordsApplier handles applying keywords from enrichment to the database.
type KeywordsApplier struct {
	keywordRepo media.KeywordRepository
	logger      *slog.Logger
}

// NewKeywordsApplier creates a new KeywordsApplier.
func NewKeywordsApplier(keywordRepo media.KeywordRepository, logger *slog.Logger) *KeywordsApplier {
	return &KeywordsApplier{
		keywordRepo: keywordRepo,
		logger:      logger,
	}
}

// Apply applies keywords from enriched metadata to the database.
func (a *KeywordsApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error {
	if a.keywordRepo == nil {
		return nil // No keyword repo configured
	}

	// Only movies and TV shows have keywords
	entityType := keywordMediaTypeToEntityType(mediaType)
	if entityType == "" {
		return nil // Not a supported type for keywords
	}

	// No keywords to apply
	if len(metadata.Keywords) == 0 {
		return nil
	}

	// Convert proto keywords to domain keywords
	keywords := make([]*media.Keyword, 0, len(metadata.Keywords))
	for _, kw := range metadata.Keywords {
		keywords = append(keywords, &media.Keyword{
			KeywordID:  int(kw.Id),
			Name:       kw.Name,
			IsLocation: kw.IsLocation,
		})
	}

	// Replace all keywords for this entity
	if err := a.keywordRepo.ReplaceKeywordsForEntity(entityType, mediaID, keywords); err != nil {
		a.logger.Warn("failed to apply keywords",
			slog.Int64("media_id", mediaID),
			slog.String("media_type", string(mediaType)),
			slog.Any("error", err))
		return err
	}

	// Count location keywords for logging
	var locationCount int
	for _, kw := range keywords {
		if kw.IsLocation {
			locationCount++
		}
	}

	a.logger.Debug("applied keywords",
		slog.Int64("media_id", mediaID),
		slog.String("media_type", string(mediaType)),
		slog.Int("keyword_count", len(keywords)),
		slog.Int("location_count", locationCount))

	return nil
}

// keywordMediaTypeToEntityType converts enrichment.MediaType to keywords entity type.
func keywordMediaTypeToEntityType(mt enrichment.MediaType) string {
	switch mt {
	case enrichment.MediaTypeMovie:
		return "movie"
	case enrichment.MediaTypeTVShow:
		return "tv_show"
	default:
		return ""
	}
}
