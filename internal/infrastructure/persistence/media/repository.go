package media

import (
	"context"
	"database/sql"
	"errors"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new media repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
		db:      db,
		dbType:  driver,
		adapter: adapters.NewTypeAdapter(),
		router:  common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// Create adds a new media item to the database
func (r *Repository) Create(ctx context.Context, m *media.Media) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreateMedia(ctx, sqlc_postgres.CreateMediaParams{
				LibraryID:         int32(m.LibraryID),
				Title:             m.Title,
				FilePath:          m.FilePath,
				FileSize:          common.NullInt64(m.FileSize),
				Duration:          common.NullFloat64(float64(m.Duration)),
				Type:              "movie", // Default for now
				FileHash:          sql.NullString{},
				ContainerFormat:   sql.NullString{},
				Width:             sql.NullInt32{},
				Height:            sql.NullInt32{},
				AspectRatio:       sql.NullString{},
				Codec:             sql.NullString{},
				CodecProfile:      sql.NullString{},
				BitRate:           sql.NullInt64{},
				FrameRate:         sql.NullFloat64{},
				ScanType:          sql.NullString{},
				HdrFormat:         sql.NullString{},
				ColorSpace:        sql.NullString{},
				ColorPrimaries:    sql.NullString{},
				ThumbnailPath:     sql.NullString{},
				SourceType:        sql.NullString{},
				ResolutionLabel:   sql.NullString{},
				QualityScore:      sql.NullInt32{},
				Is3d:              sql.NullBool{},
				StereoMode:        sql.NullString{},
				HasDash:           sql.NullBool{},
				DashManifestPath:  sql.NullString{},
				TranscodingStatus: sql.NullString{},
			})
		},
		func() (any, error) {
			return r.sqlite.CreateMedia(ctx, sqlc_sqlite.CreateMediaParams{
				LibraryID:         m.LibraryID,
				Title:             m.Title,
				FilePath:          m.FilePath,
				FileSize:          common.NullInt64(m.FileSize),
				Duration:          common.NullFloat64(float64(m.Duration)),
				Type:              "movie", // Default for now
				FileHash:          sql.NullString{},
				ContainerFormat:   sql.NullString{},
				Width:             sql.NullInt64{},
				Height:            sql.NullInt64{},
				AspectRatio:       sql.NullString{},
				Codec:             sql.NullString{},
				CodecProfile:      sql.NullString{},
				BitRate:           sql.NullInt64{},
				FrameRate:         sql.NullFloat64{},
				ScanType:          sql.NullString{},
				HdrFormat:         sql.NullString{},
				ColorSpace:        sql.NullString{},
				ColorPrimaries:    sql.NullString{},
				ThumbnailPath:     sql.NullString{},
				SourceType:        sql.NullString{},
				ResolutionLabel:   sql.NullString{},
				QualityScore:      sql.NullInt64{},
				Is3d:              sql.NullBool{},
				StereoMode:        sql.NullString{},
				HasDash:           sql.NullBool{},
				DashManifestPath:  sql.NullString{},
				TranscodingStatus: sql.NullString{},
			})
		},
	)
	if err != nil {
		return err
	}

	// Convert result to domain media
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		m.ID = int64(pgResult.ID)
		m.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		m.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Medium)
		m.ID = sqResult.ID
		m.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		m.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// GetByID retrieves a media item by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMediaByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetMediaByID(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		return &media.Media{
			ID:        int64(pgResult.ID),
			LibraryID: int64(pgResult.LibraryID),
			Title:     pgResult.Title,
			FilePath:  pgResult.FilePath,
			FileSize:  pgResult.FileSize.Int64,
			Duration:  int(pgResult.Duration.Float64),
			CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Medium)
	return &media.Media{
		ID:        sqResult.ID,
		LibraryID: sqResult.LibraryID,
		Title:     sqResult.Title,
		FilePath:  sqResult.FilePath,
		FileSize:  sqResult.FileSize.Int64,
		Duration:  int(sqResult.Duration.Float64),
		CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
	}, nil
}

// GetByFilePath retrieves a media item by its file path within a library
func (r *Repository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetMediaByFilePath(ctx, sqlc_postgres.GetMediaByFilePathParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return r.sqlite.GetMediaByFilePath(ctx, sqlc_sqlite.GetMediaByFilePathParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, media.ErrMediaNotFound
		}
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		return &media.Media{
			ID:        int64(pgResult.ID),
			LibraryID: int64(pgResult.LibraryID),
			Title:     pgResult.Title,
			FilePath:  pgResult.FilePath,
			FileSize:  pgResult.FileSize.Int64,
			Duration:  int(pgResult.Duration.Float64),
			CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Medium)
	return &media.Media{
		ID:        sqResult.ID,
		LibraryID: sqResult.LibraryID,
		Title:     sqResult.Title,
		FilePath:  sqResult.FilePath,
		FileSize:  sqResult.FileSize.Int64,
		Duration:  int(sqResult.Duration.Float64),
		CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
	}, nil
}

// ListByLibrary retrieves all media items in a specific library
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListMediaByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.ListMediaByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = &media.Media{
				ID:        int64(pgResult.ID),
				LibraryID: int64(pgResult.LibraryID),
				Title:     pgResult.Title,
				FilePath:  pgResult.FilePath,
				FileSize:  pgResult.FileSize.Int64,
				Duration:  int(pgResult.Duration.Float64),
				CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = &media.Media{
			ID:        sqResult.ID,
			LibraryID: sqResult.LibraryID,
			Title:     sqResult.Title,
			FilePath:  sqResult.FilePath,
			FileSize:  sqResult.FileSize.Int64,
			Duration:  int(sqResult.Duration.Float64),
			CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
		}
	}
	return mediaList, nil
}

// ListByType retrieves all media items of a specific type in a library
func (r *Repository) ListByType(
	ctx context.Context,
	libraryID int64,
	mediaType media.MediaType,
) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListMediaByType(ctx, sqlc_postgres.ListMediaByTypeParams{
				LibraryID: int32(libraryID),
				Type:      string(mediaType),
			})
		},
		func() (any, error) {
			return r.sqlite.ListMediaByType(ctx, sqlc_sqlite.ListMediaByTypeParams{
				LibraryID: libraryID,
				Type:      string(mediaType),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain media
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Medium)
		mediaList := make([]*media.Media, len(pgResults))
		for i, pgResult := range pgResults {
			mediaList[i] = &media.Media{
				ID:        int64(pgResult.ID),
				LibraryID: int64(pgResult.LibraryID),
				Title:     pgResult.Title,
				FilePath:  pgResult.FilePath,
				FileSize:  pgResult.FileSize.Int64,
				Duration:  int(pgResult.Duration.Float64),
				CreatedAt: common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt: common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = &media.Media{
			ID:        sqResult.ID,
			LibraryID: sqResult.LibraryID,
			Title:     sqResult.Title,
			FilePath:  sqResult.FilePath,
			FileSize:  sqResult.FileSize.Int64,
			Duration:  int(sqResult.Duration.Float64),
			CreatedAt: common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt: common.ParseNullTime(sqResult.UpdatedAt),
		}
	}
	return mediaList, nil
}

// Update modifies an existing media item
func (r *Repository) Update(ctx context.Context, m *media.Media) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.UpdateMedia(ctx, sqlc_postgres.UpdateMediaParams{
				LibraryID:         int32(m.LibraryID),
				Title:             m.Title,
				FilePath:          m.FilePath,
				FileSize:          common.NullInt64(m.FileSize),
				Duration:          common.NullFloat64(float64(m.Duration)),
				Type:              "movie", // Preserve type
				FileHash:          sql.NullString{},
				ContainerFormat:   sql.NullString{},
				Width:             sql.NullInt32{},
				Height:            sql.NullInt32{},
				AspectRatio:       sql.NullString{},
				Codec:             sql.NullString{},
				CodecProfile:      sql.NullString{},
				BitRate:           sql.NullInt64{},
				FrameRate:         sql.NullFloat64{},
				ScanType:          sql.NullString{},
				HdrFormat:         sql.NullString{},
				ColorSpace:        sql.NullString{},
				ColorPrimaries:    sql.NullString{},
				ThumbnailPath:     sql.NullString{},
				SourceType:        sql.NullString{},
				ResolutionLabel:   sql.NullString{},
				QualityScore:      sql.NullInt32{},
				Is3d:              sql.NullBool{},
				StereoMode:        sql.NullString{},
				HasDash:           sql.NullBool{},
				DashManifestPath:  sql.NullString{},
				TranscodingStatus: sql.NullString{},
				ID:                int32(m.ID),
			})
		},
		func() (any, error) {
			return r.sqlite.UpdateMedia(ctx, sqlc_sqlite.UpdateMediaParams{
				LibraryID:         m.LibraryID,
				Title:             m.Title,
				FilePath:          m.FilePath,
				FileSize:          common.NullInt64(m.FileSize),
				Duration:          common.NullFloat64(float64(m.Duration)),
				Type:              "movie", // Preserve type
				FileHash:          sql.NullString{},
				ContainerFormat:   sql.NullString{},
				Width:             sql.NullInt64{},
				Height:            sql.NullInt64{},
				AspectRatio:       sql.NullString{},
				Codec:             sql.NullString{},
				CodecProfile:      sql.NullString{},
				BitRate:           sql.NullInt64{},
				FrameRate:         sql.NullFloat64{},
				ScanType:          sql.NullString{},
				HdrFormat:         sql.NullString{},
				ColorSpace:        sql.NullString{},
				ColorPrimaries:    sql.NullString{},
				ThumbnailPath:     sql.NullString{},
				SourceType:        sql.NullString{},
				ResolutionLabel:   sql.NullString{},
				QualityScore:      sql.NullInt64{},
				Is3d:              sql.NullBool{},
				StereoMode:        sql.NullString{},
				HasDash:           sql.NullBool{},
				DashManifestPath:  sql.NullString{},
				TranscodingStatus: sql.NullString{},
				ID:                m.ID,
			})
		},
	)
	if err != nil {
		return err
	}

	// Update timestamps
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Medium)
		m.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Medium)
		m.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// Delete removes a media item from the repository
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteMedia(ctx, int32(id))
		},
		func() error {
			return r.sqlite.DeleteMedia(ctx, id)
		},
	)
}

// ExistsInLibrary checks if a media item with the given file path exists in the library
func (r *Repository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.MediaExistsInLibrary(ctx, sqlc_postgres.MediaExistsInLibraryParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			count, err := r.sqlite.MediaExistsInLibrary(ctx, sqlc_sqlite.MediaExistsInLibraryParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
			if err != nil {
				return false, err
			}
			return count > 0, nil
		},
	)
	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// Count returns the total number of media items in a library
func (r *Repository) Count(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountMediaInLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.CountMediaInLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// CountByType returns the number of media items of a specific type in a library
func (r *Repository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountMediaByType(ctx, sqlc_postgres.CountMediaByTypeParams{
				LibraryID: int32(libraryID),
				Type:      string(mediaType),
			})
		},
		func() (any, error) {
			return r.sqlite.CountMediaByType(ctx, sqlc_sqlite.CountMediaByTypeParams{
				LibraryID: libraryID,
				Type:      string(mediaType),
			})
		},
	)
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}
