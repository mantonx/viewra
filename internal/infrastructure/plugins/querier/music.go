package querier

import (
	"context"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// getMusicArtistDetailsDirectly fetches music artist details.
func (q *DBMediaQuerier) getMusicArtistDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetArtistByID(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_artist",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicArtistRowToDetails(result, externalIDs), nil
}

// getMusicAlbumDetailsDirectly fetches music album details.
func (q *DBMediaQuerier) getMusicAlbumDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetAlbumByID(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_album",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicAlbumRowToDetails(result, externalIDs), nil
}

// getMusicTrackDetailsDirectly fetches music track details.
func (q *DBMediaQuerier) getMusicTrackDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.querier.GetMusicTrackByMediaID(ctx, id)
	if err != nil {
		return &MediaDetailsInfo{
			ID:          id,
			MediaType:   "music_track",
			ExternalIDs: externalIDs,
		}, nil
	}

	return q.musicTrackRowToDetails(result, externalIDs), nil
}

// musicArtistRowToDetails converts a music artist row to details.
func (q *DBMediaQuerier) musicArtistRowToDetails(row unified.MusicArtist, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_artist",
		ExternalIDs: externalIDs,
		ID:          row.ID,
		Title:       row.Name,
		LibraryID:   row.LibraryID,
	}

	if row.Bio.Valid {
		info.Biography = row.Bio.String
	}
	if row.Country.Valid {
		info.Country = row.Country.String
	}
	if row.Genre.Valid {
		info.Genres = splitAndTrim(row.Genre.String)
	}

	return info
}

// musicAlbumRowToDetails converts a music album row to details.
func (q *DBMediaQuerier) musicAlbumRowToDetails(row unified.MusicAlbum, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_album",
		ExternalIDs: externalIDs,
		ID:          row.ID,
		Title:       row.Title,
		LibraryID:   row.LibraryID,
	}

	if row.Year.Valid {
		info.Year = int(row.Year.Int64)
	}
	if row.Genre.Valid {
		info.Genres = splitAndTrim(row.Genre.String)
	}
	if row.ReleaseType.Valid {
		info.ReleaseType = row.ReleaseType.String
	}

	return info
}

// musicTrackRowToDetails converts a music track row to details.
func (q *DBMediaQuerier) musicTrackRowToDetails(row unified.GetMusicTrackByMediaIDRow, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_track",
		ExternalIDs: externalIDs,
		ID:          row.MediaID,
		Title:       row.Title,
		LibraryID:   row.LibraryID,
	}

	if row.Artist.Valid {
		info.ArtistName = row.Artist.String
	}
	if row.Album.Valid {
		info.AlbumTitle = row.Album.String
	}
	if row.Genre.Valid {
		info.Genres = splitAndTrim(row.Genre.String)
	}

	return info
}

func (q *DBMediaQuerier) listMusicDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	// Music libraries contain tracks - list them with album/artist context
	results, err := q.querier.ListMusicTracksByLibraryPaginated(ctx, unified.ListMusicTracksByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := q.querier.CountMusicTracksByLibrary(ctx, libraryID)
	if err != nil {
		return nil, 0, err
	}

	return q.musicTrackRowsToDetails(results), int(total), nil
}

func (q *DBMediaQuerier) musicTrackRowsToDetails(results []unified.ListMusicTracksByLibraryPaginatedRow) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	for _, row := range results {
		info := &MediaDetailsInfo{
			ID:        row.MediaID,
			MediaType: "music_track",
			Title:     row.Title,
			LibraryID: row.LibraryID,
		}
		if row.Artist.Valid {
			info.ArtistName = row.Artist.String
		}
		if row.Album.Valid {
			info.AlbumTitle = row.Album.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		infos = append(infos, info)
	}

	return infos
}
