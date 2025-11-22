package music

import (
	"database/sql"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
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
		// Basic music metadata
		Artist:      common.ParseNullString(row.Artist),
		Album:       common.ParseNullString(row.Album),
		AlbumArtist: common.ParseNullString(row.AlbumArtist),
		TrackNumber: int(common.ParseNullInt64(row.TrackNumber)),
		DiscNumber:  int(common.ParseNullInt64(row.DiscNumber)),
		Year:        int(common.ParseNullInt64(row.Year)),
		Genre:       common.ParseNullString(row.Genre),
		Composer:    common.ParseNullString(row.Composer),
		Publisher:   common.ParseNullString(row.RecordLabel),
		Bitrate:     int(common.ParseNullInt64(row.BitRate) / 1000), // Convert from bps to kbps
		// Extended metadata
		TotalTracks:         int(common.ParseNullInt64(row.TotalTracks)),
		TotalDiscs:          int(common.ParseNullInt64(row.TotalDiscs)),
		ReleaseDate:         common.FormatNullDate(row.ReleaseDate),
		Lyricist:            common.ParseNullString(row.Lyricist),
		ISRC:                common.ParseNullString(row.Isrc),
		ReleaseType:         common.ParseNullString(row.ReleaseType),
		Compilation:         common.ParseNullBool(row.Compilation),
		OriginalTitle:       common.ParseNullString(row.OriginalTitle),
		SortTitle:           common.ParseNullString(row.SortTitle),
		MusicBrainzTrackID:  common.ParseNullString(row.MusicbrainzTrackID),
		MusicBrainzAlbumID:  common.ParseNullString(row.MusicbrainzAlbumID),
		MusicBrainzArtistID: common.ParseNullString(row.MusicbrainzArtistID),
	}
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
		Compilation:         common.NullBool(t.Compilation),
		MusicbrainzTrackID:  common.NullString(t.MusicBrainzTrackID),
		MusicbrainzAlbumID:  common.NullString(t.MusicBrainzAlbumID),
		MusicbrainzArtistID: common.NullString(t.MusicBrainzArtistID),
		OriginalTitle:       common.NullString(t.OriginalTitle),
		SortTitle:           common.NullString(sortTitle),
		AlbumID:             common.NullInt64(t.AlbumID),
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
		MediaID:             t.Media.ID,
	}
}

// sqliteAlbumToDomain converts a SQLite album row to a domain Album entity
func sqliteAlbumToDomain(row sqlc_sqlite.MusicAlbum) *media.Album {
	var createdAt, updatedAt string
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Album{
		ID:                 row.ID,
		LibraryID:          row.LibraryID,
		Title:              row.Title,
		AlbumArtist:        common.ParseNullString(row.AlbumArtist),
		Artist:             common.ParseNullString(row.Artist),
		Year:               int(common.ParseNullInt64(row.Year)),
		ReleaseDate:        common.FormatNullDate(row.ReleaseDate),
		Genre:              common.ParseNullString(row.Genre),
		TotalTracks:        int(common.ParseNullInt64(row.TotalTracks)),
		TotalDiscs:         int(common.ParseNullInt64(row.TotalDiscs)),
		RecordLabel:        common.ParseNullString(row.RecordLabel),
		ReleaseType:        common.ParseNullString(row.ReleaseType),
		Compilation:        common.ParseNullBool(row.Compilation),
		MusicBrainzAlbumID: common.ParseNullString(row.MusicbrainzAlbumID),
		CoverArtPath:       common.ParseNullString(row.CoverArtPath),
		SortTitle:          common.ParseNullString(row.SortTitle),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
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
		Compilation:        common.NullBool(a.Compilation),
		MusicbrainzAlbumID: common.NullString(a.MusicBrainzAlbumID),
		CoverArtPath:       common.NullString(a.CoverArtPath),
		SortTitle:          common.NullString(sortTitle),
		CreatedAt:          now,
		UpdatedAt:          now,
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
		UpdatedAt:          sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:                 a.ID,
	}
}

// sqliteArtistToDomain converts a SQLite artist row to a domain Artist entity
func sqliteArtistToDomain(row sqlc_sqlite.MusicArtist) *media.Artist {
	var createdAt, updatedAt string
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Artist{
		ID:                  row.ID,
		LibraryID:           row.LibraryID,
		Name:                row.Name,
		SortName:            common.ParseNullString(row.SortName),
		MusicBrainzArtistID: common.ParseNullString(row.MusicbrainzArtistID),
		Bio:                 common.ParseNullString(row.Bio),
		Country:             common.ParseNullString(row.Country),
		FormedYear:          int(common.ParseNullInt64(row.FormedYear)),
		Genre:               common.ParseNullString(row.Genre),
		ImagePath:           common.ParseNullString(row.ImagePath),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}
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
		UpdatedAt:           sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:                  a.ID,
	}
}
