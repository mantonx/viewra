package music

import (
	"database/sql"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// ========================================
// Unified Row Mappers
// ========================================
// Since PostgreSQL and SQLite types are now structurally identical,
// we use a single converter per query type using unified type aliases.

// musicTrackRowToDomain converts a GetMusicTrackByMediaIDRow to domain MusicTrack
func musicTrackRowToDomain(row unified.GetMusicTrackByMediaIDRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// listMusicTrackRowToDomain converts a ListMusicTracksByLibraryRow to domain MusicTrack
func listMusicTrackRowToDomain(row unified.ListMusicTracksByLibraryRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// albumMusicTrackRowToDomain converts a ListMusicTracksByAlbumRow to domain MusicTrack
func albumMusicTrackRowToDomain(row unified.ListMusicTracksByAlbumRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// albumIDMusicTrackRowToDomain converts a ListMusicTracksByAlbumIDRow to domain MusicTrack
func albumIDMusicTrackRowToDomain(row unified.ListMusicTracksByAlbumIDRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// artistMusicTrackRowToDomain converts a ListMusicTracksByArtistRow to domain MusicTrack
func artistMusicTrackRowToDomain(row unified.ListMusicTracksByArtistRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// searchMusicTrackRowToDomain converts a SearchMusicTracksRow to domain MusicTrack
func searchMusicTrackRowToDomain(row unified.SearchMusicTracksRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// paginatedMusicTrackRowToDomain converts a ListMusicTracksByLibraryPaginatedRow to domain MusicTrack
func paginatedMusicTrackRowToDomain(row unified.ListMusicTracksByLibraryPaginatedRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// paginatedDescMusicTrackRowToDomain converts a ListMusicTracksByLibraryPaginatedDescRow to domain MusicTrack
func paginatedDescMusicTrackRowToDomain(row unified.ListMusicTracksByLibraryPaginatedDescRow) *media.MusicTrack {
	return toMusicTrackDomain(
		row.MediaID_2, row.LibraryID, row.Title, row.Type, row.FilePath,
		row.FileSize, row.ContainerFormat, row.Duration, row.Width, row.Height,
		row.Codec, row.AudioCodec, row.BitRate, row.FrameRate, row.IsExtra != 0,
		row.CreatedAt, row.UpdatedAt,
		row.Artist, row.Album, row.AlbumArtist, row.TrackNumber, row.DiscNumber,
		row.TotalTracks, row.TotalDiscs, row.Genre, row.Year, row.ReleaseDate,
		row.Composer, row.Lyricist, row.RecordLabel, row.Isrc, row.ReleaseType,
		row.Compilation, row.OriginalTitle, row.SortTitle,
		row.MusicbrainzTrackID, row.MusicbrainzAlbumID, row.MusicbrainzArtistID,
		row.AlbumID, row.ArtistID,
	)
}

// toMusicTrackDomain is the single source of truth for converting database fields to domain MusicTrack.
// All row-specific converters delegate to this function.
func toMusicTrackDomain(
	// Media fields
	mediaID, libraryID int64, title, mediaType, filePath string,
	fileSize sql.NullInt64, containerFormat sql.NullString, duration sql.NullFloat64,
	width, height sql.NullInt64, codec, audioCodec sql.NullString,
	bitRate sql.NullInt64, frameRate sql.NullFloat64, isExtra bool,
	createdAt, updatedAt sql.NullTime,
	// Music-specific fields
	artist, album, albumArtist sql.NullString,
	trackNumber, discNumber, totalTracks, totalDiscs sql.NullInt64,
	genre sql.NullString, year sql.NullInt64, releaseDate sql.NullTime,
	composer, lyricist, recordLabel, isrc, releaseType sql.NullString,
	compilation sql.NullInt64, originalTitle, sortTitle sql.NullString,
	musicbrainzTrackID, musicbrainzAlbumID, musicbrainzArtistID sql.NullString,
	albumID, artistID sql.NullInt64,
) *media.MusicTrack {
	return &media.MusicTrack{
		Media: media.Media{
			ID:              mediaID,
			LibraryID:       libraryID,
			Title:           title,
			Type:            mediaType,
			FilePath:        filePath,
			FileSize:        common.ParseNullInt64(fileSize),
			Duration:        int(common.ParseNullFloat64(duration)),
			IsExtra:         isExtra,
			Width:           int(common.ParseNullInt64(width)),
			Height:          int(common.ParseNullInt64(height)),
			VideoCodec:      common.ParseNullString(codec),
			AudioCodec:      common.ParseNullString(audioCodec),
			Bitrate:         common.ParseNullInt64(bitRate),
			FrameRate:       common.ParseNullFloat64(frameRate),
			ContainerFormat: common.ParseNullString(containerFormat),
			CreatedAt:       common.ParseNullTime(createdAt),
			UpdatedAt:       common.ParseNullTime(updatedAt),
		},
		Artist:              common.ParseNullString(artist),
		Album:               common.ParseNullString(album),
		AlbumArtist:         common.ParseNullString(albumArtist),
		TrackNumber:         int(common.ParseNullInt64(trackNumber)),
		DiscNumber:          int(common.ParseNullInt64(discNumber)),
		TotalTracks:         int(common.ParseNullInt64(totalTracks)),
		TotalDiscs:          int(common.ParseNullInt64(totalDiscs)),
		Genre:               common.ParseNullString(genre),
		Year:                int(common.ParseNullInt64(year)),
		ReleaseDate:         common.FormatNullDate(releaseDate),
		Composer:            common.ParseNullString(composer),
		Lyricist:            common.ParseNullString(lyricist),
		Publisher:           common.ParseNullString(recordLabel),
		ISRC:                common.ParseNullString(isrc),
		ReleaseType:         common.ParseNullString(releaseType),
		Compilation:         compilation.Valid && compilation.Int64 != 0,
		OriginalTitle:       common.ParseNullString(originalTitle),
		SortTitle:           common.ParseNullString(sortTitle),
		MusicBrainzTrackID:  common.ParseNullString(musicbrainzTrackID),
		MusicBrainzAlbumID:  common.ParseNullString(musicbrainzAlbumID),
		MusicBrainzArtistID: common.ParseNullString(musicbrainzArtistID),
		Bitrate:             int(common.ParseNullInt64(bitRate) / 1000), // Convert from bps to kbps
		AlbumID:             common.ParseNullInt64(albumID),
		ArtistID:            common.ParseNullInt64(artistID),
	}
}

// ========================================
// Album Row Mappers
// ========================================

// albumToDomain converts a MusicAlbum to domain Album
func albumToDomain(row unified.MusicAlbum) *media.Album {
	var createdAtStr, updatedAtStr string
	if row.CreatedAt.Valid {
		createdAtStr = row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if row.UpdatedAt.Valid {
		updatedAtStr = row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Album{
		ID:                 row.ID,
		LibraryID:          row.LibraryID,
		ArtistID:           common.ParseNullInt64(row.ArtistID),
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
		Compilation:        row.Compilation.Valid && row.Compilation.Int64 != 0,
		MusicBrainzAlbumID: common.ParseNullString(row.MusicbrainzAlbumID),
		CoverArtPath:       common.ParseNullString(row.CoverArtPath),
		SortTitle:          common.ParseNullString(row.SortTitle),
		Directory:          common.ParseNullString(row.Directory),
		CreatedAt:          createdAtStr,
		UpdatedAt:          updatedAtStr,
	}
}

// listAlbumRowToDomain converts a ListAlbumsByLibraryPaginatedRow to domain MusicAlbum
func listAlbumRowToDomain(row unified.ListAlbumsByLibraryPaginatedRow) media.MusicAlbum {
	return media.MusicAlbum{
		Album:       common.ParseNullString(row.Album),
		AlbumArtist: common.ParseNullString(row.AlbumArtist),
		Year:        common.ParseNullInt64(row.Year),
		TrackCount:  row.TrackCount,
		Duration:    row.TotalDuration,
	}
}

// listAlbumDescRowToDomain converts a ListAlbumsByLibraryPaginatedDescRow to domain MusicAlbum
func listAlbumDescRowToDomain(row unified.ListAlbumsByLibraryPaginatedDescRow) media.MusicAlbum {
	return media.MusicAlbum{
		Album:       common.ParseNullString(row.Album),
		AlbumArtist: common.ParseNullString(row.AlbumArtist),
		Year:        common.ParseNullInt64(row.Year),
		TrackCount:  row.TrackCount,
		Duration:    row.TotalDuration,
	}
}

// ========================================
// Artist Row Mappers
// ========================================

// artistToDomain converts a MusicArtist to domain Artist
func artistToDomain(row unified.MusicArtist) *media.Artist {
	var createdAtStr, updatedAtStr string
	if row.CreatedAt.Valid {
		createdAtStr = row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if row.UpdatedAt.Valid {
		updatedAtStr = row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &media.Artist{
		ID:                  row.ID,
		LibraryID:           row.LibraryID,
		Name:                row.Name,
		SortName:            common.ParseNullString(row.SortName),
		Bio:                 common.ParseNullString(row.Bio),
		Country:             common.ParseNullString(row.Country),
		FormedYear:          int(common.ParseNullInt64(row.FormedYear)),
		Genre:               common.ParseNullString(row.Genre),
		MusicBrainzArtistID: common.ParseNullString(row.MusicbrainzArtistID),
		ImagePath:           common.ParseNullString(row.ImagePath),
		Directory:           common.ParseNullString(row.Directory),
		CreatedAt:           createdAtStr,
		UpdatedAt:           updatedAtStr,
	}
}

// artistWithCountsRowToDomain converts a GetArtistsWithCountsByLibraryPaginatedRow to domain MusicArtist
func artistWithCountsRowToDomain(row unified.GetArtistsWithCountsByLibraryPaginatedRow) media.MusicArtist {
	return media.MusicArtist{
		RepresentativeID: row.ID,
		Artist:           row.Name,
		AlbumCount:       row.AlbumCount,
		TrackCount:       row.TrackCount,
	}
}

// artistWithCountsDescRowToDomain converts a GetArtistsWithCountsByLibraryPaginatedDescRow to domain MusicArtist
func artistWithCountsDescRowToDomain(row unified.GetArtistsWithCountsByLibraryPaginatedDescRow) media.MusicArtist {
	return media.MusicArtist{
		RepresentativeID: row.ID,
		Artist:           row.Name,
		AlbumCount:       row.AlbumCount,
		TrackCount:       row.TrackCount,
	}
}

// searchArtistWithCountsRowToDomain converts a SearchArtistsWithCountsByNamePaginatedRow to ArtistWithCounts
func searchArtistWithCountsRowToDomain(row unified.SearchArtistsWithCountsByNamePaginatedRow) ArtistWithCounts {
	return ArtistWithCounts{
		ID:         row.ID,
		LibraryID:  row.LibraryID,
		Name:       row.Name,
		AlbumCount: int(row.AlbumCount),
		TrackCount: int(row.TrackCount),
	}
}

// artistWithCountsToInternal converts a GetArtistsWithCountsByLibraryPaginatedRow to ArtistWithCounts
func artistWithCountsToInternal(row unified.GetArtistsWithCountsByLibraryPaginatedRow) ArtistWithCounts {
	return ArtistWithCounts{
		ID:         row.ID,
		LibraryID:  row.LibraryID,
		Name:       row.Name,
		AlbumCount: int(row.AlbumCount),
		TrackCount: int(row.TrackCount),
	}
}

// artistWithCountsDescToInternal converts a GetArtistsWithCountsByLibraryPaginatedDescRow to ArtistWithCounts
func artistWithCountsDescToInternal(row unified.GetArtistsWithCountsByLibraryPaginatedDescRow) ArtistWithCounts {
	return ArtistWithCounts{
		ID:         row.ID,
		LibraryID:  row.LibraryID,
		Name:       row.Name,
		AlbumCount: int(row.AlbumCount),
		TrackCount: int(row.TrackCount),
	}
}

// ========================================
// Unified Param Builders
// ========================================

// buildCreateMusicTrackParams builds CreateMusicTrackParams from a domain MusicTrack entity
func buildCreateMusicTrackParams(t *media.MusicTrack) unified.CreateMusicTrackParams {
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

	return unified.CreateMusicTrackParams{
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

// buildUpdateMusicTrackParams builds UpdateMusicTrackParams from a domain MusicTrack entity
func buildUpdateMusicTrackParams(t *media.MusicTrack) unified.UpdateMusicTrackParams {
	params := buildCreateMusicTrackParams(t)
	return unified.UpdateMusicTrackParams{
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

// buildCreateAlbumParams builds CreateAlbumParams from a domain Album entity
func buildCreateAlbumParams(a *media.Album) unified.CreateAlbumParams {
	sortTitle := a.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(a.Title)
	}

	now := sql.NullTime{Time: time.Now().UTC(), Valid: true}

	return unified.CreateAlbumParams{
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

// buildUpdateAlbumParams builds UpdateAlbumParams from a domain Album entity
func buildUpdateAlbumParams(a *media.Album) unified.UpdateAlbumParams {
	params := buildCreateAlbumParams(a)
	return unified.UpdateAlbumParams{
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

// buildCreateArtistParams builds CreateArtistParams from a domain Artist entity
func buildCreateArtistParams(a *media.Artist) unified.CreateArtistParams {
	return unified.CreateArtistParams{
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

// buildUpdateArtistParams builds UpdateArtistParams from a domain Artist entity
func buildUpdateArtistParams(a *media.Artist) unified.UpdateArtistParams {
	return unified.UpdateArtistParams{
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

// buildCreateMediaParams builds CreateMediaParams from a domain Media entity
func buildCreateMediaParams(m *media.Media) unified.CreateMediaParams {
	return unified.CreateMediaParams{
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

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
