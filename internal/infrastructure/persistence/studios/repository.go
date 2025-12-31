package studios

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.StudioRepository.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new studios repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// GetStudioByID retrieves a studio by its ID.
func (r *Repository) GetStudioByID(id int64) (*media.Studio, error) {
	ctx := context.Background()
	row, err := r.Q().GetStudioByID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return studioToDomain(row), nil
}

// GetStudioByName retrieves a studio by its name.
func (r *Repository) GetStudioByName(name string) (*media.Studio, error) {
	ctx := context.Background()
	row, err := r.Q().GetStudioByName(ctx, name)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return studioToDomain(row), nil
}

// GetStudioByTMDbID retrieves a studio by its TMDb ID.
func (r *Repository) GetStudioByTMDbID(tmdbID int) (*media.Studio, error) {
	ctx := context.Background()
	row, err := r.Q().GetStudioByTMDbID(ctx, sql.NullInt64{Int64: int64(tmdbID), Valid: true})
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return studioToDomain(row), nil
}

// CreateStudio creates a new studio.
func (r *Repository) CreateStudio(studio *media.Studio) error {
	ctx := context.Background()
	result, err := r.Q().CreateStudio(ctx, buildCreateStudioParams(studio))
	if err != nil {
		return err
	}
	studio.ID = result.ID
	return nil
}

// UpdateStudio updates an existing studio.
func (r *Repository) UpdateStudio(studio *media.Studio) error {
	ctx := context.Background()
	return r.Q().UpdateStudio(ctx, buildUpdateStudioParams(studio))
}

// isNotFoundError checks if the error indicates a record was not found.
func isNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, media.ErrMediaNotFound)
}

// FindOrCreateStudio finds a studio by name or TMDb ID, or creates it if not found.
func (r *Repository) FindOrCreateStudio(name string, tmdbID int) (*media.Studio, error) {
	// Try to find by TMDb ID first
	if tmdbID > 0 {
		studio, err := r.GetStudioByTMDbID(tmdbID)
		if err == nil {
			return studio, nil
		}
		if !isNotFoundError(err) {
			return nil, err
		}
	}

	// Try to find by name
	studio, err := r.GetStudioByName(name)
	if err == nil {
		// Update TMDb ID if we have one and it wasn't set
		if tmdbID > 0 && studio.TMDbID == 0 {
			studio.TMDbID = tmdbID
			_ = r.UpdateStudio(studio) // Ignore error, just try to update
		}
		return studio, nil
	}
	if !isNotFoundError(err) {
		return nil, err
	}

	// Create new studio
	newStudio := &media.Studio{
		Name:   name,
		TMDbID: tmdbID,
	}
	if err := r.CreateStudio(newStudio); err != nil {
		return nil, err
	}
	return newStudio, nil
}

// GetStudiosForEntity retrieves all studios for a media entity.
func (r *Repository) GetStudiosForEntity(mediaType string, entityID int64) ([]*media.Studio, error) {
	ctx := context.Background()
	rows, err := r.Q().GetStudiosForEntity(ctx, unified.GetStudiosForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, studioToDomain), nil
}

// AddStudioToEntity adds a studio association to a media entity.
func (r *Repository) AddStudioToEntity(mediaType string, entityID int64, studioID int64) error {
	ctx := context.Background()
	return r.Q().AddMediaStudio(ctx, unified.AddMediaStudioParams{
		MediaType: mediaType,
		EntityID:  entityID,
		StudioID:  studioID,
	})
}

// RemoveStudioFromEntity removes a studio association from a media entity.
func (r *Repository) RemoveStudioFromEntity(mediaType string, entityID int64, studioID int64) error {
	ctx := context.Background()
	return r.Q().RemoveMediaStudio(ctx, unified.RemoveMediaStudioParams{
		MediaType: mediaType,
		EntityID:  entityID,
		StudioID:  studioID,
	})
}

// ClearStudiosForEntity removes all studio associations for a media entity.
func (r *Repository) ClearStudiosForEntity(mediaType string, entityID int64) error {
	ctx := context.Background()
	return r.Q().ClearStudiosForEntity(ctx, unified.ClearStudiosForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
}

// ReplaceStudiosForEntity clears existing studios and adds new ones.
func (r *Repository) ReplaceStudiosForEntity(mediaType string, entityID int64, studios []*media.Studio) error {
	// Clear existing studios
	if err := r.ClearStudiosForEntity(mediaType, entityID); err != nil {
		return err
	}

	// Add new studios
	for _, studio := range studios {
		if err := r.AddStudioToEntity(mediaType, entityID, studio.ID); err != nil {
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
