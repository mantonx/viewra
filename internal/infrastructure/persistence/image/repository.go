package image

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements images.Repository using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new image repository with the specified database driver.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// Create inserts a new image record
func (r *Repository) Create(ctx context.Context, image *images.Image) error {
	result, err := r.Q().CreateImage(ctx, buildCreateImageParams(image))
	if err != nil {
		// If this is a UNIQUE constraint error, the image already exists - just skip it
		if common.IsUniqueConstraintError(err) {
			return nil
		}
		return fmt.Errorf("failed to create image: %w", err)
	}

	// Update the domain entity with the generated values
	image.ID = int(result.ID)
	image.CreatedAt = result.CreatedAt.Time
	image.UpdatedAt = result.UpdatedAt.Time

	return nil
}

// GetByID retrieves an image by its ID
func (r *Repository) GetByID(ctx context.Context, id int) (*images.Image, error) {
	row, err := r.Q().GetImageByID(ctx, int64(id))
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return imageToDomain(row), nil
}

// GetByMediaID retrieves all images for a media item
func (r *Repository) GetByMediaID(ctx context.Context, mediaID int) ([]*images.Image, error) {
	rows, err := r.Q().ListImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, imageToDomain), nil
}

// GetByEntity retrieves all images for a specific entity (show, season, album, artist)
func (r *Repository) GetByEntity(ctx context.Context, mediaType images.MediaType, entityID int) ([]*images.Image, error) {
	rows, err := r.Q().ListImagesByEntity(ctx, buildListImagesByEntityParams(mediaType, entityID))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, imageToDomain), nil
}

// GetByTypeAndEntity retrieves a specific image type for an entity
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndEntity(ctx context.Context, mediaType images.MediaType, entityID int, imageType images.ImageType) (*images.Image, error) {
	row, err := r.Q().GetImageByTypeAndEntity(ctx, buildGetImageByTypeAndEntityParams(mediaType, entityID, imageType))
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return imageToDomain(row), nil
}

// GetByTypeAndMediaID retrieves a specific image type for a media item
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndMediaID(ctx context.Context, mediaID int, imageType images.ImageType) (*images.Image, error) {
	row, err := r.Q().GetImageByTypeAndMediaID(ctx, buildGetImageByTypeAndMediaIDParams(mediaID, imageType))
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return imageToDomain(row), nil
}

// Update updates an existing image record
func (r *Repository) Update(ctx context.Context, image *images.Image) error {
	return r.Q().UpdateImage(ctx, buildUpdateImageParams(image))
}

// Delete removes an image record by ID
func (r *Repository) Delete(ctx context.Context, id int) error {
	return r.Q().DeleteImage(ctx, int64(id))
}

// DeleteByMediaID removes all images for a media item
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int) error {
	return r.Q().DeleteImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
}

// DeleteByEntity removes all images for an entity
func (r *Repository) DeleteByEntity(ctx context.Context, mediaType images.MediaType, entityID int) error {
	return r.Q().DeleteImagesByEntity(ctx, buildDeleteImagesByEntityParams(mediaType, entityID))
}

// ListOrphans finds images whose files no longer exist
func (r *Repository) ListOrphans(ctx context.Context) ([]*images.Image, error) {
	rows, err := r.Q().ListOrphanImages(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, imageToDomain), nil
}

// ListBySource retrieves all images from a specific source (for cleanup)
func (r *Repository) ListBySource(ctx context.Context, sourceType images.SourceType) ([]*images.Image, error) {
	rows, err := r.Q().ListImagesBySource(ctx, string(sourceType))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, imageToDomain), nil
}

// GetByFilePath retrieves an image by its file path
func (r *Repository) GetByFilePath(ctx context.Context, filePath string) (*images.Image, error) {
	row, err := r.Q().GetImageByFilePath(ctx, common.NullString(filePath))
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return imageToDomain(row), nil
}

// GetAllFileHashes retrieves all unique file hashes from the database
func (r *Repository) GetAllFileHashes(ctx context.Context) ([]string, error) {
	results, err := r.Q().GetAllFileHashes(ctx)
	if err != nil {
		return nil, err
	}

	// Convert NullString slice to string slice, filtering out nulls
	hashes := make([]string, 0, len(results))
	for _, h := range results {
		if h.Valid {
			hashes = append(hashes, h.String)
		}
	}
	return hashes, nil
}

// GetByHash retrieves all images with a specific file hash
func (r *Repository) GetByHash(ctx context.Context, hash string) ([]*images.Image, error) {
	rows, err := r.Q().GetImagesByHash(ctx, common.NullString(hash))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, imageToDomain), nil
}

// GetByExternalURL retrieves an image by its external URL and media ID
func (r *Repository) GetByExternalURL(ctx context.Context, externalURL string, mediaID int64) (*images.Image, error) {
	row, err := r.Q().GetImageByExternalURL(ctx, unified.GetImageByExternalURLParams{
		ExternalUrl: common.NullString(externalURL),
		MediaID:     common.NullInt64(mediaID),
	})
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return imageToDomain(row), nil
}

// HasImagesForEntity checks if an entity has any images
func (r *Repository) HasImagesForEntity(ctx context.Context, mediaType images.MediaType, entityID int) (bool, error) {
	imgs, err := r.GetByEntity(ctx, mediaType, entityID)
	if err != nil {
		return false, err
	}
	return len(imgs) > 0, nil
}

// DeleteOrphanedEntityImages removes images for entities that no longer exist.
// This handles polymorphic entity_id references (tv_show, tv_season, music_album, music_artist)
// that don't have foreign key constraints. Returns the number of deleted images.
func (r *Repository) DeleteOrphanedEntityImages(ctx context.Context) (int64, error) {
	result, err := r.Q().DeleteOrphanedEntityImages(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountOrphanedEntityImages counts images for entities that no longer exist.
func (r *Repository) CountOrphanedEntityImages(ctx context.Context) (int64, error) {
	return r.Q().CountOrphanedEntityImages(ctx)
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
