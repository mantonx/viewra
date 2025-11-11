package media

import (
	"context"
	"database/sql"
	"errors"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.Repository using sqlc-generated queries
type Repository struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewRepository creates a new media repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Create adds a new media item to the database
func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	result, err := r.queries.CreateMedia(ctx, sqlc.CreateMediaParams{
		LibraryID: m.LibraryID,
		Title:     m.Title,
		FilePath:  m.FilePath,
		FileSize:  common.NullInt64(m.FileSize),
		Duration:  common.NullFloat64(float64(m.Duration)),
		Type:      string(media.MediaTypeMovie), // Default for now
	})
	if err != nil {
		return err
	}

	// Update the media with generated values
	m.ID = result.ID
	m.CreatedAt = common.ParseNullTime(result.CreatedAt)
	m.UpdatedAt = common.ParseNullTime(result.UpdatedAt)

	return nil
}

// GetByID retrieves a media item by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	result, err := r.queries.GetMediaByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	return toMedia(result), nil
}

// GetByFilePath retrieves a media item by its file path within a library
func (r *Repository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	result, err := r.queries.GetMediaByFilePath(ctx, sqlc.GetMediaByFilePathParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	return toMedia(result), nil
}

// ListByLibrary retrieves all media items in a specific library
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	results, err := r.queries.ListMediaByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	mediaList := make([]*media.Media, len(results))
	for i, result := range results {
		mediaList[i] = toMedia(result)
	}

	return mediaList, nil
}

// ListByType retrieves all media items of a specific type in a library
func (r *Repository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	results, err := r.queries.ListMediaByType(ctx, sqlc.ListMediaByTypeParams{
		LibraryID: libraryID,
		Type:      string(mediaType),
	})
	if err != nil {
		return nil, err
	}

	mediaList := make([]*media.Media, len(results))
	for i, result := range results {
		mediaList[i] = toMedia(result)
	}

	return mediaList, nil
}

// Update modifies an existing media item
func (r *Repository) Update(ctx context.Context, m *media.Media) error {
	result, err := r.queries.UpdateMedia(ctx, sqlc.UpdateMediaParams{
		LibraryID: m.LibraryID,
		Title:     m.Title,
		FilePath:  m.FilePath,
		FileSize:  common.NullInt64(m.FileSize),
		Duration:  common.NullFloat64(float64(m.Duration)),
		Type:      string(media.MediaTypeMovie), // Preserve type
		ID:        m.ID,
	})
	if err != nil {
		return err
	}

	// Update timestamps
	m.UpdatedAt = common.ParseNullTime(result.UpdatedAt)

	return nil
}

// Delete removes a media item from the repository
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteMedia(ctx, id)
}

// ExistsInLibrary checks if a media item with the given file path exists in the library
func (r *Repository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	count, err := r.queries.MediaExistsInLibrary(ctx, sqlc.MediaExistsInLibraryParams{
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
	return r.queries.CountMediaInLibrary(ctx, libraryID)
}

// CountByType returns the number of media items of a specific type in a library
func (r *Repository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	return r.queries.CountMediaByType(ctx, sqlc.CountMediaByTypeParams{
		LibraryID: libraryID,
		Type:      string(mediaType),
	})
}

// toMedia converts sqlc Medium to domain Media
func toMedia(m sqlc.Medium) *media.Media {
	return &media.Media{
		ID:        m.ID,
		LibraryID: m.LibraryID,
		Title:     m.Title,
		FilePath:  m.FilePath,
		FileSize:  m.FileSize.Int64,
		Duration:  int(m.Duration.Float64),
		CreatedAt: common.ParseNullTime(m.CreatedAt),
		UpdatedAt: common.ParseNullTime(m.UpdatedAt),
	}
}
