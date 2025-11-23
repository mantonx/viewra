package image

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
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
	result, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MediaImage, error) {
			return r.Postgres().CreateImage(ctx, buildPostgresCreateImageParams(image))
		},
		func() (sqlc_sqlite.MediaImage, error) {
			return r.SQLite().CreateImage(ctx, buildSQLiteCreateImageParams(image))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
	if err != nil {
		// If this is a UNIQUE constraint error, the image already exists - just skip it
		if common.IsUniqueConstraintError(err) {
			return nil
		}
		return fmt.Errorf("failed to create image: %w", err)
	}

	// Update the domain entity with the generated values
	image.ID = result.ID
	image.CreatedAt = result.CreatedAt
	image.UpdatedAt = result.UpdatedAt

	return nil
}

// GetByID retrieves an image by its ID
func (r *Repository) GetByID(ctx context.Context, id int) (*images.Image, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MediaImage, error) {
			return r.Postgres().GetImageByID(ctx, int32(id))
		},
		func() (sqlc_sqlite.MediaImage, error) {
			return r.SQLite().GetImageByID(ctx, int64(id))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetByMediaID retrieves all images for a media item
func (r *Repository) GetByMediaID(ctx context.Context, mediaID int) ([]*images.Image, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MediaImage, error) {
			return r.Postgres().ListImagesByMediaID(ctx, common.NullInt32FromInt64(int64(mediaID)))
		},
		func() ([]sqlc_sqlite.MediaImage, error) {
			return r.SQLite().ListImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetByEntity retrieves all images for a specific entity (show, season, album, artist)
func (r *Repository) GetByEntity(ctx context.Context, mediaType images.MediaType, entityID int) ([]*images.Image, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MediaImage, error) {
			return r.Postgres().ListImagesByEntity(ctx, buildPostgresListImagesByEntityParams(mediaType, entityID))
		},
		func() ([]sqlc_sqlite.MediaImage, error) {
			return r.SQLite().ListImagesByEntity(ctx, buildSQLiteListImagesByEntityParams(mediaType, entityID))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetByTypeAndEntity retrieves a specific image type for an entity
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndEntity(ctx context.Context, mediaType images.MediaType, entityID int, imageType images.ImageType) (*images.Image, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MediaImage, error) {
			return r.Postgres().GetImageByTypeAndEntity(ctx, buildPostgresGetImageByTypeAndEntityParams(mediaType, entityID, imageType))
		},
		func() (sqlc_sqlite.MediaImage, error) {
			return r.SQLite().GetImageByTypeAndEntity(ctx, buildSQLiteGetImageByTypeAndEntityParams(mediaType, entityID, imageType))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetByTypeAndMediaID retrieves a specific image type for a media item
// Returns the highest priority image of that type
func (r *Repository) GetByTypeAndMediaID(ctx context.Context, mediaID int, imageType images.ImageType) (*images.Image, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MediaImage, error) {
			return r.Postgres().GetImageByTypeAndMediaID(ctx, buildPostgresGetImageByTypeAndMediaIDParams(mediaID, imageType))
		},
		func() (sqlc_sqlite.MediaImage, error) {
			return r.SQLite().GetImageByTypeAndMediaID(ctx, buildSQLiteGetImageByTypeAndMediaIDParams(mediaID, imageType))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// Update updates an existing image record
func (r *Repository) Update(ctx context.Context, image *images.Image) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateImage(ctx, buildPostgresUpdateImageParams(image))
		},
		func() error {
			return r.SQLite().UpdateImage(ctx, buildSQLiteUpdateImageParams(image))
		},
	)
}

// Delete removes an image record by ID
func (r *Repository) Delete(ctx context.Context, id int) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteImage(ctx, int32(id))
		},
		func() error {
			return r.SQLite().DeleteImage(ctx, int64(id))
		},
	)
}

// DeleteByMediaID removes all images for a media item
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteImagesByMediaID(ctx, common.NullInt32FromInt64(int64(mediaID)))
		},
		func() error {
			return r.SQLite().DeleteImagesByMediaID(ctx, common.NullInt64(int64(mediaID)))
		},
	)
}

// DeleteByEntity removes all images for an entity
func (r *Repository) DeleteByEntity(ctx context.Context, mediaType images.MediaType, entityID int) error {
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteImagesByEntity(ctx, buildPostgresDeleteImagesByEntityParams(mediaType, entityID))
		},
		func() error {
			return r.SQLite().DeleteImagesByEntity(ctx, buildSQLiteDeleteImagesByEntityParams(mediaType, entityID))
		},
	)
}

// ListOrphans finds images whose files no longer exist
func (r *Repository) ListOrphans(ctx context.Context) ([]*images.Image, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MediaImage, error) {
			return r.Postgres().ListOrphanImages(ctx)
		},
		func() ([]sqlc_sqlite.MediaImage, error) {
			return r.SQLite().ListOrphanImages(ctx)
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// ListBySource retrieves all images from a specific source (for cleanup)
func (r *Repository) ListBySource(ctx context.Context, sourceType images.SourceType) ([]*images.Image, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MediaImage, error) {
			return r.Postgres().ListImagesBySource(ctx, string(sourceType))
		},
		func() ([]sqlc_sqlite.MediaImage, error) {
			return r.SQLite().ListImagesBySource(ctx, string(sourceType))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetByFilePath retrieves an image by its file path
func (r *Repository) GetByFilePath(ctx context.Context, filePath string) (*images.Image, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MediaImage, error) {
			return r.Postgres().GetImageByFilePath(ctx, common.NullString(filePath))
		},
		func() (sqlc_sqlite.MediaImage, error) {
			return r.SQLite().GetImageByFilePath(ctx, common.NullString(filePath))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// GetAllFileHashes retrieves all unique file hashes from the database
func (r *Repository) GetAllFileHashes(ctx context.Context) ([]string, error) {
	results, err := common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sql.NullString, error) {
			return r.Postgres().GetAllFileHashes(ctx)
		},
		func() ([]sql.NullString, error) {
			return r.SQLite().GetAllFileHashes(ctx)
		},
		func(row sql.NullString) sql.NullString { return row },
		func(row sql.NullString) sql.NullString { return row },
	)
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
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MediaImage, error) {
			return r.Postgres().GetImagesByHash(ctx, common.NullString(hash))
		},
		func() ([]sqlc_sqlite.MediaImage, error) {
			return r.SQLite().GetImagesByHash(ctx, common.NullString(hash))
		},
		postgresImageToDomain,
		sqliteImageToDomain,
	)
}

// HasImagesForEntity checks if an entity has any images
func (r *Repository) HasImagesForEntity(ctx context.Context, mediaType images.MediaType, entityID int) (bool, error) {
	imgs, err := r.GetByEntity(ctx, mediaType, entityID)
	if err != nil {
		return false, err
	}
	return len(imgs) > 0, nil
}
