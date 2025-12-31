package media

import (
	"context"
	"database/sql"
	"errors"

	domaincommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new media repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// Create adds a new media item to the database
func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	result, err := r.Q().CreateMedia(ctx, buildCreateParams(m))
	if err != nil {
		return err
	}
	m.ID = result.ID
	m.CreatedAt = common.ParseNullTime(result.CreatedAt)
	m.UpdatedAt = common.ParseNullTime(result.UpdatedAt)
	return nil
}

// GetByID retrieves a media item by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	row, err := r.Q().GetMediaByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}
	return mediumToDomain(row), nil
}

// GetByFilePath retrieves a media item by its file path within a library
func (r *Repository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	row, err := r.Q().GetMediaByFilePath(ctx, unified.GetMediaByFilePathParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}
	return mediumToDomain(row), nil
}

// ListAll retrieves all media items across all libraries
func (r *Repository) ListAll(ctx context.Context) ([]*media.Media, error) {
	rows, err := r.Q().ListAllMedia(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mediumToDomain), nil
}

// ListByLibrary retrieves all media items in a specific library
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	rows, err := r.Q().ListMediaByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mediumToDomain), nil
}

// GetFilePathCache retrieves a map of file_path -> id for all media in a library.
func (r *Repository) GetFilePathCache(ctx context.Context, libraryID int64) (map[string]int64, error) {
	count, _ := r.Q().CountMediaInLibrary(ctx, libraryID)

	rows, err := r.Q().GetFilePathCache(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]int64, count)
	for _, row := range rows {
		cache[row.FilePath] = row.ID
	}
	return cache, nil
}

// ListByType retrieves all media items of a specific type in a library
func (r *Repository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	rows, err := r.Q().ListMediaByType(ctx, unified.ListMediaByTypeParams{
		LibraryID: libraryID,
		Type:      string(mediaType),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mediumToDomain), nil
}

// Update modifies an existing media item
func (r *Repository) Update(ctx context.Context, m *media.Media) error {
	result, err := r.Q().UpdateMedia(ctx, buildUpdateParams(m))
	if err != nil {
		return err
	}
	m.UpdatedAt = common.ParseNullTime(result.UpdatedAt)
	return nil
}

// Delete removes a media item from the repository
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteMedia(ctx, id)
}

// ExistsInLibrary checks if a media item with the given file path exists in the library
func (r *Repository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	count, err := r.Q().MediaExistsInLibrary(ctx, unified.MediaExistsInLibraryParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Count returns the total number of media items in a library
func (r *Repository) Count(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountMediaInLibrary(ctx, libraryID)
}

// CountByType returns the number of media items of a specific type in a library
func (r *Repository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	return r.Q().CountMediaByType(ctx, unified.CountMediaByTypeParams{
		LibraryID: libraryID,
		Type:      string(mediaType),
	})
}

// DeleteWithTx removes a media item from the repository within a transaction
func (r *Repository) DeleteWithTx(ctx context.Context, tx domaincommon.Transaction, id int64) error {
	sqlTx := tx.Unwrap().(*sql.Tx)
	return r.QWithTx(sqlTx).DeleteMedia(ctx, id)
}

// ListByLibraryWithTx retrieves all media items in a specific library within a transaction
func (r *Repository) ListByLibraryWithTx(ctx context.Context, tx domaincommon.Transaction, libraryID int64) ([]*media.Media, error) {
	sqlTx := tx.Unwrap().(*sql.Tx)
	rows, err := r.QWithTx(sqlTx).ListMediaByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, mediumToDomain), nil
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
