package querier

import (
	"context"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// getMusicArtistDetailsDirectly fetches music artist details.
func (q *DBMediaQuerier) getMusicArtistDetailsDirectly(ctx context.Context, id int64, externalIDs map[string]string) (*MediaDetailsInfo, error) {
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetArtistByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetArtistByID(ctx, id)
		},
	)
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
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetAlbumByID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetAlbumByID(ctx, id)
		},
	)
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
	result, err := q.router.Route(
		func() (any, error) {
			return q.postgres.GetMusicTrackByMediaID(ctx, int32(id))
		},
		func() (any, error) {
			return q.sqlite.GetMusicTrackByMediaID(ctx, id)
		},
	)
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
func (q *DBMediaQuerier) musicArtistRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_artist",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.MusicArtist)
		info.ID = int64(row.ID)
		info.Title = row.Name
		info.LibraryID = int64(row.LibraryID)
		if row.Bio.Valid {
			info.Biography = row.Bio.String
		}
		if row.Country.Valid {
			info.Country = row.Country.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	} else {
		row := result.(sqlc_sqlite.MusicArtist)
		info.ID = row.ID
		info.Title = row.Name
		info.LibraryID = row.LibraryID
		if row.Bio.Valid {
			info.Biography = row.Bio.String
		}
		if row.Country.Valid {
			info.Country = row.Country.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	}

	return info
}

// musicAlbumRowToDetails converts a music album row to details.
func (q *DBMediaQuerier) musicAlbumRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_album",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.MusicAlbum)
		info.ID = int64(row.ID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Year.Valid {
			info.Year = int(row.Year.Int32)
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.ReleaseType.Valid {
			info.ReleaseType = row.ReleaseType.String
		}
	} else {
		row := result.(sqlc_sqlite.MusicAlbum)
		info.ID = row.ID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Year.Valid {
			info.Year = int(row.Year.Int64)
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
		if row.ReleaseType.Valid {
			info.ReleaseType = row.ReleaseType.String
		}
	}

	return info
}

// musicTrackRowToDetails converts a music track row to details.
func (q *DBMediaQuerier) musicTrackRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
	info := &MediaDetailsInfo{
		MediaType:   "music_track",
		ExternalIDs: externalIDs,
	}

	if q.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetMusicTrackByMediaIDRow)
		info.ID = int64(row.MediaID)
		info.Title = row.Title
		info.LibraryID = int64(row.LibraryID)
		if row.Artist.Valid {
			info.ArtistName = row.Artist.String
		}
		if row.Album.Valid {
			info.AlbumTitle = row.Album.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	} else {
		row := result.(sqlc_sqlite.GetMusicTrackByMediaIDRow)
		info.ID = row.MediaID
		info.Title = row.Title
		info.LibraryID = row.LibraryID
		if row.Artist.Valid {
			info.ArtistName = row.Artist.String
		}
		if row.Album.Valid {
			info.AlbumTitle = row.Album.String
		}
		if row.Genre.Valid {
			info.Genres = splitAndTrim(row.Genre.String)
		}
	}

	return info
}

func (q *DBMediaQuerier) listMusicDetailsByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error) {
	// Music libraries contain tracks - list them with album/artist context
	results, err := q.router.Route(
		func() (any, error) {
			return q.postgres.ListMusicTracksByLibraryPaginated(ctx, sqlc_postgres.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
		},
		func() (any, error) {
			return q.sqlite.ListMusicTracksByLibraryPaginated(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(limit),
				Offset:    int64(offset),
			})
		},
	)
	if err != nil {
		return nil, 0, err
	}

	countResult, err := q.router.Route(
		func() (any, error) {
			return q.postgres.CountMusicTracksByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return q.sqlite.CountMusicTracksByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, 0, err
	}

	total := int(countResult.(int64))
	return q.musicTrackRowsToDetails(results), total, nil
}

func (q *DBMediaQuerier) musicTrackRowsToDetails(results any) []*MediaDetailsInfo {
	var infos []*MediaDetailsInfo

	if q.router.IsPostgresDB() {
		for _, row := range results.([]sqlc_postgres.ListMusicTracksByLibraryPaginatedRow) {
			info := &MediaDetailsInfo{
				ID:        int64(row.MediaID),
				MediaType: "music_track",
				Title:     row.Title,
				LibraryID: int64(row.LibraryID),
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
	} else {
		for _, row := range results.([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow) {
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
	}

	return infos
}
