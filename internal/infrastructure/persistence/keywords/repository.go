package keywords

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.KeywordRepository.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new keywords repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// UpsertKeyword adds or updates a keyword for an entity.
func (r *Repository) UpsertKeyword(mediaType string, entityID int64, keyword *media.Keyword) error {
	ctx := context.Background()
	return r.Q().InsertKeyword(ctx, unified.InsertKeywordParams{
		MediaType:  mediaType,
		EntityID:   entityID,
		KeywordID:  int64(keyword.KeywordID),
		Keyword:    keyword.Name,
		IsLocation: common.NullInt64FromBool(keyword.IsLocation),
	})
}

// GetKeywordsForEntity retrieves all keywords for a media entity.
func (r *Repository) GetKeywordsForEntity(mediaType string, entityID int64) ([]*media.Keyword, error) {
	ctx := context.Background()
	rows, err := r.Q().GetKeywordsByEntity(ctx, unified.GetKeywordsByEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, keywordRowToDomain), nil
}

// GetLocationKeywordsForEntity retrieves only location-related keywords.
func (r *Repository) GetLocationKeywordsForEntity(mediaType string, entityID int64) ([]*media.Keyword, error) {
	ctx := context.Background()
	rows, err := r.Q().GetLocationKeywordsByEntity(ctx, unified.GetLocationKeywordsByEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, locationKeywordRowToDomain), nil
}

// ClearKeywordsForEntity removes all keywords for a media entity.
func (r *Repository) ClearKeywordsForEntity(mediaType string, entityID int64) error {
	ctx := context.Background()
	return r.Q().DeleteKeywordsByEntity(ctx, unified.DeleteKeywordsByEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
}

// ReplaceKeywordsForEntity clears existing keywords and adds new ones.
func (r *Repository) ReplaceKeywordsForEntity(mediaType string, entityID int64, keywords []*media.Keyword) error {
	// Clear existing keywords
	if err := r.ClearKeywordsForEntity(mediaType, entityID); err != nil {
		return err
	}

	// Add new keywords
	for _, keyword := range keywords {
		if err := r.UpsertKeyword(mediaType, entityID, keyword); err != nil {
			return err
		}
	}

	return nil
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}

// Ensure Repository implements media.KeywordRepository
var _ media.KeywordRepository = (*Repository)(nil)
