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

// convertRowsToMusicTracks converts a slice of any music track row type to domain MusicTrack entities
func convertRowsToMusicTracks[T musicTrackRow](rows []T) []*media.MusicTrack {
	tracks := make([]*media.MusicTrack, len(rows))
	for i, row := range rows {
		fields := extractMusicTrackFields(row)
		tracks[i] = sqliteMusicTrackRowToDomain(fields)
	}
	return tracks
}

// CountArtistsByLibrary returns the total count of artists in a library
func (r *Repository) CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CountArtistsInLibrary(ctx, libraryID)
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

// CreateAlbum creates a new album entity
func (r *Repository) CreateAlbum(ctx context.Context, album *media.Album) error {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().CreateAlbum(ctx, buildSQLiteCreateAlbumParams(album))
		},
	)
	if err != nil {
		return err
	}

	if r.Router().IsPostgresDB() {
		return r.PostgresNotImplemented()
	}

	// Update album ID from the returned row
	createdAlbum := result.(sqlc_sqlite.MusicAlbum)
	album.ID = createdAlbum.ID
	return nil
}

// GetAlbumByID retrieves an album by its ID
func (r *Repository) GetAlbumByID(ctx context.Context, id int64) (*media.Album, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetAlbumByID(ctx, id)
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return sqliteAlbumToDomain(result.(sqlc_sqlite.MusicAlbum)), nil
}

// FindAlbumByTitle finds an album by library, title, and album artist
func (r *Repository) FindAlbumByTitle(ctx context.Context, libraryID int64, title, albumArtist string) (*media.Album, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().FindAlbumByTitle(ctx, sqlc_sqlite.FindAlbumByTitleParams{
				LibraryID:   libraryID,
				Title:       title,
				AlbumArtist: common.NullString(albumArtist),
			})
		},
	)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return sqliteAlbumToDomain(result.(sqlc_sqlite.MusicAlbum)), nil
}

// ListAlbumsByLibrary retrieves all albums in a library
func (r *Repository) ListAlbumsByLibrary(ctx context.Context, libraryID int64) ([]*media.Album, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListAlbumsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	rows := result.([]sqlc_sqlite.MusicAlbum)
	albums := make([]*media.Album, len(rows))
	for i, row := range rows {
		albums[i] = sqliteAlbumToDomain(row)
	}
	return albums, nil
}

// ListMusicTracksByAlbumID retrieves all tracks for a specific album ID
func (r *Repository) ListMusicTracksByAlbumID(ctx context.Context, albumID int64) ([]*media.MusicTrack, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListMusicTracksByAlbumID(ctx, common.NullInt64(albumID))
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	return convertRowsToMusicTracks(result.([]sqlc_sqlite.ListMusicTracksByAlbumIDRow)), nil
}

// ============================================================================
// Artist Entity Operations
// ============================================================================

// CreateArtist creates a new artist entity
func (r *Repository) CreateArtist(ctx context.Context, artist *media.Artist) error {
	if err := artist.IsValid(); err != nil {
		return err
	}

	_, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			params := buildSQLiteCreateArtistParams(artist)
			result, err := r.SQLite().CreateArtist(ctx, params)
			if err != nil {
				return nil, err
			}
			artist.ID = result.ID
			return nil, nil
		},
	)

	return err
}

// GetArtistByID retrieves an artist by its ID
func (r *Repository) GetArtistByID(ctx context.Context, id int64) (*media.Artist, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().GetArtistByID(ctx, id)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	row := result.(sqlc_sqlite.MusicArtist)
	return sqliteArtistToDomain(row), nil
}

// FindArtistByName finds an artist by library and name
func (r *Repository) FindArtistByName(ctx context.Context, libraryID int64, name string) (*media.Artist, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().FindArtistByName(ctx, sqlc_sqlite.FindArtistByNameParams{
				LibraryID: libraryID,
				Name:      name,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	row := result.(sqlc_sqlite.MusicArtist)
	return sqliteArtistToDomain(row), nil
}

// ListArtistsByLibrary retrieves all artist entities in a library
func (r *Repository) ListArtistsByLibrary(ctx context.Context, libraryID int64) ([]*media.Artist, error) {
	result, err := r.Router().Route(
		func() (any, error) {
			return nil, r.PostgresNotImplemented()
		},
		func() (any, error) {
			return r.SQLite().ListArtistsByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.Router().IsPostgresDB() {
		return nil, r.PostgresNotImplemented()
	}

	rows := result.([]sqlc_sqlite.MusicArtist)
	artists := make([]*media.Artist, len(rows))
	for i, row := range rows {
		artists[i] = sqliteArtistToDomain(row)
	}
	return artists, nil
}

// CreateMusicTrackWithEntities atomically creates a music track along with artist and album entities if needed.
// This operation is transactional - all entities are created or none are created.
// If any step fails, the entire transaction is rolled back to prevent orphaned records.
func (r *Repository) CreateMusicTrackWithEntities(ctx context.Context, track *media.MusicTrack, artist *media.Artist, album *media.Album) error {
	// PostgreSQL support not yet implemented
	if r.Router().IsPostgresDB() {
		return r.PostgresNotImplemented()
	}

	// Begin transaction
	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback in case of panic or error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Create queries instance bound to this transaction
	txQueries := sqlc_sqlite.New(tx)

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
			params := buildSQLiteCreateArtistParams(artist)
			createdArtist, err := txQueries.CreateArtist(ctx, params)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to create artist in transaction: %w", err)
			}
			artist.ID = createdArtist.ID
			track.ArtistID = createdArtist.ID
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
			params := buildSQLiteCreateAlbumParams(album)
			createdAlbum, err := txQueries.CreateAlbum(ctx, params)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to create album in transaction: %w", err)
			}
			album.ID = createdAlbum.ID
			track.AlbumID = createdAlbum.ID
		}
	}

	// Step 3: Create base media record within transaction
	mediaParams := buildSQLiteCreateMediaParams(&track.Media)
	createdMedia, err := txQueries.CreateMedia(ctx, mediaParams)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to create media in transaction: %w", err)
	}
	track.Media.ID = createdMedia.ID

	// Step 4: Create music track record within transaction
	trackParams := buildSQLiteCreateMusicTrackParams(track)
	if err := txQueries.CreateMusicTrack(ctx, trackParams); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to create music track in transaction: %w", err)
	}

	// Commit transaction - all operations succeeded
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
