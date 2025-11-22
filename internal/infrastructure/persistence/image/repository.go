package image

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
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
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CreateImage(ctx, buildSQLiteCreateImageParams(image))
		},
	)
	if err != nil {
		// If this is a UNIQUE constraint error, the image already exists - just skip it
		if common.IsUniqueConstraintError(err) {
			return nil
		}
		return fmt.Errorf("failed to create image: %w", err)
	}

	// Update the domain entity with the generated ID
	if r.Router().IsPostgresDB() {
		return r.PostgresNotImplemented()
	}

	dbImage := result.(sqlc_sqlite.MediaImage)
	image.ID = int(dbImage.ID)
	image.CreatedAt = dbImage.CreatedAt.Time
	image.UpdatedAt = dbImage.UpdatedAt.Time

	return nil
}

// GetByID retrieves an image by its ID
func (r *Repository) GetByID(ctx context.Context, id int) (*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetImageByID(ctx, int64(id))
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain image
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteImageToDomain(result.(sqlc_sqlite.MediaImage)), nil
}

// GetByMediaID retrieves all images for a media item
func (r *Repository) GetByMediaID(ctx context.Context, mediaID int) ([]*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain images
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbImages := result.([]sqlc_sqlite.MediaImage)
	domainImages := make([]*images.Image, len(dbImages))
	for i, dbImage := range dbImages {
		domainImages[i] = sqliteImageToDomain(dbImage)
	}
	return domainImages, nil
}

// GetByEntity retrieves all images for a specific entity (show, season, album, artist)
func (r *Repository) GetByEntity(ctx context.Context, mediaType images.MediaType, entityID int) ([]*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListImagesByEntity(ctx, sqlc_sqlite.ListImagesByEntityParams{
				MediaType: string(mediaType),
				EntityID:  int64(entityID),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain images
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbImages := result.([]sqlc_sqlite.MediaImage)
	domainImages := make([]*images.Image, len(dbImages))
	for i, dbImage := range dbImages {
		domainImages[i] = sqliteImageToDomain(dbImage)
	}
	return domainImages, nil
}

// GetByTypeAndEntity retrieves a specific image type for an entity
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndEntity(ctx context.Context, mediaType images.MediaType, entityID int, imageType images.ImageType) (*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetImageByTypeAndEntity(ctx, sqlc_sqlite.GetImageByTypeAndEntityParams{
				MediaType: string(mediaType),
				EntityID:  int64(entityID),
				ImageType: string(imageType),
			})
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain image
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteImageToDomain(result.(sqlc_sqlite.MediaImage)), nil
}

// GetByTypeAndMediaID retrieves a specific image type for a media item
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndMediaID(ctx context.Context, mediaID int, imageType images.ImageType) (*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetImageByTypeAndMediaID(ctx, sqlc_sqlite.GetImageByTypeAndMediaIDParams{
				MediaID:   common.NullInt64(int64(mediaID)),
				ImageType: string(imageType),
			})
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain image
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteImageToDomain(result.(sqlc_sqlite.MediaImage)), nil
}

// Update updates an existing image record
func (r *Repository) Update(ctx context.Context, image *images.Image) error {
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().UpdateImage(ctx, buildSQLiteUpdateImageParams(image))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	return nil
}

// Delete removes an image record by ID
func (r *Repository) Delete(ctx context.Context, id int) error {
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().DeleteImage(ctx, int64(id))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	return nil
}

// DeleteByMediaID removes all images for a media item
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int) error {
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().DeleteImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete images by media ID: %w", err)
	}

	return nil
}

// DeleteByEntity removes all images for an entity
func (r *Repository) DeleteByEntity(ctx context.Context, mediaType images.MediaType, entityID int) error {
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().DeleteImagesByEntity(ctx, sqlc_sqlite.DeleteImagesByEntityParams{
				MediaType: string(mediaType),
				EntityID:  int64(entityID),
			})
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete images by entity: %w", err)
	}

	return nil
}

// ListOrphans finds images whose files no longer exist
func (r *Repository) ListOrphans(ctx context.Context) ([]*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListOrphanImages(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain images
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbImages := result.([]sqlc_sqlite.MediaImage)
	domainImages := make([]*images.Image, len(dbImages))
	for i, dbImage := range dbImages {
		domainImages[i] = sqliteImageToDomain(dbImage)
	}
	return domainImages, nil
}

// ListBySource retrieves all images from a specific source (for cleanup)
func (r *Repository) ListBySource(ctx context.Context, sourceType images.SourceType) ([]*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListImagesBySource(ctx, string(sourceType))
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain images
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbImages := result.([]sqlc_sqlite.MediaImage)
	domainImages := make([]*images.Image, len(dbImages))
	for i, dbImage := range dbImages {
		domainImages[i] = sqliteImageToDomain(dbImage)
	}
	return domainImages, nil
}

// sqliteImageToDomain converts a SQLite database image to a domain image
func sqliteImageToDomain(dbImage sqlc_sqlite.MediaImage) *images.Image {
	var mediaID *int
	if dbImage.MediaID.Valid {
		id := int(dbImage.MediaID.Int64)
		mediaID = &id
	}

	var width *int
	if dbImage.Width.Valid {
		w := int(dbImage.Width.Int64)
		width = &w
	}

	var height *int
	if dbImage.Height.Valid {
		h := int(dbImage.Height.Int64)
		height = &h
	}

	var fileSizeBytes *int64
	if dbImage.FileSizeBytes.Valid {
		fileSizeBytes = &dbImage.FileSizeBytes.Int64
	}

	var mimeType *string
	if dbImage.MimeType.Valid {
		mimeType = &dbImage.MimeType.String
	}

	var fileHash *string
	if dbImage.FileHash.Valid {
		fileHash = &dbImage.FileHash.String
	}

	var language *string
	if dbImage.Language.Valid {
		language = &dbImage.Language.String
	}

	var externalURL *string
	if dbImage.ExternalUrl.Valid {
		externalURL = &dbImage.ExternalUrl.String
	}

	var localCachePath *string
	if dbImage.LocalCachePath.Valid {
		localCachePath = &dbImage.LocalCachePath.String
	}

	priority := 0
	if dbImage.Priority.Valid {
		priority = int(dbImage.Priority.Int64)
	}

	filePath := ""
	if dbImage.FilePath.Valid {
		filePath = dbImage.FilePath.String
	}

	return &images.Image{
		ID:             int(dbImage.ID),
		MediaID:        mediaID,
		MediaType:      images.MediaType(dbImage.MediaType),
		EntityID:       int(dbImage.EntityID),
		ImageType:      images.ImageType(dbImage.ImageType),
		SourceType:     images.SourceType(dbImage.SourceType),
		FilePath:       filePath,
		ExternalURL:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       priority,
		CreatedAt:      dbImage.CreatedAt.Time,
		UpdatedAt:      dbImage.UpdatedAt.Time,
	}
}

// buildSQLiteCreateImageParams converts a domain image to SQLite create params
func buildSQLiteCreateImageParams(image *images.Image) sqlc_sqlite.CreateImageParams {
	var mediaID sql.NullInt64
	if image.MediaID != nil {
		mediaID = sql.NullInt64{Int64: int64(*image.MediaID), Valid: true}
	}

	var width sql.NullInt64
	if image.Width != nil {
		width = sql.NullInt64{Int64: int64(*image.Width), Valid: true}
	}

	var height sql.NullInt64
	if image.Height != nil {
		height = sql.NullInt64{Int64: int64(*image.Height), Valid: true}
	}

	var fileSizeBytes sql.NullInt64
	if image.FileSizeBytes != nil {
		fileSizeBytes = sql.NullInt64{Int64: *image.FileSizeBytes, Valid: true}
	}

	var mimeType sql.NullString
	if image.MimeType != nil {
		mimeType = sql.NullString{String: *image.MimeType, Valid: true}
	}

	var fileHash sql.NullString
	if image.FileHash != nil {
		fileHash = sql.NullString{String: *image.FileHash, Valid: true}
	}

	var language sql.NullString
	if image.Language != nil {
		language = sql.NullString{String: *image.Language, Valid: true}
	}

	var externalURL sql.NullString
	if image.ExternalURL != nil {
		externalURL = sql.NullString{String: *image.ExternalURL, Valid: true}
	}

	var localCachePath sql.NullString
	if image.LocalCachePath != nil {
		localCachePath = sql.NullString{String: *image.LocalCachePath, Valid: true}
	}

	filePath := sql.NullString{}
	if image.FilePath != "" {
		filePath = sql.NullString{String: image.FilePath, Valid: true}
	}

	return sqlc_sqlite.CreateImageParams{
		MediaID:        mediaID,
		MediaType:      string(image.MediaType),
		EntityID:       int64(image.EntityID),
		ImageType:      string(image.ImageType),
		SourceType:     string(image.SourceType),
		FilePath:       filePath,
		ExternalUrl:    externalURL,
		LocalCachePath: localCachePath,
		Width:          width,
		Height:         height,
		FileSizeBytes:  fileSizeBytes,
		MimeType:       mimeType,
		FileHash:       fileHash,
		Language:       language,
		Priority:       sql.NullInt64{Int64: int64(image.Priority), Valid: true},
	}
}

// buildSQLiteUpdateImageParams converts a domain image to SQLite update params
func buildSQLiteUpdateImageParams(image *images.Image) sqlc_sqlite.UpdateImageParams {
	createParams := buildSQLiteCreateImageParams(image)

	return sqlc_sqlite.UpdateImageParams{
		MediaID:        createParams.MediaID,
		MediaType:      createParams.MediaType,
		EntityID:       createParams.EntityID,
		ImageType:      createParams.ImageType,
		SourceType:     createParams.SourceType,
		FilePath:       createParams.FilePath,
		ExternalUrl:    createParams.ExternalUrl,
		LocalCachePath: createParams.LocalCachePath,
		Width:          createParams.Width,
		Height:         createParams.Height,
		FileSizeBytes:  createParams.FileSizeBytes,
		MimeType:       createParams.MimeType,
		FileHash:       createParams.FileHash,
		Language:       createParams.Language,
		Priority:       createParams.Priority,
		ID:             int64(image.ID),
	}
}

// GetByFilePath retrieves an image by its file path
func (r *Repository) GetByFilePath(ctx context.Context, filePath string) (*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetImageByFilePath(ctx, common.NullString(filePath))
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain image
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteImageToDomain(result.(sqlc_sqlite.MediaImage)), nil
}

// GetAllFileHashes retrieves all unique file hashes from the database
func (r *Repository) GetAllFileHashes(ctx context.Context) ([]string, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetAllFileHashes(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to string slice
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbHashes := result.([]sql.NullString)
	hashes := make([]string, 0, len(dbHashes))
	for _, h := range dbHashes {
		if h.Valid {
			hashes = append(hashes, h.String)
		}
	}
	return hashes, nil
}

// GetByHash retrieves all images with a specific file hash
func (r *Repository) GetByHash(ctx context.Context, hash string) ([]*images.Image, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetImagesByHash(ctx, common.NullString(hash))
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	dbImages := result.([]sqlc_sqlite.MediaImage)
	domainImages := make([]*images.Image, len(dbImages))
	for i, dbImg := range dbImages {
		domainImages[i] = sqliteImageToDomain(dbImg)
	}

	return domainImages, nil
}

// HasImagesForEntity checks if an entity has any images
func (r *Repository) HasImagesForEntity(ctx context.Context, mediaType images.MediaType, entityID int) (bool, error) {
	imgs, err := r.GetByEntity(ctx, mediaType, entityID)
	if err != nil {
		return false, err
	}
	return len(imgs) > 0, nil
}
