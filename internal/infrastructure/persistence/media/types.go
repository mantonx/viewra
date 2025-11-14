package media

import (
	"database/sql"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.Repository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type Repository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}

// pgMediumToDomain converts a PostgreSQL Medium model to a domain Media entity.
func pgMediumToDomain(pg sqlc_postgres.Medium) *media.Media {
	return &media.Media{
		ID:              int64(pg.ID),
		LibraryID:       int64(pg.LibraryID),
		Title:           pg.Title,
		Type:            pg.Type,
		FilePath:        pg.FilePath,
		FileSize:        pg.FileSize.Int64,
		Duration:        int(pg.Duration.Float64),
		IsExtra:         pg.IsExtra,
		Width:           int(pg.Width.Int32),
		Height:          int(pg.Height.Int32),
		VideoCodec:      pg.Codec.String,
		AudioCodec:      pg.AudioCodec.String,
		Bitrate:         pg.BitRate.Int64,
		FrameRate:       pg.FrameRate.Float64,
		ContainerFormat: pg.ContainerFormat.String,
		CreatedAt:       common.ParseNullTime(pg.CreatedAt),
		UpdatedAt:       common.ParseNullTime(pg.UpdatedAt),
	}
}

// sqliteMediumToDomain converts a SQLite Medium model to a domain Media entity.
func sqliteMediumToDomain(sq sqlc_sqlite.Medium) *media.Media {
	return &media.Media{
		ID:              sq.ID,
		LibraryID:       sq.LibraryID,
		Title:           sq.Title,
		Type:            sq.Type,
		FilePath:        sq.FilePath,
		FileSize:        sq.FileSize.Int64,
		Duration:        int(sq.Duration.Float64),
		IsExtra:         sq.IsExtra,
		Width:           int(sq.Width.Int64),
		Height:          int(sq.Height.Int64),
		VideoCodec:      sq.Codec.String,
		AudioCodec:      sq.AudioCodec.String,
		Bitrate:         sq.BitRate.Int64,
		FrameRate:       sq.FrameRate.Float64,
		ContainerFormat: sq.ContainerFormat.String,
		CreatedAt:       common.ParseNullTime(sq.CreatedAt),
		UpdatedAt:       common.ParseNullTime(sq.UpdatedAt),
	}
}

// buildPostgresCreateParams builds CreateMediaParams for PostgreSQL from a domain Media entity.
func buildPostgresCreateParams(m *media.Media) sqlc_postgres.CreateMediaParams {
	return sqlc_postgres.CreateMediaParams{
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
	}
}

// buildSQLiteCreateParams builds CreateMediaParams for SQLite from a domain Media entity.
func buildSQLiteCreateParams(m *media.Media) sqlc_sqlite.CreateMediaParams {
	return sqlc_sqlite.CreateMediaParams{
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
	}
}

// buildPostgresUpdateParams builds UpdateMediaParams for PostgreSQL from a domain Media entity.
func buildPostgresUpdateParams(m *media.Media) sqlc_postgres.UpdateMediaParams {
	params := buildPostgresCreateParams(m)
	return sqlc_postgres.UpdateMediaParams{
		LibraryID:         params.LibraryID,
		Title:             params.Title,
		FilePath:          params.FilePath,
		FileSize:          params.FileSize,
		Duration:          params.Duration,
		Type:              params.Type,
		IsExtra:           params.IsExtra,
		FileHash:          params.FileHash,
		ContainerFormat:   params.ContainerFormat,
		Width:             params.Width,
		Height:            params.Height,
		AspectRatio:       params.AspectRatio,
		Codec:             params.Codec,
		AudioCodec:        params.AudioCodec,
		CodecProfile:      params.CodecProfile,
		BitRate:           params.BitRate,
		FrameRate:         params.FrameRate,
		ScanType:          params.ScanType,
		HdrFormat:         params.HdrFormat,
		ColorSpace:        params.ColorSpace,
		ColorPrimaries:    params.ColorPrimaries,
		ThumbnailPath:     params.ThumbnailPath,
		SourceType:        params.SourceType,
		ResolutionLabel:   params.ResolutionLabel,
		QualityScore:      params.QualityScore,
		Is3d:              params.Is3d,
		StereoMode:        params.StereoMode,
		HasDash:           params.HasDash,
		DashManifestPath:  params.DashManifestPath,
		TranscodingStatus: params.TranscodingStatus,
		ID:                int32(m.ID),
	}
}

// buildSQLiteUpdateParams builds UpdateMediaParams for SQLite from a domain Media entity.
func buildSQLiteUpdateParams(m *media.Media) sqlc_sqlite.UpdateMediaParams {
	params := buildSQLiteCreateParams(m)
	return sqlc_sqlite.UpdateMediaParams{
		LibraryID:         params.LibraryID,
		Title:             params.Title,
		FilePath:          params.FilePath,
		FileSize:          params.FileSize,
		Duration:          params.Duration,
		Type:              params.Type,
		IsExtra:           params.IsExtra,
		FileHash:          params.FileHash,
		ContainerFormat:   params.ContainerFormat,
		Width:             params.Width,
		Height:            params.Height,
		AspectRatio:       params.AspectRatio,
		Codec:             params.Codec,
		AudioCodec:        params.AudioCodec,
		CodecProfile:      params.CodecProfile,
		BitRate:           params.BitRate,
		FrameRate:         params.FrameRate,
		ScanType:          params.ScanType,
		HdrFormat:         params.HdrFormat,
		ColorSpace:        params.ColorSpace,
		ColorPrimaries:    params.ColorPrimaries,
		ThumbnailPath:     params.ThumbnailPath,
		SourceType:        params.SourceType,
		ResolutionLabel:   params.ResolutionLabel,
		QualityScore:      params.QualityScore,
		Is3d:              params.Is3d,
		StereoMode:        params.StereoMode,
		HasDash:           params.HasDash,
		DashManifestPath:  params.DashManifestPath,
		TranscodingStatus: params.TranscodingStatus,
		ID:                m.ID,
	}
}
