package music

import (
	"context"
	"fmt"
	"unsafe"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
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
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().CreateMusicTrack(ctx, buildSQLiteCreateMusicTrackParams(track))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create music track record: %w", err)
	}

	return nil
}

// GetMusicTrackByID retrieves a music track by its media ID
func (r *Repository) GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetMusicTrackByMediaID(ctx, id)
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	// Convert to domain music track
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}
	return sqliteMusicTrackToDomain(result.(sqlc_sqlite.GetMusicTrackByMediaIDRow)), nil
}

// ListMusicTracksByLibrary retrieves all music tracks in a specific library
func (r *Repository) ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*media.MusicTrack, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMusicTracksByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain music tracks
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return convertRowsToMusicTracks(result.([]sqlc_sqlite.ListMusicTracksByLibraryRow)), nil
}

// ListMusicTracksByAlbum retrieves all tracks from a specific album
func (r *Repository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMusicTracksByAlbum(ctx, sqlc_sqlite.ListMusicTracksByAlbumParams{
				LibraryID: libraryID,
				Album:     common.NullString(album),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain music tracks
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return convertRowsToMusicTracks(result.([]sqlc_sqlite.ListMusicTracksByAlbumRow)), nil
}

// ListMusicTracksByArtist retrieves all tracks by a specific artist
func (r *Repository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	// Use LIKE pattern for artist search
	searchPattern := "%" + artist + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMusicTracksByArtist(ctx, sqlc_sqlite.ListMusicTracksByArtistParams{
				LibraryID:   libraryID,
				Artist:      common.NullString(searchPattern),
				AlbumArtist: common.NullString(searchPattern),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain music tracks
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return convertRowsToMusicTracks(result.([]sqlc_sqlite.ListMusicTracksByArtistRow)), nil
}

// UpdateMusicTrack modifies an existing music track
func (r *Repository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &track.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Then, update the music track-specific record
	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return nil, r.SQLite().UpdateMusicTrack(ctx, buildSQLiteUpdateMusicTrackParams(track))
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update music track record: %w", err)
	}

	return nil
}

// SearchMusicTracks searches for music tracks by title, artist, or album
func (r *Repository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().SearchMusicTracks(ctx, sqlc_sqlite.SearchMusicTracksParams{
				LibraryID: libraryID,
				Title:     searchPattern,
				Artist:    common.NullString(searchPattern),
				Album:     common.NullString(searchPattern),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain music tracks
	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return convertRowsToMusicTracks(result.([]sqlc_sqlite.SearchMusicTracksRow)), nil
}

// musicTrackRow is a generic interface for all music track query row types.
// This allows us to write a single conversion function for all row types.
type musicTrackRow interface {
	sqlc_sqlite.ListMusicTracksByLibraryRow |
		sqlc_sqlite.ListMusicTracksByAlbumRow |
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

// convertRowsToMusicTracks converts a slice of any music track row type to domain MusicTrack entities
func convertRowsToMusicTracks[T musicTrackRow](rows []T) []*media.MusicTrack {
	tracks := make([]*media.MusicTrack, len(rows))
	for i, row := range rows {
		fields := extractMusicTrackFields(row)
		tracks[i] = sqliteMusicTrackRowToDomain(fields)
	}
	return tracks
}

// CountArtistsByLibrary returns the total count of unique artists in a library
func (r *Repository) CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountArtistsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
}

// ListArtistsByLibraryPaginated retrieves unique artists in a library with pagination
func (r *Repository) ListArtistsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]media.MusicArtist, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			if sortBy == "title_desc" {
				return r.SQLite().ListArtistsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListArtistsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListArtistsByLibraryPaginated(ctx, sqlc_sqlite.ListArtistsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	// Handle different row types based on sort order
	var artists []media.MusicArtist
	if sortBy == "title_desc" {
		sqResults := result.([]sqlc_sqlite.ListArtistsByLibraryPaginatedDescRow)
		artists = make([]media.MusicArtist, len(sqResults))
		for i, row := range sqResults {
			var repID int64
			if row.RepresentativeID != nil {
				switch v := row.RepresentativeID.(type) {
				case int64:
					repID = v
				case int:
					repID = int64(v)
				}
			}
			artists[i] = media.MusicArtist{
				RepresentativeID: repID,
				Artist:           common.ParseNullString(row.Artist),
				AlbumCount:       row.AlbumCount,
				TrackCount:       row.TrackCount,
			}
		}
	} else {
		sqResults := result.([]sqlc_sqlite.ListArtistsByLibraryPaginatedRow)
		artists = make([]media.MusicArtist, len(sqResults))
		for i, row := range sqResults {
			var repID int64
			if row.RepresentativeID != nil {
				switch v := row.RepresentativeID.(type) {
				case int64:
					repID = v
				case int:
					repID = int64(v)
				}
			}
			artists[i] = media.MusicArtist{
				RepresentativeID: repID,
				Artist:           common.ParseNullString(row.Artist),
				AlbumCount:       row.AlbumCount,
				TrackCount:       row.TrackCount,
			}
		}
	}
	return artists, nil
}

// CountAlbumsByLibrary returns the total count of unique albums in a library
func (r *Repository) CountAlbumsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountAlbumsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
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

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			if sortBy == "title_desc" {
				return r.SQLite().ListAlbumsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListAlbumsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListAlbumsByLibraryPaginated(ctx, sqlc_sqlite.ListAlbumsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	// Handle different row types based on sort order
	var albums []media.MusicAlbum
	if sortBy == "title_desc" {
		sqResults := result.([]sqlc_sqlite.ListAlbumsByLibraryPaginatedDescRow)
		albums = make([]media.MusicAlbum, len(sqResults))
		for i, row := range sqResults {
			albums[i] = media.MusicAlbum{
				Album:       common.ParseNullString(row.Album),
				AlbumArtist: common.ParseNullString(row.AlbumArtist),
				Year:        common.ParseNullInt64(row.Year),
				TrackCount:  row.TrackCount,
				Duration:    int64(common.ParseNullFloat64(row.TotalDuration)),
			}
		}
	} else {
		sqResults := result.([]sqlc_sqlite.ListAlbumsByLibraryPaginatedRow)
		albums = make([]media.MusicAlbum, len(sqResults))
		for i, row := range sqResults {
			albums[i] = media.MusicAlbum{
				Album:       common.ParseNullString(row.Album),
				AlbumArtist: common.ParseNullString(row.AlbumArtist),
				Year:        common.ParseNullInt64(row.Year),
				TrackCount:  row.TrackCount,
				Duration:    int64(common.ParseNullFloat64(row.TotalDuration)),
			}
		}
	}
	return albums, nil
}

// CountMusicTracksByLibrary returns the total count of music tracks in a library
func (r *Repository) CountMusicTracksByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountMusicTracksByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.Router().IsPostgresDB() {
		return 0, r.PostgresNotImplemented()
	}

	return result.(int64), nil
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

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			if sortBy == "title_desc" {
				return r.SQLite().ListMusicTracksByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListMusicTracksByLibraryPaginated(ctx, sqlc_sqlite.ListMusicTracksByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	// Handle different row types based on sort order - both types are compatible with musicTrackRow interface
	if sortBy == "title_desc" {
		sqResults := result.([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedDescRow)
		return convertRowsToMusicTracks(sqResults), nil
	}

	sqResults := result.([]sqlc_sqlite.ListMusicTracksByLibraryPaginatedRow)
	return convertRowsToMusicTracks(sqResults), nil
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

	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			// Choose the appropriate query based on sort order
			if sortBy == "title_desc" {
				return r.SQLite().ListArtistIDsByLibraryPaginatedDesc(ctx, sqlc_sqlite.ListArtistIDsByLibraryPaginatedDescParams{
					LibraryID: libraryID,
					Limit:     int64(pagination.Limit),
					Offset:    int64(pagination.Offset),
				})
			}
			return r.SQLite().ListArtistIDsByLibraryPaginated(ctx, sqlc_sqlite.ListArtistIDsByLibraryPaginatedParams{
				LibraryID: libraryID,
				Limit:     int64(pagination.Limit),
				Offset:    int64(pagination.Offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return result.([]int64), nil
}
