package music

import (
	"database/sql"

	domainCommon "github.com/viewra/viewra/internal/domain/common"
	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// sqliteMusicTrackToDomain converts a SQLite music track query row to a domain MusicTrack entity
func sqliteMusicTrackToDomain(row sqlc_sqlite.GetMusicTrackByMediaIDRow) *media.MusicTrack {
	return &media.MusicTrack{
		Media: media.Media{
			ID:              row.MediaID_2,
			LibraryID:       row.LibraryID,
			Title:           row.Title,
			Type:            row.Type,
			FilePath:        row.FilePath,
			FileSize:        common.ParseNullInt64(row.FileSize),
			Duration:        int(common.ParseNullFloat64(row.Duration)),
			IsExtra:         row.IsExtra,
			Width:           int(common.ParseNullInt64(row.Width)),
			Height:          int(common.ParseNullInt64(row.Height)),
			VideoCodec:      common.ParseNullString(row.Codec),
			AudioCodec:      common.ParseNullString(row.AudioCodec),
			Bitrate:         common.ParseNullInt64(row.BitRate),
			FrameRate:       common.ParseNullFloat64(row.FrameRate),
			ContainerFormat: common.ParseNullString(row.ContainerFormat),
			CreatedAt:       common.ParseNullTime(row.CreatedAt),
			UpdatedAt:       common.ParseNullTime(row.UpdatedAt),
		},
		Artist:      common.ParseNullString(row.Artist),
		Album:       common.ParseNullString(row.Album),
		TrackNumber: int(common.ParseNullInt64(row.TrackNumber)),
		Year:        int(common.ParseNullInt64(row.Year)),
		Genre:       common.ParseNullString(row.Genre),
		Composer:    common.ParseNullString(row.Composer),
		Publisher:   "", // TODO: Add Publisher field to database schema
		Bitrate:     int(common.ParseNullInt64(row.BitRate) / 1000), // Convert from bps to kbps
	}
}

// musicTrackFields holds all the fields needed to construct a MusicTrack from various query row types
type musicTrackFields struct {
	MediaID        int64
	Artist         sql.NullString
	Album          sql.NullString
	AlbumArtist    sql.NullString
	TrackNumber    sql.NullInt64
	DiscNumber     sql.NullInt64
	Genre          sql.NullString
	Year           sql.NullInt64
	Composer       sql.NullString
	MediaID2       int64
	LibraryID      int64
	Title          string
	FilePath       string
	FileSize       sql.NullInt64
	Duration       sql.NullFloat64
	Width          sql.NullInt64
	Height         sql.NullInt64
	Codec          sql.NullString
	AudioCodec     sql.NullString
	BitRate        sql.NullInt64
	FrameRate      sql.NullFloat64
	ContainerFormat sql.NullString
	Type           string
	IsExtra        bool
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
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
		Artist:      common.ParseNullString(fields.Artist),
		Album:       common.ParseNullString(fields.Album),
		TrackNumber: int(common.ParseNullInt64(fields.TrackNumber)),
		Year:        int(common.ParseNullInt64(fields.Year)),
		Genre:       common.ParseNullString(fields.Genre),
		Composer:    common.ParseNullString(fields.Composer),
		Publisher:   "", // TODO: Add Publisher field to database schema
		Bitrate:     int(common.ParseNullInt64(fields.BitRate) / 1000), // Convert from bps to kbps
	}
}

// buildSQLiteCreateMusicTrackParams builds CreateMusicTrackParams for SQLite from a domain MusicTrack entity
func buildSQLiteCreateMusicTrackParams(t *media.MusicTrack) sqlc_sqlite.CreateMusicTrackParams {
	return sqlc_sqlite.CreateMusicTrackParams{
		MediaID:             t.Media.ID,
		Artist:              common.NullString(t.Artist),
		Album:               common.NullString(t.Album),
		AlbumArtist:         common.NullString(t.Artist), // Default to track artist if no album artist
		TrackNumber:         common.NullInt64(int64(t.TrackNumber)),
		DiscNumber:          sql.NullInt64{},                              // TODO: Add DiscNumber to domain.MusicTrack
		TotalTracks:         sql.NullInt64{},                              // TODO: Add TotalTracks to domain.MusicTrack
		TotalDiscs:          sql.NullInt64{},                              // TODO: Add TotalDiscs to domain.MusicTrack
		Genre:               common.NullString(t.Genre),
		Year:                common.NullInt64(int64(t.Year)),
		ReleaseDate:         sql.NullTime{},                               // TODO: Add ReleaseDate to domain.MusicTrack
		Composer:            common.NullString(t.Composer),
		Lyricist:            sql.NullString{},                             // TODO: Add Lyricist to domain.MusicTrack
		RecordLabel:         common.NullString(t.Publisher),
		Isrc:                sql.NullString{},                             // TODO: Add ISRC to domain.MusicTrack
		ReleaseType:         sql.NullString{},                             // TODO: Add ReleaseType to domain.MusicTrack
		Compilation:         sql.NullBool{},                               // TODO: Add Compilation to domain.MusicTrack
		MusicbrainzTrackID:  sql.NullString{},                             // TODO: Add MusicBrainz IDs to domain.MusicTrack
		MusicbrainzAlbumID:  sql.NullString{},
		MusicbrainzArtistID: sql.NullString{},
		OriginalTitle:       sql.NullString{},                             // TODO: Add OriginalTitle to domain.MusicTrack
		SortTitle:           common.NullString(domainCommon.NormalizeSortTitle(t.Media.Title)),
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
		MediaID:             t.Media.ID,
	}
}
