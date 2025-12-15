package images

import "context"

// Repository defines the interface for image persistence operations
type Repository interface {
	// Create inserts a new image record
	Create(ctx context.Context, image *Image) error

	// GetByID retrieves an image by its ID
	GetByID(ctx context.Context, id int) (*Image, error)

	// GetByMediaID retrieves all images for a media item
	GetByMediaID(ctx context.Context, mediaID int) ([]*Image, error)

	// GetByEntity retrieves all images for a specific entity (show, season, album, artist)
	GetByEntity(ctx context.Context, mediaType MediaType, entityID int) ([]*Image, error)

	// GetByTypeAndEntity retrieves a specific image type for an entity
	// Returns the highest priority image of that type
	GetByTypeAndEntity(ctx context.Context, mediaType MediaType, entityID int, imageType ImageType) (*Image, error)

	// GetByTypeAndMediaID retrieves a specific image type for a media item
	// Returns the highest priority image of that type
	GetByTypeAndMediaID(ctx context.Context, mediaID int, imageType ImageType) (*Image, error)

	// Update updates an existing image record
	Update(ctx context.Context, image *Image) error

	// Delete removes an image record by ID
	Delete(ctx context.Context, id int) error

	// DeleteByMediaID removes all images for a media item
	DeleteByMediaID(ctx context.Context, mediaID int) error

	// DeleteByEntity removes all images for an entity
	DeleteByEntity(ctx context.Context, mediaType MediaType, entityID int) error

	// ListOrphans finds images whose files no longer exist
	ListOrphans(ctx context.Context) ([]*Image, error)

	// ListBySource retrieves all images from a specific source (for cleanup)
	ListBySource(ctx context.Context, sourceType SourceType) ([]*Image, error)

	// GetByFilePath retrieves an image by its file path (for update detection)
	GetByFilePath(ctx context.Context, filePath string) (*Image, error)

	// GetAllFileHashes retrieves all unique file hashes (for orphan cache cleanup)
	GetAllFileHashes(ctx context.Context) ([]string, error)

	// GetByHash retrieves all images with a specific file hash (for deduplication checks)
	GetByHash(ctx context.Context, hash string) ([]*Image, error)

	// GetByExternalURL retrieves an image by its external URL and media ID (for remote dedup)
	GetByExternalURL(ctx context.Context, externalURL string, mediaID int64) (*Image, error)

	// HasImagesForEntity checks if an entity has any images
	HasImagesForEntity(ctx context.Context, mediaType MediaType, entityID int) (bool, error)

	// DeleteOrphanedEntityImages removes images for entities that no longer exist.
	// This handles polymorphic entity_id references (tv_show, tv_season, music_album, music_artist)
	// that don't have foreign key constraints. Returns the number of deleted images.
	DeleteOrphanedEntityImages(ctx context.Context) (int64, error)

	// CountOrphanedEntityImages counts images for entities that no longer exist.
	CountOrphanedEntityImages(ctx context.Context) (int64, error)
}
