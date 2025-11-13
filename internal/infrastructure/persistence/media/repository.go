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
				Type:              m.Type,
				IsExtra:           m.IsExtra,
				FileHash:          sql.NullString{}, // TODO: Add Hash field to domain.Media
				ContainerFormat:   common.NullString(m.ContainerFormat),
				Width:             common.NullInt32(int32(m.Width)),
				Height:            common.NullInt32(int32(m.Height)),
				AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
				Codec:             common.NullString(m.VideoCodec),
				AudioCodec:        common.NullString(m.AudioCodec),
				CodecProfile:      sql.NullString{}, // TODO: Extract from FFmpeg if available
				BitRate:           common.NullInt64(m.Bitrate),
				FrameRate:         common.NullFloat64(m.FrameRate),
				ScanType:          sql.NullString{},   // TODO: Extract from FFmpeg
				HdrFormat:         sql.NullString{},   // TODO: Extract from FFmpeg
				ColorSpace:        sql.NullString{},   // TODO: Extract from FFmpeg
				ColorPrimaries:    sql.NullString{},   // TODO: Extract from FFmpeg
				ThumbnailPath:     sql.NullString{},   // TODO: Generate during scan
				SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
				ResolutionLabel:   common.NullString(media.CalculateResolutionLabel(m.Height)),
				QualityScore:      sql.NullInt32{},    // TODO: Calculate heuristic
				Is3d:              sql.NullBool{},     // TODO: Detect from filename
				StereoMode:        sql.NullString{},   // TODO: Detect if 3D
				HasDash:           common.NullBool(false),
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
				Type:              m.Type,
				IsExtra:           m.IsExtra,
				FileHash:          sql.NullString{}, // TODO: Add Hash field to domain.Media
				ContainerFormat:   common.NullString(m.ContainerFormat),
				Width:             common.NullInt64(int64(m.Width)),
				Height:            common.NullInt64(int64(m.Height)),
				AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
				Codec:             common.NullString(m.VideoCodec),
				AudioCodec:        common.NullString(m.AudioCodec),
				CodecProfile:      sql.NullString{}, // TODO: Extract from FFmpeg if available
				BitRate:           common.NullInt64(m.Bitrate),
				FrameRate:         common.NullFloat64(m.FrameRate),
				ScanType:          sql.NullString{},   // TODO: Extract from FFmpeg
				HdrFormat:         sql.NullString{},   // TODO: Extract from FFmpeg
				ColorSpace:        sql.NullString{},   // TODO: Extract from FFmpeg
				ColorPrimaries:    sql.NullString{},   // TODO: Extract from FFmpeg
				ThumbnailPath:     sql.NullString{},   // TODO: Generate during scan
				SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
				ResolutionLabel:   common.NullString(media.CalculateResolutionLabel(m.Height)),
				QualityScore:      sql.NullInt64{},    // TODO: Calculate heuristic
				Is3d:              sql.NullBool{},     // TODO: Detect from filename
				StereoMode:        sql.NullString{},   // TODO: Detect if 3D
				HasDash:           common.NullBool(false),
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
			ID:              int64(pgResult.ID),
			LibraryID:       int64(pgResult.LibraryID),
			Title:           pgResult.Title,
			Type:            pgResult.Type,
			FilePath:        pgResult.FilePath,
			FileSize:        pgResult.FileSize.Int64,
			Duration:        int(pgResult.Duration.Float64),
			IsExtra:         pgResult.IsExtra,
			Width:           int(pgResult.Width.Int32),
			Height:          int(pgResult.Height.Int32),
			VideoCodec:      pgResult.Codec.String,
			AudioCodec:      pgResult.AudioCodec.String,
			Bitrate:         pgResult.BitRate.Int64,
			FrameRate:       pgResult.FrameRate.Float64,
			ContainerFormat: pgResult.ContainerFormat.String,
			CreatedAt:       common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt:       common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Medium)
	return &media.Media{
		ID:              sqResult.ID,
		LibraryID:       sqResult.LibraryID,
		Title:           sqResult.Title,
		Type:            sqResult.Type,
		FilePath:        sqResult.FilePath,
		FileSize:        sqResult.FileSize.Int64,
		Duration:        int(sqResult.Duration.Float64),
		IsExtra:         sqResult.IsExtra,
		Width:           int(sqResult.Width.Int64),
		Height:          int(sqResult.Height.Int64),
		VideoCodec:      sqResult.Codec.String,
		AudioCodec:      sqResult.AudioCodec.String,
		Bitrate:         sqResult.BitRate.Int64,
		FrameRate:       sqResult.FrameRate.Float64,
		ContainerFormat: sqResult.ContainerFormat.String,
		CreatedAt:       common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt:       common.ParseNullTime(sqResult.UpdatedAt),
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
			ID:              int64(pgResult.ID),
			LibraryID:       int64(pgResult.LibraryID),
			Title:           pgResult.Title,
			Type:            pgResult.Type,
			FilePath:        pgResult.FilePath,
			FileSize:        pgResult.FileSize.Int64,
			Duration:        int(pgResult.Duration.Float64),
			IsExtra:         pgResult.IsExtra,
			Width:           int(pgResult.Width.Int32),
			Height:          int(pgResult.Height.Int32),
			VideoCodec:      pgResult.Codec.String,
			AudioCodec:      pgResult.AudioCodec.String,
			Bitrate:         pgResult.BitRate.Int64,
			FrameRate:       pgResult.FrameRate.Float64,
			ContainerFormat: pgResult.ContainerFormat.String,
			CreatedAt:       common.ParseNullTime(pgResult.CreatedAt),
			UpdatedAt:       common.ParseNullTime(pgResult.UpdatedAt),
		}, nil
	}

	sqResult := result.(sqlc_sqlite.Medium)
	return &media.Media{
		ID:              sqResult.ID,
		LibraryID:       sqResult.LibraryID,
		Title:           sqResult.Title,
		Type:            sqResult.Type,
		FilePath:        sqResult.FilePath,
		FileSize:        sqResult.FileSize.Int64,
		Duration:        int(sqResult.Duration.Float64),
		IsExtra:         sqResult.IsExtra,
		Width:           int(sqResult.Width.Int64),
		Height:          int(sqResult.Height.Int64),
		VideoCodec:      sqResult.Codec.String,
		AudioCodec:      sqResult.AudioCodec.String,
		Bitrate:         sqResult.BitRate.Int64,
		FrameRate:       sqResult.FrameRate.Float64,
		ContainerFormat: sqResult.ContainerFormat.String,
		CreatedAt:       common.ParseNullTime(sqResult.CreatedAt),
		UpdatedAt:       common.ParseNullTime(sqResult.UpdatedAt),
	}, nil
}

// ListAll retrieves all media items across all libraries
func (r *Repository) ListAll(ctx context.Context) ([]*media.Media, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListAllMedia(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListAllMedia(ctx)
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
				ID:              int64(pgResult.ID),
				LibraryID:       int64(pgResult.LibraryID),
				Title:           pgResult.Title,
				Type:            pgResult.Type,
				FilePath:        pgResult.FilePath,
				FileSize:        pgResult.FileSize.Int64,
				Duration:        int(pgResult.Duration.Float64),
				IsExtra:         pgResult.IsExtra,
				Width:           int(pgResult.Width.Int32),
				Height:          int(pgResult.Height.Int32),
				VideoCodec:      pgResult.Codec.String,
				AudioCodec:      "",
				Bitrate:         pgResult.BitRate.Int64,
				FrameRate:       pgResult.FrameRate.Float64,
				ContainerFormat: pgResult.ContainerFormat.String,
				CreatedAt:       common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt:       common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = &media.Media{
			ID:              sqResult.ID,
			LibraryID:       sqResult.LibraryID,
			Title:           sqResult.Title,
			Type:            sqResult.Type,
			FilePath:        sqResult.FilePath,
			FileSize:        sqResult.FileSize.Int64,
			Duration:        int(sqResult.Duration.Float64),
			IsExtra:         sqResult.IsExtra,
			Width:           int(sqResult.Width.Int64),
			Height:          int(sqResult.Height.Int64),
			VideoCodec:      sqResult.Codec.String,
			AudioCodec:      "",
			Bitrate:         sqResult.BitRate.Int64,
			FrameRate:       sqResult.FrameRate.Float64,
			ContainerFormat: sqResult.ContainerFormat.String,
			CreatedAt:       common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt:       common.ParseNullTime(sqResult.UpdatedAt),
		}
	}
	return mediaList, nil
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
				ID:              int64(pgResult.ID),
				LibraryID:       int64(pgResult.LibraryID),
				Title:           pgResult.Title,
				Type:            pgResult.Type,
				FilePath:        pgResult.FilePath,
				FileSize:        pgResult.FileSize.Int64,
				Duration:        int(pgResult.Duration.Float64),
				IsExtra:         pgResult.IsExtra,
				Width:           int(pgResult.Width.Int32),
				Height:          int(pgResult.Height.Int32),
				VideoCodec:      pgResult.Codec.String,
				AudioCodec:      "",
				Bitrate:         pgResult.BitRate.Int64,
				FrameRate:       pgResult.FrameRate.Float64,
				ContainerFormat: pgResult.ContainerFormat.String,
				CreatedAt:       common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt:       common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = &media.Media{
			ID:              sqResult.ID,
			LibraryID:       sqResult.LibraryID,
			Title:           sqResult.Title,
			Type:            sqResult.Type,
			FilePath:        sqResult.FilePath,
			FileSize:        sqResult.FileSize.Int64,
			Duration:        int(sqResult.Duration.Float64),
			IsExtra:         sqResult.IsExtra,
			Width:           int(sqResult.Width.Int64),
			Height:          int(sqResult.Height.Int64),
			VideoCodec:      sqResult.Codec.String,
			AudioCodec:      "",
			Bitrate:         sqResult.BitRate.Int64,
			FrameRate:       sqResult.FrameRate.Float64,
			ContainerFormat: sqResult.ContainerFormat.String,
			CreatedAt:       common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt:       common.ParseNullTime(sqResult.UpdatedAt),
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
				ID:              int64(pgResult.ID),
				LibraryID:       int64(pgResult.LibraryID),
				Title:           pgResult.Title,
				Type:            pgResult.Type,
				FilePath:        pgResult.FilePath,
				FileSize:        pgResult.FileSize.Int64,
				Duration:        int(pgResult.Duration.Float64),
				IsExtra:         pgResult.IsExtra,
				Width:           int(pgResult.Width.Int32),
				Height:          int(pgResult.Height.Int32),
				VideoCodec:      pgResult.Codec.String,
				AudioCodec:      "",
				Bitrate:         pgResult.BitRate.Int64,
				FrameRate:       pgResult.FrameRate.Float64,
				ContainerFormat: pgResult.ContainerFormat.String,
				CreatedAt:       common.ParseNullTime(pgResult.CreatedAt),
				UpdatedAt:       common.ParseNullTime(pgResult.UpdatedAt),
			}
		}
		return mediaList, nil
	}

	sqResults := result.([]sqlc_sqlite.Medium)
	mediaList := make([]*media.Media, len(sqResults))
	for i, sqResult := range sqResults {
		mediaList[i] = &media.Media{
			ID:              sqResult.ID,
			LibraryID:       sqResult.LibraryID,
			Title:           sqResult.Title,
			Type:            sqResult.Type,
			FilePath:        sqResult.FilePath,
			FileSize:        sqResult.FileSize.Int64,
			Duration:        int(sqResult.Duration.Float64),
			IsExtra:         sqResult.IsExtra,
			Width:           int(sqResult.Width.Int64),
			Height:          int(sqResult.Height.Int64),
			VideoCodec:      sqResult.Codec.String,
			AudioCodec:      "",
			Bitrate:         sqResult.BitRate.Int64,
			FrameRate:       sqResult.FrameRate.Float64,
			ContainerFormat: sqResult.ContainerFormat.String,
			CreatedAt:       common.ParseNullTime(sqResult.CreatedAt),
			UpdatedAt:       common.ParseNullTime(sqResult.UpdatedAt),
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
				Type:              m.Type,
				IsExtra:           m.IsExtra,
				FileHash:          sql.NullString{}, // TODO: Add Hash field to domain.Media
				ContainerFormat:   common.NullString(m.ContainerFormat),
				Width:             common.NullInt32(int32(m.Width)),
				Height:            common.NullInt32(int32(m.Height)),
				AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
				Codec:             common.NullString(m.VideoCodec),
				AudioCodec:        common.NullString(m.AudioCodec),
				CodecProfile:      sql.NullString{}, // TODO: Extract from FFmpeg if available
				BitRate:           common.NullInt64(m.Bitrate),
				FrameRate:         common.NullFloat64(m.FrameRate),
				ScanType:          sql.NullString{},   // TODO: Extract from FFmpeg
				HdrFormat:         sql.NullString{},   // TODO: Extract from FFmpeg
				ColorSpace:        sql.NullString{},   // TODO: Extract from FFmpeg
				ColorPrimaries:    sql.NullString{},   // TODO: Extract from FFmpeg
				ThumbnailPath:     sql.NullString{},   // TODO: Generate during scan
				SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
				ResolutionLabel:   common.NullString(media.CalculateResolutionLabel(m.Height)),
				QualityScore:      sql.NullInt32{},    // TODO: Calculate heuristic
				Is3d:              sql.NullBool{},     // TODO: Detect from filename
				StereoMode:        sql.NullString{},   // TODO: Detect if 3D
				HasDash:           common.NullBool(false),
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
				Type:              m.Type,
				IsExtra:           m.IsExtra,
				FileHash:          sql.NullString{}, // TODO: Add Hash field to domain.Media
				ContainerFormat:   common.NullString(m.ContainerFormat),
				Width:             common.NullInt64(int64(m.Width)),
				Height:            common.NullInt64(int64(m.Height)),
				AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
				Codec:             common.NullString(m.VideoCodec),
				AudioCodec:        common.NullString(m.AudioCodec),
				CodecProfile:      sql.NullString{}, // TODO: Extract from FFmpeg if available
				BitRate:           common.NullInt64(m.Bitrate),
				FrameRate:         common.NullFloat64(m.FrameRate),
				ScanType:          sql.NullString{},   // TODO: Extract from FFmpeg
				HdrFormat:         sql.NullString{},   // TODO: Extract from FFmpeg
				ColorSpace:        sql.NullString{},   // TODO: Extract from FFmpeg
				ColorPrimaries:    sql.NullString{},   // TODO: Extract from FFmpeg
				ThumbnailPath:     sql.NullString{},   // TODO: Generate during scan
				SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
				ResolutionLabel:   common.NullString(media.CalculateResolutionLabel(m.Height)),
				QualityScore:      sql.NullInt64{},    // TODO: Calculate heuristic
				Is3d:              sql.NullBool{},     // TODO: Detect from filename
				StereoMode:        sql.NullString{},   // TODO: Detect if 3D
				HasDash:           common.NullBool(false),
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
