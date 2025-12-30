package music

import (
	"database/sql"
	"time"
	"unsafe"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// ========================================
// Generic Music Track Mapper
// ========================================

// mapMusicTrackToDomain converts any music track query row to domain MusicTrack
// Handles both SQLite and PostgreSQL row types using reflection-based field getters
// This single function replaces 400+ lines of duplicate SQLite/PostgreSQL converters
func mapMusicTrackToDomain(row interface{}) *media.MusicTrack {
	return &media.MusicTrack{
		Media: media.Media{
			ID:              common.IntFieldGetter(row, "MediaID_2"),
			LibraryID:       common.IntFieldGetter(row, "LibraryID"),
			Title:           common.StringFieldGetter(row, "Title"),
			Type:            common.StringFieldGetter(row, "Type"),
			FilePath:        common.StringFieldGetter(row, "FilePath"),
			FileSize:        common.ParseNullInt64(common.NullIntFieldGetter(row, "FileSize")),
			Duration:        int(common.Float64FieldGetter(row, "Duration")),
			IsExtra:         common.BoolFieldGetter(row, "IsExtra"),
			Width:           int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Width"))),
			Height:          int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Height"))),
			VideoCodec:      common.ParseNullString(common.NullStringFieldGetter(row, "Codec")),
			AudioCodec:      common.ParseNullString(common.NullStringFieldGetter(row, "AudioCodec")),
			Bitrate:         common.ParseNullInt64(common.NullIntFieldGetter(row, "BitRate")),
			FrameRate:       common.Float64FieldGetter(row, "FrameRate"),
			ContainerFormat: common.ParseNullString(common.NullStringFieldGetter(row, "ContainerFormat")),
			CreatedAt:       common.ParseNullTime(common.TimeFieldGetter(row, "CreatedAt")),
			UpdatedAt:       common.ParseNullTime(common.TimeFieldGetter(row, "UpdatedAt")),
		},
		Artist:              common.ParseNullString(common.NullStringFieldGetter(row, "Artist")),
		Album:               common.ParseNullString(common.NullStringFieldGetter(row, "Album")),
		AlbumArtist:         common.ParseNullString(common.NullStringFieldGetter(row, "AlbumArtist")),
		TrackNumber:         int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TrackNumber"))),
		DiscNumber:          int(common.ParseNullInt64(common.NullIntFieldGetter(row, "DiscNumber"))),
		Year:                int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Year"))),
		Genre:               common.ParseNullString(common.NullStringFieldGetter(row, "Genre")),
		Composer:            common.ParseNullString(common.NullStringFieldGetter(row, "Composer")),
		Publisher:           common.ParseNullString(common.NullStringFieldGetter(row, "RecordLabel")),
		Bitrate:             int(common.ParseNullInt64(common.NullIntFieldGetter(row, "BitRate")) / 1000),
		TotalTracks:         int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TotalTracks"))),
		TotalDiscs:          int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TotalDiscs"))),
		ReleaseDate:         common.FormatNullDate(common.TimeFieldGetter(row, "ReleaseDate")),
		Lyricist:            common.ParseNullString(common.NullStringFieldGetter(row, "Lyricist")),
		ISRC:                common.ParseNullString(common.NullStringFieldGetter(row, "Isrc")),
		ReleaseType:         common.ParseNullString(common.NullStringFieldGetter(row, "ReleaseType")),
		Compilation:         common.ParseNullBool(sql.NullBool{Bool: common.BoolFieldGetter(row, "Compilation"), Valid: true}),
		OriginalTitle:       common.ParseNullString(common.NullStringFieldGetter(row, "OriginalTitle")),
		SortTitle:           common.ParseNullString(common.NullStringFieldGetter(row, "SortTitle")),
		MusicBrainzTrackID:  common.ParseNullString(common.NullStringFieldGetter(row, "MusicbrainzTrackID")),
		MusicBrainzAlbumID:  common.ParseNullString(common.NullStringFieldGetter(row, "MusicbrainzAlbumID")),
		MusicBrainzArtistID: common.ParseNullString(common.NullStringFieldGetter(row, "MusicbrainzArtistID")),
		AlbumID:             common.IntFieldGetter(row, "AlbumID"),
		ArtistID:            common.IntFieldGetter(row, "ArtistID"),
	}
}

// ========================================
// Generic Album Mapper
// ========================================

// mapAlbumToDomain converts any album row to domain Album
func mapAlbumToDomain(row interface{}) *media.Album {
	createdAt := common.TimeFieldGetter(row, "CreatedAt")
	updatedAt := common.TimeFieldGetter(row, "UpdatedAt")

	var createdAtStr, updatedAtStr string
	if createdAt.Valid {
		createdAtStr = createdAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if updatedAt.Valid {
		updatedAtStr = updatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Album{
		ID:                 common.IntFieldGetter(row, "ID"),
		LibraryID:          common.IntFieldGetter(row, "LibraryID"),
		ArtistID:           common.IntFieldGetter(row, "ArtistID"),
		Title:              common.StringFieldGetter(row, "Title"),
		AlbumArtist:        common.ParseNullString(common.NullStringFieldGetter(row, "AlbumArtist")),
		Artist:             common.ParseNullString(common.NullStringFieldGetter(row, "Artist")),
		Year:               int(common.ParseNullInt64(common.NullIntFieldGetter(row, "Year"))),
		ReleaseDate:        common.FormatNullDate(common.TimeFieldGetter(row, "ReleaseDate")),
		Genre:              common.ParseNullString(common.NullStringFieldGetter(row, "Genre")),
		TotalTracks:        int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TotalTracks"))),
		TotalDiscs:         int(common.ParseNullInt64(common.NullIntFieldGetter(row, "TotalDiscs"))),
		RecordLabel:        common.ParseNullString(common.NullStringFieldGetter(row, "RecordLabel")),
		ReleaseType:        common.ParseNullString(common.NullStringFieldGetter(row, "ReleaseType")),
		Compilation:        common.ParseNullBool(sql.NullBool{Bool: common.BoolFieldGetter(row, "Compilation"), Valid: true}),
		MusicBrainzAlbumID: common.ParseNullString(common.NullStringFieldGetter(row, "MusicbrainzAlbumID")),
		CoverArtPath:       common.ParseNullString(common.NullStringFieldGetter(row, "CoverArtPath")),
		SortTitle:          common.ParseNullString(common.NullStringFieldGetter(row, "SortTitle")),
		Directory:          common.ParseNullString(common.NullStringFieldGetter(row, "Directory")),
		CreatedAt:          createdAtStr,
		UpdatedAt:          updatedAtStr,
	}
}

// ========================================
// Generic Artist Mapper
// ========================================

// mapArtistToDomain converts any artist row to domain Artist
func mapArtistToDomain(row interface{}) *media.Artist {
	createdAt := common.TimeFieldGetter(row, "CreatedAt")
	updatedAt := common.TimeFieldGetter(row, "UpdatedAt")

	var createdAtStr, updatedAtStr string
	if createdAt.Valid {
		createdAtStr = createdAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if updatedAt.Valid {
		updatedAtStr = updatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Artist{
		ID:                  common.IntFieldGetter(row, "ID"),
		LibraryID:           common.IntFieldGetter(row, "LibraryID"),
		Name:                common.StringFieldGetter(row, "Name"),
		SortName:            common.ParseNullString(common.NullStringFieldGetter(row, "SortName")),
		Bio:                 common.ParseNullString(common.NullStringFieldGetter(row, "Bio")),
		Country:             common.ParseNullString(common.NullStringFieldGetter(row, "Country")),
		FormedYear:          int(common.ParseNullInt64(common.NullIntFieldGetter(row, "FormedYear"))),
		Genre:               common.ParseNullString(common.NullStringFieldGetter(row, "Genre")),
		MusicBrainzArtistID: common.ParseNullString(common.NullStringFieldGetter(row, "MusicbrainzArtistID")),
		ImagePath:           common.ParseNullString(common.NullStringFieldGetter(row, "ImagePath")),
		Directory:           common.ParseNullString(common.NullStringFieldGetter(row, "Directory")),
		CreatedAt:           createdAtStr,
		UpdatedAt:           updatedAtStr,
	}
}

// ========================================
// SQLite/PostgreSQL Specific Mappers
// ========================================

// buildSQLiteCreateMediaParams builds CreateMediaParams for SQLite from a domain Media entity
func buildSQLiteCreateMediaParams(m *media.Media) sqlc_sqlite.CreateMediaParams {
	return sqlc_sqlite.CreateMediaParams{
		LibraryID:         m.LibraryID,
		Title:             m.Title,
		FilePath:          m.FilePath,
		FileSize:          common.NullInt64(m.FileSize),
		Duration:          common.NullFloat64(float64(m.Duration)),
		Type:              m.Type,
		IsExtra:           common.BoolToInt64(m.IsExtra),
		FileHash:          common.NullString(m.FileHash),
		ContainerFormat:   common.NullString(m.ContainerFormat),
		Width:             common.NullInt64(int64(m.Width)),
		Height:            common.NullInt64(int64(m.Height)),
		AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
		Codec:             common.NullString(m.VideoCodec),
		AudioCodec:        common.NullString(m.AudioCodec),
		CodecProfile:      common.NullString(m.CodecProfile),
		BitRate:           common.NullInt64(m.Bitrate),
		FrameRate:         common.NullFloat64(m.FrameRate),
		ScanType:          common.NullString(m.ScanType),
		HdrFormat:         common.NullString(m.HDRFormat),
		ColorSpace:        common.NullString(m.ColorSpace),
		ColorPrimaries:    common.NullString(m.ColorPrimaries),
		ThumbnailPath:     sql.NullString{},
		SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
		ResolutionLabel:   common.NullString(media.CalculateResolutionLabelFromDimensions(m.Width, m.Height)),
		QualityScore:      sql.NullInt64{},
		Is3d:              common.NullInt64FromBool(func() bool { is3d, _ := media.Detect3D(m.FilePath); return is3d }()),
		StereoMode:        common.NullString(func() string { _, stereoMode := media.Detect3D(m.FilePath); return stereoMode }()),
		HasDash:           common.NullInt64FromBool(false),
		DashManifestPath:  sql.NullString{},
		TranscodingStatus: sql.NullString{},
	}
}

// sqliteMusicTrackToDomain converts a SQLite music track query row to a domain MusicTrack entity
func sqliteMusicTrackToDomain(row sqlc_sqlite.GetMusicTrackByMediaIDRow) *media.MusicTrack {
	return mapMusicTrackToDomain(row)
}

// musicTrackFields holds all the fields needed to construct a MusicTrack from various query row types
type musicTrackFields struct {
	MediaID             int64
	Artist              sql.NullString
	Album               sql.NullString
	AlbumArtist         sql.NullString
	TrackNumber         sql.NullInt64
	DiscNumber          sql.NullInt64
	Genre               sql.NullString
	Year                sql.NullInt64
	Composer            sql.NullString
	RecordLabel         sql.NullString
	TotalTracks         sql.NullInt64
	TotalDiscs          sql.NullInt64
	ReleaseDate         sql.NullTime
	Lyricist            sql.NullString
	ISRC                sql.NullString
	ReleaseType         sql.NullString
	Compilation         sql.NullBool
	OriginalTitle       sql.NullString
	SortTitle           sql.NullString
	MusicBrainzTrackID  sql.NullString
	MusicBrainzAlbumID  sql.NullString
	MusicBrainzArtistID sql.NullString
	AlbumID             sql.NullInt64
	ArtistID            sql.NullInt64
	MediaID2            int64
	LibraryID           int64
	Title               string
	FilePath            string
	FileSize            sql.NullInt64
	Duration            sql.NullFloat64
	Width               sql.NullInt64
	Height              sql.NullInt64
	Codec               sql.NullString
	AudioCodec          sql.NullString
	BitRate             sql.NullInt64
	FrameRate           sql.NullFloat64
	ContainerFormat     sql.NullString
	Type                string
	IsExtra             bool
	CreatedAt           sql.NullTime
	UpdatedAt           sql.NullTime
}

// sqliteMusicTrackRowToDomain converts music track fields to a domain MusicTrack entity
func sqliteMusicTrackRowToDomain(fields musicTrackFields) *media.MusicTrack {
	return &media.MusicTrack{
		Media: media.Media{
			ID:              fields.MediaID2,
			LibraryID:       fields.LibraryID,
			Title:           fields.Title,
			Type:            fields.Type,
			FilePath:        fields.FilePath,
			FileSize:        common.ParseNullInt64(fields.FileSize),
			Duration:        int(common.ParseNullFloat64(fields.Duration)),
			IsExtra:         fields.IsExtra,
			Width:           int(common.ParseNullInt64(fields.Width)),
			Height:          int(common.ParseNullInt64(fields.Height)),
			VideoCodec:      common.ParseNullString(fields.Codec),
			AudioCodec:      common.ParseNullString(fields.AudioCodec),
			Bitrate:         common.ParseNullInt64(fields.BitRate),
			FrameRate:       common.ParseNullFloat64(fields.FrameRate),
			ContainerFormat: common.ParseNullString(fields.ContainerFormat),
			CreatedAt:       common.ParseNullTime(fields.CreatedAt),
			UpdatedAt:       common.ParseNullTime(fields.UpdatedAt),
		},
		// Basic music metadata
		Artist:      common.ParseNullString(fields.Artist),
		Album:       common.ParseNullString(fields.Album),
		AlbumArtist: common.ParseNullString(fields.AlbumArtist),
		TrackNumber: int(common.ParseNullInt64(fields.TrackNumber)),
		DiscNumber:  int(common.ParseNullInt64(fields.DiscNumber)),
		Year:        int(common.ParseNullInt64(fields.Year)),
		Genre:       common.ParseNullString(fields.Genre),
		Composer:    common.ParseNullString(fields.Composer),
		Publisher:   common.ParseNullString(fields.RecordLabel),
		Bitrate:     int(common.ParseNullInt64(fields.BitRate) / 1000), // Convert from bps to kbps
		// Extended metadata
		TotalTracks:         int(common.ParseNullInt64(fields.TotalTracks)),
		TotalDiscs:          int(common.ParseNullInt64(fields.TotalDiscs)),
		ReleaseDate:         common.FormatNullDate(fields.ReleaseDate),
		Lyricist:            common.ParseNullString(fields.Lyricist),
		ISRC:                common.ParseNullString(fields.ISRC),
		ReleaseType:         common.ParseNullString(fields.ReleaseType),
		Compilation:         common.ParseNullBool(fields.Compilation),
		OriginalTitle:       common.ParseNullString(fields.OriginalTitle),
		SortTitle:           common.ParseNullString(fields.SortTitle),
		MusicBrainzTrackID:  common.ParseNullString(fields.MusicBrainzTrackID),
		MusicBrainzAlbumID:  common.ParseNullString(fields.MusicBrainzAlbumID),
		MusicBrainzArtistID: common.ParseNullString(fields.MusicBrainzArtistID),
		// Entity references
		AlbumID:  common.ParseNullInt64(fields.AlbumID),
		ArtistID: common.ParseNullInt64(fields.ArtistID),
	}
}

// buildSQLiteCreateMusicTrackParams builds CreateMusicTrackParams for SQLite from a domain MusicTrack entity
func buildSQLiteCreateMusicTrackParams(t *media.MusicTrack) sqlc_sqlite.CreateMusicTrackParams {
	// Use AlbumArtist if set, otherwise default to track artist
	albumArtist := t.AlbumArtist
	if albumArtist == "" {
		albumArtist = t.Artist
	}

	// Use SortTitle from track if set, otherwise generate from title
	sortTitle := t.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(t.Media.Title)
	}

	return sqlc_sqlite.CreateMusicTrackParams{
		MediaID:             t.Media.ID,
		Artist:              common.NullString(t.Artist),
		Album:               common.NullString(t.Album),
		AlbumArtist:         common.NullString(albumArtist),
		TrackNumber:         common.NullInt64(int64(t.TrackNumber)),
		DiscNumber:          common.NullInt64(int64(t.DiscNumber)),
		TotalTracks:         common.NullInt64(int64(t.TotalTracks)),
		TotalDiscs:          common.NullInt64(int64(t.TotalDiscs)),
		Genre:               common.NullString(t.Genre),
		Year:                common.NullInt64(int64(t.Year)),
		ReleaseDate:         common.ParseDateString(t.ReleaseDate),
		Composer:            common.NullString(t.Composer),
		Lyricist:            common.NullString(t.Lyricist),
		RecordLabel:         common.NullString(t.Publisher),
		Isrc:                common.NullString(t.ISRC),
		ReleaseType:         common.NullString(t.ReleaseType),
		Compilation:         common.NullInt64FromBool(t.Compilation),
		MusicbrainzTrackID:  common.NullString(t.MusicBrainzTrackID),
		MusicbrainzAlbumID:  common.NullString(t.MusicBrainzAlbumID),
		MusicbrainzArtistID: common.NullString(t.MusicBrainzArtistID),
		OriginalTitle:       common.NullString(t.OriginalTitle),
		SortTitle:           common.NullString(sortTitle),
		AlbumID:             common.NullInt64(t.AlbumID),
		ArtistID:            common.NullInt64(t.ArtistID),
	}
}

// buildSQLiteUpdateMusicTrackParams builds UpdateMusicTrackParams for SQLite from a domain MusicTrack entity
func buildSQLiteUpdateMusicTrackParams(t *media.MusicTrack) sqlc_sqlite.UpdateMusicTrackParams {
	params := buildSQLiteCreateMusicTrackParams(t)
	return sqlc_sqlite.UpdateMusicTrackParams{
		Artist:              params.Artist,
		Album:               params.Album,
		AlbumArtist:         params.AlbumArtist,
		TrackNumber:         params.TrackNumber,
		DiscNumber:          params.DiscNumber,
		TotalTracks:         params.TotalTracks,
		TotalDiscs:          params.TotalDiscs,
		Genre:               params.Genre,
		Year:                params.Year,
		ReleaseDate:         params.ReleaseDate,
		Composer:            params.Composer,
		Lyricist:            params.Lyricist,
		RecordLabel:         params.RecordLabel,
		Isrc:                params.Isrc,
		ReleaseType:         params.ReleaseType,
		Compilation:         params.Compilation,
		MusicbrainzTrackID:  params.MusicbrainzTrackID,
		MusicbrainzAlbumID:  params.MusicbrainzAlbumID,
		MusicbrainzArtistID: params.MusicbrainzArtistID,
		OriginalTitle:       params.OriginalTitle,
		SortTitle:           params.SortTitle,
		AlbumID:             params.AlbumID,
		ArtistID:            params.ArtistID,
		MediaID:             t.Media.ID,
	}
}

// sqliteAlbumToDomain converts a SQLite album row to a domain Album entity
func sqliteAlbumToDomain(row sqlc_sqlite.MusicAlbum) *media.Album {
	return mapAlbumToDomain(row)
}

// buildSQLiteCreateAlbumParams builds CreateAlbumParams for SQLite from a domain Album entity
func buildSQLiteCreateAlbumParams(a *media.Album) sqlc_sqlite.CreateAlbumParams {
	sortTitle := a.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(a.Title)
	}

	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}

	return sqlc_sqlite.CreateAlbumParams{
		LibraryID:          a.LibraryID,
		Title:              a.Title,
		AlbumArtist:        common.NullString(a.AlbumArtist),
		Artist:             common.NullString(a.Artist),
		Year:               common.NullInt64(int64(a.Year)),
		ReleaseDate:        common.ParseDateString(a.ReleaseDate),
		Genre:              common.NullString(a.Genre),
		TotalTracks:        common.NullInt64(int64(a.TotalTracks)),
		TotalDiscs:         common.NullInt64(int64(a.TotalDiscs)),
		RecordLabel:        common.NullString(a.RecordLabel),
		ReleaseType:        common.NullString(a.ReleaseType),
		Compilation:        common.NullInt64FromBool(a.Compilation),
		MusicbrainzAlbumID: common.NullString(a.MusicBrainzAlbumID),
		CoverArtPath:       common.NullString(a.CoverArtPath),
		SortTitle:          common.NullString(sortTitle),
		CreatedAt:          now,
		UpdatedAt:          now,
		ArtistID:           common.NullInt64(a.ArtistID),
		Directory:          common.NullString(a.Directory),
	}
}

// buildSQLiteUpdateAlbumParams builds UpdateAlbumParams for SQLite from a domain Album entity
func buildSQLiteUpdateAlbumParams(a *media.Album) sqlc_sqlite.UpdateAlbumParams {
	params := buildSQLiteCreateAlbumParams(a)
	return sqlc_sqlite.UpdateAlbumParams{
		Title:              params.Title,
		AlbumArtist:        params.AlbumArtist,
		Artist:             params.Artist,
		Year:               params.Year,
		ReleaseDate:        params.ReleaseDate,
		Genre:              params.Genre,
		TotalTracks:        params.TotalTracks,
		TotalDiscs:         params.TotalDiscs,
		RecordLabel:        params.RecordLabel,
		ReleaseType:        params.ReleaseType,
		Compilation:        params.Compilation,
		MusicbrainzAlbumID: params.MusicbrainzAlbumID,
		CoverArtPath:       params.CoverArtPath,
		SortTitle:          params.SortTitle,
		Directory:          params.Directory,
		UpdatedAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:                 a.ID,
	}
}

// sqliteArtistToDomain converts a SQLite artist row to a domain Artist entity
func sqliteArtistToDomain(row sqlc_sqlite.MusicArtist) *media.Artist {
	return mapArtistToDomain(row)
}

// buildSQLiteCreateArtistParams builds parameters for creating an artist in SQLite
func buildSQLiteCreateArtistParams(a *media.Artist) sqlc_sqlite.CreateArtistParams {
	return sqlc_sqlite.CreateArtistParams{
		LibraryID:           a.LibraryID,
		Name:                a.Name,
		SortName:            common.NullString(a.SortName),
		MusicbrainzArtistID: common.NullString(a.MusicBrainzArtistID),
		Bio:                 common.NullString(a.Bio),
		Country:             common.NullString(a.Country),
		FormedYear:          common.NullInt64(int64(a.FormedYear)),
		Genre:               common.NullString(a.Genre),
		ImagePath:           common.NullString(a.ImagePath),
		CreatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		Directory:           common.NullString(a.Directory),
	}
}

// buildSQLiteUpdateArtistParams builds parameters for updating an artist in SQLite
func buildSQLiteUpdateArtistParams(a *media.Artist) sqlc_sqlite.UpdateArtistParams {
	return sqlc_sqlite.UpdateArtistParams{
		Name:                a.Name,
		SortName:            common.NullString(a.SortName),
		MusicbrainzArtistID: common.NullString(a.MusicBrainzArtistID),
		Bio:                 common.NullString(a.Bio),
		Country:             common.NullString(a.Country),
		FormedYear:          common.NullInt64(int64(a.FormedYear)),
		Genre:               common.NullString(a.Genre),
		ImagePath:           common.NullString(a.ImagePath),
		Directory:           common.NullString(a.Directory),
		UpdatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:                  a.ID,
	}
}

// ========================================
// PostgreSQL Mappers
// ========================================

// postgresMusicTrackToDomain converts a PostgreSQL GetMusicTrackByMediaIDRow to domain MusicTrack
func postgresMusicTrackToDomain(row sqlc_postgres.GetMusicTrackByMediaIDRow) *media.MusicTrack {
	return mapMusicTrackToDomain(row)
}

// postgresAlbumToDomain converts a PostgreSQL MusicAlbum row to domain Album
func postgresAlbumToDomain(row sqlc_postgres.MusicAlbum) *media.Album {
	return mapAlbumToDomain(row)
}

// postgresArtistToDomain converts a PostgreSQL MusicArtist row to domain Artist
func postgresArtistToDomain(row sqlc_postgres.MusicArtist) *media.Artist {
	return mapArtistToDomain(row)
}

// ========================================
// PostgreSQL Param Builders
// ========================================

// buildPostgresCreateMusicTrackParams builds CreateMusicTrackParams for PostgreSQL from a domain MusicTrack entity
func buildPostgresCreateMusicTrackParams(t *media.MusicTrack) sqlc_postgres.CreateMusicTrackParams {
	// Use AlbumArtist if set, otherwise default to track artist
	albumArtist := t.AlbumArtist
	if albumArtist == "" {
		albumArtist = t.Artist
	}

	// Use SortTitle from track if set, otherwise generate from title
	sortTitle := t.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(t.Media.Title)
	}

	return sqlc_postgres.CreateMusicTrackParams{
		MediaID:             t.Media.ID,
		Artist:              common.NullString(t.Artist),
		Album:               common.NullString(t.Album),
		AlbumArtist:         common.NullString(albumArtist),
		TrackNumber:         common.NullInt64(int64(t.TrackNumber)),
		DiscNumber:          common.NullInt64(int64(t.DiscNumber)),
		TotalTracks:         common.NullInt64(int64(t.TotalTracks)),
		TotalDiscs:          common.NullInt64(int64(t.TotalDiscs)),
		Genre:               common.NullString(t.Genre),
		Year:                common.NullInt64(int64(t.Year)),
		ReleaseDate:         common.ParseDateString(t.ReleaseDate),
		Composer:            common.NullString(t.Composer),
		Lyricist:            common.NullString(t.Lyricist),
		RecordLabel:         common.NullString(t.Publisher),
		Isrc:                common.NullString(t.ISRC),
		ReleaseType:         common.NullString(t.ReleaseType),
		Compilation:         common.NullInt64FromBool(t.Compilation),
		MusicbrainzTrackID:  common.NullString(t.MusicBrainzTrackID),
		MusicbrainzAlbumID:  common.NullString(t.MusicBrainzAlbumID),
		MusicbrainzArtistID: common.NullString(t.MusicBrainzArtistID),
		OriginalTitle:       common.NullString(t.OriginalTitle),
		SortTitle:           common.NullString(sortTitle),
		AlbumID:             common.NullInt64(t.AlbumID),
		ArtistID:            common.NullInt64(t.ArtistID),
	}
}

// buildPostgresUpdateMusicTrackParams builds UpdateMusicTrackParams for PostgreSQL from a domain MusicTrack entity
func buildPostgresUpdateMusicTrackParams(t *media.MusicTrack) sqlc_postgres.UpdateMusicTrackParams {
	params := buildPostgresCreateMusicTrackParams(t)
	return sqlc_postgres.UpdateMusicTrackParams{
		Artist:              params.Artist,
		Album:               params.Album,
		AlbumArtist:         params.AlbumArtist,
		TrackNumber:         params.TrackNumber,
		DiscNumber:          params.DiscNumber,
		TotalTracks:         params.TotalTracks,
		TotalDiscs:          params.TotalDiscs,
		Genre:               params.Genre,
		Year:                params.Year,
		ReleaseDate:         params.ReleaseDate,
		Composer:            params.Composer,
		Lyricist:            params.Lyricist,
		RecordLabel:         params.RecordLabel,
		Isrc:                params.Isrc,
		ReleaseType:         params.ReleaseType,
		Compilation:         params.Compilation,
		MusicbrainzTrackID:  params.MusicbrainzTrackID,
		MusicbrainzAlbumID:  params.MusicbrainzAlbumID,
		MusicbrainzArtistID: params.MusicbrainzArtistID,
		OriginalTitle:       params.OriginalTitle,
		SortTitle:           params.SortTitle,
		AlbumID:             params.AlbumID,
		ArtistID:            params.ArtistID,
		MediaID:             t.Media.ID,
	}
}

// buildPostgresCreateAlbumParams builds CreateAlbumParams for PostgreSQL from a domain Album entity
func buildPostgresCreateAlbumParams(a *media.Album) sqlc_postgres.CreateAlbumParams {
	sortTitle := a.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(a.Title)
	}

	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}

	return sqlc_postgres.CreateAlbumParams{
		LibraryID:          a.LibraryID,
		Title:              a.Title,
		AlbumArtist:        common.NullString(a.AlbumArtist),
		Artist:             common.NullString(a.Artist),
		Year:               common.NullInt64(int64(a.Year)),
		ReleaseDate:        common.ParseDateString(a.ReleaseDate),
		Genre:              common.NullString(a.Genre),
		TotalTracks:        common.NullInt64(int64(a.TotalTracks)),
		TotalDiscs:         common.NullInt64(int64(a.TotalDiscs)),
		RecordLabel:        common.NullString(a.RecordLabel),
		ReleaseType:        common.NullString(a.ReleaseType),
		Compilation:        common.NullInt64FromBool(a.Compilation),
		MusicbrainzAlbumID: common.NullString(a.MusicBrainzAlbumID),
		CoverArtPath:       common.NullString(a.CoverArtPath),
		SortTitle:          common.NullString(sortTitle),
		CreatedAt:          now,
		UpdatedAt:          now,
		ArtistID:           common.NullInt64(a.ArtistID),
		Directory:          common.NullString(a.Directory),
	}
}

// buildPostgresUpdateAlbumParams builds UpdateAlbumParams for PostgreSQL from a domain Album entity
func buildPostgresUpdateAlbumParams(a *media.Album) sqlc_postgres.UpdateAlbumParams {
	sortTitle := a.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(a.Title)
	}

	return sqlc_postgres.UpdateAlbumParams{
		ID:                 a.ID,
		Title:              a.Title,
		AlbumArtist:        common.NullString(a.AlbumArtist),
		Artist:             common.NullString(a.Artist),
		Year:               common.NullInt64(int64(a.Year)),
		ReleaseDate:        common.ParseDateString(a.ReleaseDate),
		Genre:              common.NullString(a.Genre),
		TotalTracks:        common.NullInt64(int64(a.TotalTracks)),
		TotalDiscs:         common.NullInt64(int64(a.TotalDiscs)),
		RecordLabel:        common.NullString(a.RecordLabel),
		ReleaseType:        common.NullString(a.ReleaseType),
		Compilation:        common.NullInt64FromBool(a.Compilation),
		MusicbrainzAlbumID: common.NullString(a.MusicBrainzAlbumID),
		CoverArtPath:       common.NullString(a.CoverArtPath),
		SortTitle:          common.NullString(sortTitle),
		Directory:          common.NullString(a.Directory),
		UpdatedAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
}

// buildPostgresCreateMediaParams builds CreateMediaParams for PostgreSQL from a domain Media entity
func buildPostgresCreateMediaParams(m *media.Media) sqlc_postgres.CreateMediaParams {
	return sqlc_postgres.CreateMediaParams{
		LibraryID:         m.LibraryID,
		Title:             m.Title,
		FilePath:          m.FilePath,
		FileSize:          common.NullInt64(m.FileSize),
		Duration:          common.NullFloat64(float64(m.Duration)),
		Type:              m.Type,
		IsExtra:           common.BoolToInt64(m.IsExtra),
		FileHash:          common.NullString(m.FileHash),
		ContainerFormat:   common.NullString(m.ContainerFormat),
		Width:             common.NullInt64(int64(m.Width)),
		Height:            common.NullInt64(int64(m.Height)),
		AspectRatio:       common.NullString(media.CalculateAspectRatio(m.Width, m.Height)),
		Codec:             common.NullString(m.VideoCodec),
		AudioCodec:        common.NullString(m.AudioCodec),
		CodecProfile:      common.NullString(m.CodecProfile),
		BitRate:           common.NullInt64(m.Bitrate),
		FrameRate:         common.NullFloat64(m.FrameRate),
		ScanType:          common.NullString(m.ScanType),
		HdrFormat:         common.NullString(m.HDRFormat),
		ColorSpace:        common.NullString(m.ColorSpace),
		ColorPrimaries:    common.NullString(m.ColorPrimaries),
		ThumbnailPath:     sql.NullString{},
		SourceType:        common.NullString(media.DetectSourceType(m.FilePath)),
		ResolutionLabel:   common.NullString(media.CalculateResolutionLabelFromDimensions(m.Width, m.Height)),
		QualityScore:      sql.NullInt64{},
		Is3d:              common.NullInt64FromBool(func() bool { is3d, _ := media.Detect3D(m.FilePath); return is3d }()),
		StereoMode:        common.NullString(func() string { _, stereoMode := media.Detect3D(m.FilePath); return stereoMode }()),
		HasDash:           common.NullInt64FromBool(false),
		DashManifestPath:  sql.NullString{},
		TranscodingStatus: sql.NullString{},
	}
}

// buildPostgresCreateArtistParams builds CreateArtistParams for PostgreSQL from a domain Artist entity
func buildPostgresCreateArtistParams(a *media.Artist) sqlc_postgres.CreateArtistParams {
	return sqlc_postgres.CreateArtistParams{
		LibraryID:           a.LibraryID,
		Name:                a.Name,
		SortName:            common.NullString(a.SortName),
		MusicbrainzArtistID: common.NullString(a.MusicBrainzArtistID),
		Bio:                 common.NullString(a.Bio),
		Country:             common.NullString(a.Country),
		FormedYear:          common.NullInt64(int64(a.FormedYear)),
		Genre:               common.NullString(a.Genre),
		ImagePath:           common.NullString(a.ImagePath),
		CreatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		Directory:           common.NullString(a.Directory),
	}
}

// buildPostgresUpdateArtistParams builds UpdateArtistParams for PostgreSQL from a domain Artist entity
func buildPostgresUpdateArtistParams(a *media.Artist) sqlc_postgres.UpdateArtistParams {
	return sqlc_postgres.UpdateArtistParams{
		ID:                  a.ID,
		Name:                a.Name,
		SortName:            common.NullString(a.SortName),
		MusicbrainzArtistID: common.NullString(a.MusicBrainzArtistID),
		Bio:                 common.NullString(a.Bio),
		Country:             common.NullString(a.Country),
		FormedYear:          common.NullInt64(int64(a.FormedYear)),
		Genre:               common.NullString(a.Genre),
		ImagePath:           common.NullString(a.ImagePath),
		Directory:           common.NullString(a.Directory),
		UpdatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
}

// postgresMusicTrackRow is a generic interface for all PostgreSQL music track query row types
type postgresMusicTrackRow interface {
	sqlc_postgres.ListMusicTracksByLibraryRow |
		sqlc_postgres.ListMusicTracksByAlbumRow |
		sqlc_postgres.ListMusicTracksByAlbumIDRow |
		sqlc_postgres.ListMusicTracksByArtistRow |
		sqlc_postgres.SearchMusicTracksRow |
		sqlc_postgres.ListMusicTracksByLibraryPaginatedRow |
		sqlc_postgres.ListMusicTracksByLibraryPaginatedDescRow
}

// postgresMusicTrackRowToDomain converts any PostgreSQL music track row type to domain MusicTrack
func postgresMusicTrackRowToDomain[T postgresMusicTrackRow](row T) *media.MusicTrack {
	// Use generic mapper - no more unsafe.Pointer needed!
	return mapMusicTrackToDomain(row)
}

// convertPostgresRowsToMusicTracks converts a slice of PostgreSQL music track rows to domain MusicTrack entities
func convertPostgresRowsToMusicTracks[T postgresMusicTrackRow](rows []T) []*media.MusicTrack {
	tracks := make([]*media.MusicTrack, len(rows))
	for i, row := range rows {
		tracks[i] = postgresMusicTrackRowToDomain(row)
	}
	return tracks
}

// sqliteGenericMusicTrackRowToDomain converts any SQLite music track row type to domain MusicTrack
func sqliteGenericMusicTrackRowToDomain[T musicTrackRow](row T) *media.MusicTrack {
	fields := extractMusicTrackFields(row)
	return sqliteMusicTrackRowToDomain(fields)
}

// postgresAlbumRow is a generic interface for all PostgreSQL album query row types
type postgresAlbumRow interface {
	sqlc_postgres.ListAlbumsByLibraryPaginatedRow |
		sqlc_postgres.ListAlbumsByLibraryPaginatedDescRow
}

// postgresAlbumRowToDomain converts any PostgreSQL album row type to domain MusicAlbum
func postgresAlbumRowToDomain[T postgresAlbumRow](row T) media.MusicAlbum {
	// All row types have identical structures
	var r sqlc_postgres.ListAlbumsByLibraryPaginatedRow

	switch typed := any(row).(type) {
	case sqlc_postgres.ListAlbumsByLibraryPaginatedRow:
		r = typed
	case sqlc_postgres.ListAlbumsByLibraryPaginatedDescRow:
		r = *(*sqlc_postgres.ListAlbumsByLibraryPaginatedRow)(unsafe.Pointer(&typed))
	}

	return media.MusicAlbum{
		Album:       common.ParseNullString(r.Album),
		AlbumArtist: common.ParseNullString(r.AlbumArtist),
		Year:        common.ParseNullInt64(r.Year),
		TrackCount:  r.TrackCount,
		Duration:    r.TotalDuration,
	}
}

// sqliteAlbumRow is a generic interface for all SQLite album query row types
type sqliteAlbumRow interface {
	sqlc_sqlite.ListAlbumsByLibraryPaginatedRow |
		sqlc_sqlite.ListAlbumsByLibraryPaginatedDescRow
}

// sqliteAlbumRowToDomain converts any SQLite album row type to domain MusicAlbum
func sqliteAlbumRowToDomain[T sqliteAlbumRow](row T) media.MusicAlbum {
	// All row types have identical structures
	var r sqlc_sqlite.ListAlbumsByLibraryPaginatedRow

	switch typed := any(row).(type) {
	case sqlc_sqlite.ListAlbumsByLibraryPaginatedRow:
		r = typed
	case sqlc_sqlite.ListAlbumsByLibraryPaginatedDescRow:
		r = *(*sqlc_sqlite.ListAlbumsByLibraryPaginatedRow)(unsafe.Pointer(&typed))
	}

	return media.MusicAlbum{
		Album:       common.ParseNullString(r.Album),
		AlbumArtist: common.ParseNullString(r.AlbumArtist),
		Year:        common.ParseNullInt64(r.Year),
		TrackCount:  r.TrackCount,
		Duration:    int64(common.ParseNullFloat64(r.TotalDuration)),
	}
}
