package music

import (
	"context"
	"fmt"
	"unsafe"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.MusicRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
	mediaRepo media.Repository
}

// NewRepository creates a new music repository with the specified database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *common.BaseRepository, mediaRepo media.Repository) *Repository {
	return &Repository{
		BaseRepository: db,
		mediaRepo:      mediaRepo,
	}
}

// CreateMusicTrack adds a new music track to the repository
func (r *Repository) CreateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	// First, create the base media record
	if err := r.mediaRepo.Create(ctx, &track.Media); err != nil {
		return fmt.Errorf("failed to create base media record: %w", err)
	}

	// Then, create the music track-specific record
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().CreateMusicTrack(ctx, buildPostgresCreateMusicTrackParams(track))
		},
		func() error {
			return r.SQLite().CreateMusicTrack(ctx, buildSQLiteCreateMusicTrackParams(track))
		},
	)
}

// GetMusicTrackByID retrieves a music track by its media ID
func (r *Repository) GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.GetMusicTrackByMediaIDRow, error) {
			return r.Postgres().GetMusicTrackByMediaID(ctx, int32(id))
		},
		func() (sqlc_sqlite.GetMusicTrackByMediaIDRow, error) {
			return r.SQLite().GetMusicTrackByMediaID(ctx, id)
		},
		postgresMusicTrackToDomain,
		sqliteMusicTrackToDomain,
	)
}

// ListMusicTracksByLibrary retrieves all music tracks in a specific library
func (r *Repository) ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*media.MusicTrack, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMusicTracksByLibraryRow, error) {
			return r.Postgres().ListMusicTracksByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.ListMusicTracksByLibraryRow, error) {
			return r.SQLite().ListMusicTracksByLibrary(ctx, libraryID)
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByLibraryRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByLibraryRow],
	)
}

// ListMusicTracksByAlbum retrieves all tracks from a specific album
func (r *Repository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMusicTracksByAlbumRow, error) {
			return r.Postgres().ListMusicTracksByAlbum(ctx, sqlc_postgres.ListMusicTracksByAlbumParams{
				LibraryID: int32(libraryID),
				Album:     common.NullString(album),
			})
		},
		func() ([]sqlc_sqlite.ListMusicTracksByAlbumRow, error) {
			return r.SQLite().ListMusicTracksByAlbum(ctx, sqlc_sqlite.ListMusicTracksByAlbumParams{
				LibraryID: libraryID,
				Album:     common.NullString(album),
			})
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByAlbumRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByAlbumRow],
	)
}

// ListMusicTracksByArtist retrieves all tracks by a specific artist
func (r *Repository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	// Use LIKE pattern for artist search
	searchPattern := "%" + artist + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMusicTracksByArtistRow, error) {
			return r.Postgres().ListMusicTracksByArtist(ctx, sqlc_postgres.ListMusicTracksByArtistParams{
				LibraryID:   int32(libraryID),
				Artist:      common.NullString(searchPattern),
				AlbumArtist: common.NullString(searchPattern),
			})
		},
		func() ([]sqlc_sqlite.ListMusicTracksByArtistRow, error) {
			return r.SQLite().ListMusicTracksByArtist(ctx, sqlc_sqlite.ListMusicTracksByArtistParams{
				LibraryID:   libraryID,
				Artist:      common.NullString(searchPattern),
				AlbumArtist: common.NullString(searchPattern),
			})
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByArtistRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByArtistRow],
	)
}

// UpdateMusicTrack modifies an existing music track
func (r *Repository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &track.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Then, update the music track-specific record
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateMusicTrack(ctx, buildPostgresUpdateMusicTrackParams(track))
		},
		func() error {
			return r.SQLite().UpdateMusicTrack(ctx, buildSQLiteUpdateMusicTrackParams(track))
		},
	)
}

// SearchMusicTracks searches for music tracks by title, artist, or album
func (r *Repository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.SearchMusicTracksRow, error) {
			return r.Postgres().SearchMusicTracks(ctx, sqlc_postgres.SearchMusicTracksParams{
				LibraryID: int32(libraryID),
				Title:     searchPattern,
				Artist:    common.NullString(searchPattern),
				Album:     common.NullString(searchPattern),
			})
		},
		func() ([]sqlc_sqlite.SearchMusicTracksRow, error) {
			return r.SQLite().SearchMusicTracks(ctx, sqlc_sqlite.SearchMusicTracksParams{
				LibraryID: libraryID,
				Title:     searchPattern,
				Artist:    common.NullString(searchPattern),
				Album:     common.NullString(searchPattern),
			})
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.SearchMusicTracksRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.SearchMusicTracksRow],
	)
}

// musicTrackRow is a generic interface for all music track query row types.
// This allows us to write a single conversion function for all row types.
type musicTrackRow interface {
	sqlc_sqlite.ListMusicTracksByLibraryRow |
		sqlc_sqlite.ListMusicTracksByAlbumRow |
		sqlc_sqlite.ListMusicTracksByAlbumIDRow |
		sqlc_sqlite.ListMusicTracksByArtistRow |
		sqlc_sqlite.SearchMusicTracksRow |
		sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow |
		sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescRow
}

// extractMusicTrackFields extracts common fields from any music track row type.
// All four row types have identical structures, so we consolidate the conversion logic
// to eliminate duplication.
func extractMusicTrackFields[T musicTrackRow](row T) musicTrackFields {
	// All row types have identical structures, so we can convert to any one
	// and use it. We convert to ListMusicTracksByLibraryRow arbitrarily.
	var r sqlc_sqlite.ListMusicTracksByLibraryRow

	switch typed := any(row).(type) {
	case sqlc_sqlite.ListMusicTracksByLibraryRow:
		r = typed
	case sqlc_sqlite.ListMusicTracksByAlbumRow:
		// Cast via unsafe - safe because structures are identical
		r = *(*sqlc_sqlite.ListMusicTracksByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_sqlite.ListMusicTracksByAlbumIDRow:
		// Cast via unsafe - safe because structures are identical
		r = *(*sqlc_sqlite.ListMusicTracksByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_sqlite.ListMusicTracksByArtistRow:
		// Cast via unsafe - safe because structures are identical
		r = *(*sqlc_sqlite.ListMusicTracksByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_sqlite.SearchMusicTracksRow:
		// Cast via unsafe - safe because structures are identical
		r = *(*sqlc_sqlite.ListMusicTracksByLibraryRow)(unsafe.Pointer(&typed))
	case sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow:
		// Cast via unsafe - safe because structures are identical
		r = *(*sqlc_sqlite.ListMusicTracksByLibraryRow)(unsafe.Pointer(&typed))
	}

	return musicTrackFields{
		MediaID:         r.MediaID,
		Artist:          r.Artist,
		Album:           r.Album,
		AlbumArtist:     r.AlbumArtist,
		TrackNumber:     r.TrackNumber,
		DiscNumber:      r.DiscNumber,
		Genre:           r.Genre,
		Year:            r.Year,
		Composer:        r.Composer,
		MediaID2:        r.MediaID_2,
		LibraryID:       r.LibraryID,
		Title:           r.Title,
		FilePath:        r.FilePath,
		FileSize:        r.FileSize,
		Duration:        r.Duration,
		Width:           r.Width,
		Height:          r.Height,
		Codec:           r.Codec,
		AudioCodec:      r.AudioCodec,
		BitRate:         r.BitRate,
		FrameRate:       r.FrameRate,
		ContainerFormat: r.ContainerFormat,
		Type:            r.Type,
		IsExtra:         r.IsExtra,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// CountArtistsByLibrary returns the total count of artists in a library
func (r *Repository) CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountArtistsInLibrary(ctx, int32(libraryID))
		},
		func() (int64, error) {
			return r.SQLite().CountArtistsInLibrary(ctx, libraryID)
		},
	)
}

// ListArtistsByLibraryPaginated retrieves artists from music_artists table with pagination
// NOTE: This now queries proper artist entities. The music_artists table is currently empty
// and will be populated by the scanner when it's updated to create artist entities.
func (r *Repository) ListArtistsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]media.MusicArtist, error) {
	// Return empty list until scanner populates music_artists table
	// TODO: Once scanner is updated, implement proper pagination query
	return []media.MusicArtist{}, nil
}

// CountAlbumsByLibrary returns the total count of unique albums in a library
func (r *Repository) CountAlbumsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountAlbumsByLibrary(ctx, int32(libraryID))
		},
		func() (int64, error) {
			return r.SQLite().CountAlbumsByLibrary(ctx, libraryID)
		},
	)
}

// ListAlbumsByLibraryPaginated retrieves unique albums in a library with pagination
func (r *Repository) ListAlbumsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]media.MusicAlbum, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]sqlc_postgres.ListAlbumsByLibraryPaginatedDescRow, error) {
				return r.Postgres().ListAlbumsByLibraryPaginatedDesc(ctx, sqlc_postgres.ListAlbumsByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
			},
			func() ([]sqlc_sqlite.ListAlbumsByLibraryPaginatedDescRow, error) {
				return r.SQLite().ListAlbumsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListAlbumsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			postgresAlbumRowToDomain[sqlc_postgres.ListAlbumsByLibraryPaginatedDescRow],
			sqliteAlbumRowToDomain[sqlc_sqlite.ListAlbumsByLibraryPaginatedDescRow],
		)
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListAlbumsByLibraryPaginatedRow, error) {
			return r.Postgres().ListAlbumsByLibraryPaginated(ctx, sqlc_postgres.ListAlbumsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.ListAlbumsByLibraryPaginatedRow, error) {
			return r.SQLite().ListAlbumsByLibraryPaginated(ctx, sqlc_sqlite.ListAlbumsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		postgresAlbumRowToDomain[sqlc_postgres.ListAlbumsByLibraryPaginatedRow],
		sqliteAlbumRowToDomain[sqlc_sqlite.ListAlbumsByLibraryPaginatedRow],
	)
}

// CountMusicTracksByLibrary returns the total count of music tracks in a library
func (r *Repository) CountMusicTracksByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return common.QueryScalar(
		r.BaseRepository, ctx,
		func() (int64, error) {
			return r.Postgres().CountMusicTracksByLibrary(ctx, int32(libraryID))
		},
		func() (int64, error) {
			return r.SQLite().CountMusicTracksByLibrary(ctx, libraryID)
		},
	)
}

// ListMusicTracksByLibraryPaginated retrieves music tracks in a library with pagination
func (r *Repository) ListMusicTracksByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]*media.MusicTrack, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		return common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]sqlc_postgres.ListMusicTracksByLibraryPaginatedDescRow, error) {
				return r.Postgres().ListMusicTracksByLibraryPaginatedDesc(ctx, sqlc_postgres.ListMusicTracksByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
			},
			func() ([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescRow, error) {
				return r.SQLite().ListMusicTracksByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByLibraryPaginatedDescRow],
			sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescRow],
		)
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMusicTracksByLibraryPaginatedRow, error) {
			return r.Postgres().ListMusicTracksByLibraryPaginated(ctx, sqlc_postgres.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
		},
		func() ([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow, error) {
			return r.SQLite().ListMusicTracksByLibraryPaginated(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByLibraryPaginatedRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow],
	)
}

// ListArtistIDsByLibraryPaginated retrieves only artist representative IDs in a library with pagination
func (r *Repository) ListArtistIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]int64, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Determine sort order from pagination params
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		pgResults, err := common.QueryMany(
			r.BaseRepository, ctx,
			func() ([]int32, error) {
				return r.Postgres().ListArtistIDsByLibraryPaginatedDesc(ctx, sqlc_postgres.ListArtistIDsByLibraryPaginatedDescParams{
					LibraryID: int32(libraryID),
					Limit:     int32(pagination.Limit),
					Offset:    int32(pagination.Offset),
				})
			},
			func() ([]int64, error) {
				return r.SQLite().ListArtistIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListArtistIDsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			},
			func(id int32) int64 { return int64(id) },
			func(id int64) int64 { return id },
		)
		return pgResults, err
	}

	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]int32, error) {
			return r.Postgres().ListArtistIDsByLibraryPaginated(ctx, sqlc_postgres.ListArtistIDsByLibraryPaginatedParams{
				LibraryID: int32(libraryID),
				Limit:     int32(pagination.Limit),
				Offset:    int32(pagination.Offset),
			})
		},
		func() ([]int64, error) {
			return r.SQLite().ListArtistIDsByLibraryPaginated(ctx, sqlc_sqlite.ListArtistIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
		func(id int32) int64 { return int64(id) },
		func(id int64) int64 { return id },
	)
}

// CreateAlbum creates a new album entity
func (r *Repository) CreateAlbum(ctx context.Context, album *media.Album) error {
	pgAlbum, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicAlbum, error) {
			return r.Postgres().CreateAlbum(ctx, buildPostgresCreateAlbumParams(album))
		},
		func() (sqlc_sqlite.MusicAlbum, error) {
			return r.SQLite().CreateAlbum(ctx, buildSQLiteCreateAlbumParams(album))
		},
		func(row sqlc_postgres.MusicAlbum) *media.Album {
			album.ID = int64(row.ID)
			return album
		},
		func(row sqlc_sqlite.MusicAlbum) *media.Album {
			album.ID = row.ID
			return album
		},
	)
	if err != nil {
		return err
	}
	album.ID = pgAlbum.ID
	return nil
}

// GetAlbumByID retrieves an album by its ID
func (r *Repository) GetAlbumByID(ctx context.Context, id int64) (*media.Album, error) {
	album, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicAlbum, error) {
			return r.Postgres().GetAlbumByID(ctx, int32(id))
		},
		func() (sqlc_sqlite.MusicAlbum, error) {
			return r.SQLite().GetAlbumByID(ctx, id)
		},
		postgresAlbumToDomain,
		sqliteAlbumToDomain,
	)
	return album, r.ConvertNotFoundError(err)
}

// FindAlbumByTitle finds an album by library, title, and album artist
func (r *Repository) FindAlbumByTitle(ctx context.Context, libraryID int64, title, albumArtist string) (*media.Album, error) {
	album, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicAlbum, error) {
			return r.Postgres().FindAlbumByTitle(ctx, sqlc_postgres.FindAlbumByTitleParams{
				LibraryID:   int32(libraryID),
				Title:       title,
				AlbumArtist: common.NullString(albumArtist),
			})
		},
		func() (sqlc_sqlite.MusicAlbum, error) {
			return r.SQLite().FindAlbumByTitle(ctx, sqlc_sqlite.FindAlbumByTitleParams{
				LibraryID:   libraryID,
				Title:       title,
				AlbumArtist: common.NullString(albumArtist),
			})
		},
		postgresAlbumToDomain,
		sqliteAlbumToDomain,
	)
	return album, r.ConvertNotFoundError(err)
}

// ListAlbumsByLibrary retrieves all albums in a library
func (r *Repository) ListAlbumsByLibrary(ctx context.Context, libraryID int64) ([]*media.Album, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MusicAlbum, error) {
			return r.Postgres().ListAlbumsByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.MusicAlbum, error) {
			return r.SQLite().ListAlbumsByLibrary(ctx, libraryID)
		},
		postgresAlbumToDomain,
		sqliteAlbumToDomain,
	)
}

// ListMusicTracksByAlbumID retrieves all tracks for a specific album ID
func (r *Repository) ListMusicTracksByAlbumID(ctx context.Context, albumID int64) ([]*media.MusicTrack, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.ListMusicTracksByAlbumIDRow, error) {
			return r.Postgres().ListMusicTracksByAlbumID(ctx, common.NullInt32FromInt64(albumID))
		},
		func() ([]sqlc_sqlite.ListMusicTracksByAlbumIDRow, error) {
			return r.SQLite().ListMusicTracksByAlbumID(ctx, common.NullInt64(albumID))
		},
		postgresMusicTrackRowToDomain[sqlc_postgres.ListMusicTracksByAlbumIDRow],
		sqliteGenericMusicTrackRowToDomain[sqlc_sqlite.ListMusicTracksByAlbumIDRow],
	)
}

// ============================================================================
// Artist Entity Operations
// ============================================================================

// CreateArtist creates a new artist entity
func (r *Repository) CreateArtist(ctx context.Context, artist *media.Artist) error {
	if err := artist.IsValid(); err != nil {
		return err
	}

	createdArtist, err := common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicArtist, error) {
			return r.Postgres().CreateArtist(ctx, buildPostgresCreateArtistParams(artist))
		},
		func() (sqlc_sqlite.MusicArtist, error) {
			return r.SQLite().CreateArtist(ctx, buildSQLiteCreateArtistParams(artist))
		},
		func(row sqlc_postgres.MusicArtist) *media.Artist {
			artist.ID = int64(row.ID)
			return artist
		},
		func(row sqlc_sqlite.MusicArtist) *media.Artist {
			artist.ID = row.ID
			return artist
		},
	)
	if err != nil {
		return err
	}
	artist.ID = createdArtist.ID
	return nil
}

// GetArtistByID retrieves an artist by its ID
func (r *Repository) GetArtistByID(ctx context.Context, id int64) (*media.Artist, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicArtist, error) {
			return r.Postgres().GetArtistByID(ctx, int32(id))
		},
		func() (sqlc_sqlite.MusicArtist, error) {
			return r.SQLite().GetArtistByID(ctx, id)
		},
		postgresArtistToDomain,
		sqliteArtistToDomain,
	)
}

// FindArtistByName finds an artist by library and name
func (r *Repository) FindArtistByName(ctx context.Context, libraryID int64, name string) (*media.Artist, error) {
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.MusicArtist, error) {
			return r.Postgres().FindArtistByName(ctx, sqlc_postgres.FindArtistByNameParams{
				LibraryID: int32(libraryID),
				Name:      name,
			})
		},
		func() (sqlc_sqlite.MusicArtist, error) {
			return r.SQLite().FindArtistByName(ctx, sqlc_sqlite.FindArtistByNameParams{
				LibraryID: libraryID,
				Name:      name,
			})
		},
		postgresArtistToDomain,
		sqliteArtistToDomain,
	)
}

// ListArtistsByLibrary retrieves all artist entities in a library
func (r *Repository) ListArtistsByLibrary(ctx context.Context, libraryID int64) ([]*media.Artist, error) {
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.MusicArtist, error) {
			return r.Postgres().ListArtistsByLibrary(ctx, int32(libraryID))
		},
		func() ([]sqlc_sqlite.MusicArtist, error) {
			return r.SQLite().ListArtistsByLibrary(ctx, libraryID)
		},
		postgresArtistToDomain,
		sqliteArtistToDomain,
	)
}

// CreateMusicTrackWithEntities atomically creates a music track along with artist and album entities if needed.
// This operation is transactional - all entities are created or none are created.
// If any step fails, the entire transaction is rolled back to prevent orphaned records.
func (r *Repository) CreateMusicTrackWithEntities(ctx context.Context, track *media.MusicTrack, artist *media.Artist, album *media.Album) error {
	return common.WithTransaction(r.BaseRepository, ctx, func(tx *common.TransactionContext) error {
		// Step 1: Create or find artist entity if provided
		if artist != nil && artist.Name != "" {
			// Try to find existing artist first
			existingArtist, err := r.FindArtistByName(ctx, artist.LibraryID, artist.Name)
			if err == nil && existingArtist != nil {
				// Use existing artist
				artist.ID = existingArtist.ID
				track.ArtistID = existingArtist.ID
			} else {
				// Create new artist within transaction
				if tx.IsPostgresDB() {
					params := buildPostgresCreateArtistParams(artist)
					createdArtist, err := tx.Postgres().CreateArtist(ctx, params)
					if err != nil {
						return fmt.Errorf("failed to create artist in transaction: %w", err)
					}
					artist.ID = int64(createdArtist.ID)
					track.ArtistID = int64(createdArtist.ID)
				} else {
					params := buildSQLiteCreateArtistParams(artist)
					createdArtist, err := tx.SQLite().CreateArtist(ctx, params)
					if err != nil {
						return fmt.Errorf("failed to create artist in transaction: %w", err)
					}
					artist.ID = createdArtist.ID
					track.ArtistID = createdArtist.ID
				}
			}
		}

		// Step 2: Create or find album entity if provided
		if album != nil && album.Title != "" {
			// Determine the effective album artist for lookup
			effectiveAlbumArtist := album.AlbumArtist
			if effectiveAlbumArtist == "" {
				effectiveAlbumArtist = album.Artist
			}

			// Try to find existing album first
			existingAlbum, err := r.FindAlbumByTitle(ctx, album.LibraryID, album.Title, effectiveAlbumArtist)
			if err == nil && existingAlbum != nil {
				// Use existing album
				album.ID = existingAlbum.ID
				track.AlbumID = existingAlbum.ID
			} else {
				// Link album to artist if we just created one
				if artist != nil && artist.ID > 0 {
					album.ArtistID = artist.ID
				}

				// Create new album within transaction
				if tx.IsPostgresDB() {
					params := buildPostgresCreateAlbumParams(album)
					createdAlbum, err := tx.Postgres().CreateAlbum(ctx, params)
					if err != nil {
						return fmt.Errorf("failed to create album in transaction: %w", err)
					}
					album.ID = int64(createdAlbum.ID)
					track.AlbumID = int64(createdAlbum.ID)
				} else {
					params := buildSQLiteCreateAlbumParams(album)
					createdAlbum, err := tx.SQLite().CreateAlbum(ctx, params)
					if err != nil {
						return fmt.Errorf("failed to create album in transaction: %w", err)
					}
					album.ID = createdAlbum.ID
					track.AlbumID = createdAlbum.ID
				}
			}
		}

		// Step 3: Create base media record within transaction
		if tx.IsPostgresDB() {
			mediaParams := buildPostgresCreateMediaParams(&track.Media)
			createdMedia, err := tx.Postgres().CreateMedia(ctx, mediaParams)
			if err != nil {
				return fmt.Errorf("failed to create media in transaction: %w", err)
			}
			track.Media.ID = int64(createdMedia.ID)
		} else {
			mediaParams := buildSQLiteCreateMediaParams(&track.Media)
			createdMedia, err := tx.SQLite().CreateMedia(ctx, mediaParams)
			if err != nil {
				return fmt.Errorf("failed to create media in transaction: %w", err)
			}
			track.Media.ID = createdMedia.ID
		}

		// Step 4: Create music track record within transaction
		if tx.IsPostgresDB() {
			trackParams := buildPostgresCreateMusicTrackParams(track)
			if err := tx.Postgres().CreateMusicTrack(ctx, trackParams); err != nil {
				return fmt.Errorf("failed to create music track in transaction: %w", err)
			}
		} else {
			trackParams := buildSQLiteCreateMusicTrackParams(track)
			if err := tx.SQLite().CreateMusicTrack(ctx, trackParams); err != nil {
				return fmt.Errorf("failed to create music track in transaction: %w", err)
			}
		}

		return nil
	})
}
