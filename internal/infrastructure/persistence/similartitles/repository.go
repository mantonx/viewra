package similartitles

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.SimilarTitlesRepository.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new similar titles repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// GetSimilarTitles retrieves all similar titles for a media entity.
func (r *Repository) GetSimilarTitles(mediaType string, entityID int64) ([]string, error) {
	ctx := context.Background()
	rows, err := r.Q().GetSimilarTitlesByEntity(ctx, unified.GetSimilarTitlesByEntityParams{
		MediaID:   entityID,
		MediaType: mediaType,
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ReplaceSimilarTitles clears existing similar titles and adds new ones.
func (r *Repository) ReplaceSimilarTitles(mediaType string, entityID int64, titles []string) error {
	// Clear existing similar titles
	if err := r.ClearSimilarTitles(mediaType, entityID); err != nil {
		return err
	}

	// Add new similar titles with position
	ctx := context.Background()
	for i, title := range titles {
		if err := r.Q().InsertSimilarTitle(ctx, unified.InsertSimilarTitleParams{
			MediaID:      entityID,
			MediaType:    mediaType,
			SimilarTitle: title,
			Position:     int64(i),
		}); err != nil {
			return err
		}
	}

	return nil
}

// ClearSimilarTitles removes all similar titles for a media entity.
func (r *Repository) ClearSimilarTitles(mediaType string, entityID int64) error {
	ctx := context.Background()
	return r.Q().DeleteSimilarTitlesByEntity(ctx, unified.DeleteSimilarTitlesByEntityParams{
		MediaID:   entityID,
		MediaType: mediaType,
	})
}

// Ensure Repository implements media.SimilarTitlesRepository
var _ media.SimilarTitlesRepository = (*Repository)(nil)
