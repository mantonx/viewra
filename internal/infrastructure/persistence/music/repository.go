package music

import (
	"context"
	"fmt"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
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
	if err := r.Q().CreateMusicTrack(ctx, buildCreateMusicTrackParams(track)); err != nil {
		return fmt.Errorf("failed to create music track record: %w", err)
	}

	return nil
}

// GetMusicTrackByID retrieves a music track by its media ID
func (r *Repository) GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error) {
	row, err := r.Q().GetMusicTrackByMediaID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return musicTrackRowToDomain(row), nil
}

// ListMusicTracksByLibrary retrieves all music tracks in a specific library
func (r *Repository) ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*media.MusicTrack, error) {
	rows, err := r.Q().ListMusicTracksByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, listMusicTrackRowToDomain), nil
}

// ListMusicTracksByAlbum retrieves all tracks from a specific album
func (r *Repository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	rows, err := r.Q().ListMusicTracksByAlbum(ctx, unified.ListMusicTracksByAlbumParams{
		LibraryID: libraryID,
		Album:     common.NullString(album),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, albumMusicTrackRowToDomain), nil
}

// ListMusicTracksByArtist retrieves all tracks by a specific artist
func (r *Repository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	// Use LIKE pattern for artist search
	searchPattern := "%" + artist + "%"

	rows, err := r.Q().ListMusicTracksByArtist(ctx, unified.ListMusicTracksByArtistParams{
		LibraryID:   libraryID,
		Artist:      common.NullString(searchPattern),
		AlbumArtist: common.NullString(searchPattern),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, artistMusicTrackRowToDomain), nil
}

// UpdateMusicTrack modifies an existing music track
func (r *Repository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	// First, update the base media record
	if err := r.mediaRepo.Update(ctx, &track.Media); err != nil {
		return fmt.Errorf("failed to update base media record: %w", err)
	}

	// Then, update the music track-specific record
	if err := r.Q().UpdateMusicTrack(ctx, buildUpdateMusicTrackParams(track)); err != nil {
		return fmt.Errorf("failed to update music track record: %w", err)
	}

	return nil
}

// SearchMusicTracks searches for music tracks by title, artist, or album
func (r *Repository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	// Add wildcards for LIKE search
	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchMusicTracks(ctx, unified.SearchMusicTracksParams{
		LibraryID: libraryID,
		Title:     searchPattern,
		Artist:    common.NullString(searchPattern),
		Album:     common.NullString(searchPattern),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, searchMusicTrackRowToDomain), nil
}

// CountArtistsByLibrary returns the total count of artists in a library
func (r *Repository) CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountArtistsInLibrary(ctx, libraryID)
}

// ListArtistsByLibraryPaginated retrieves artists from music_artists table with pagination.
// Returns artists with album and track counts, sorted by name.
func (r *Repository) ListArtistsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]media.MusicArtist, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	// Default to title_asc if not specified
	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		rows, err := r.Q().GetArtistsWithCountsByLibraryPaginatedDesc(ctx, unified.GetArtistsWithCountsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, artistWithCountsDescRowToDomain), nil
	}

	rows, err := r.Q().GetArtistsWithCountsByLibraryPaginated(ctx, unified.GetArtistsWithCountsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, artistWithCountsRowToDomain), nil
}

// CountAlbumsByLibrary returns the total count of unique albums in a library
func (r *Repository) CountAlbumsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountAlbumsByLibrary(ctx, libraryID)
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
		rows, err := r.Q().ListAlbumsByLibraryPaginatedDesc(ctx, unified.ListAlbumsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, listAlbumDescRowToDomain), nil
	}

	rows, err := r.Q().ListAlbumsByLibraryPaginated(ctx, unified.ListAlbumsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, listAlbumRowToDomain), nil
}

// CountMusicTracksByLibrary returns the total count of music tracks in a library
func (r *Repository) CountMusicTracksByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountMusicTracksByLibrary(ctx, libraryID)
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
		rows, err := r.Q().ListMusicTracksByLibraryPaginatedDesc(ctx, unified.ListMusicTracksByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, paginatedDescMusicTrackRowToDomain), nil
	}

	rows, err := r.Q().ListMusicTracksByLibraryPaginated(ctx, unified.ListMusicTracksByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, paginatedMusicTrackRowToDomain), nil
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
		return r.Q().ListArtistIDsByLibraryPaginatedDesc(ctx, unified.ListArtistIDsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
	}

	return r.Q().ListArtistIDsByLibraryPaginated(ctx, unified.ListArtistIDsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
}

// CreateAlbum creates a new album entity
func (r *Repository) CreateAlbum(ctx context.Context, album *media.Album) error {
	createdAlbum, err := r.Q().CreateAlbum(ctx, buildCreateAlbumParams(album))
	if err != nil {
		return err
	}
	album.ID = createdAlbum.ID
	return nil
}

// GetAlbumByID retrieves an album by its ID
func (r *Repository) GetAlbumByID(ctx context.Context, id int64) (*media.Album, error) {
	row, err := r.Q().GetAlbumByID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return albumToDomain(row), nil
}

// UpdateAlbum updates an existing album in the database
func (r *Repository) UpdateAlbum(ctx context.Context, album *media.Album) error {
	return r.Q().UpdateAlbum(ctx, buildUpdateAlbumParams(album))
}

// FindAlbumByTitle finds an album by library, title, and album artist
func (r *Repository) FindAlbumByTitle(ctx context.Context, libraryID int64, title, albumArtist string) (*media.Album, error) {
	row, err := r.Q().FindAlbumByTitle(ctx, unified.FindAlbumByTitleParams{
		LibraryID:   libraryID,
		Title:       title,
		AlbumArtist: common.NullString(albumArtist),
	})
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return albumToDomain(row), nil
}

// ListAlbumsByLibrary retrieves all albums in a library
func (r *Repository) ListAlbumsByLibrary(ctx context.Context, libraryID int64) ([]*media.Album, error) {
	rows, err := r.Q().ListAlbumsByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, albumToDomain), nil
}

// ListMusicTracksByAlbumID retrieves all tracks for a specific album ID
func (r *Repository) ListMusicTracksByAlbumID(ctx context.Context, albumID int64) ([]*media.MusicTrack, error) {
	rows, err := r.Q().ListMusicTracksByAlbumID(ctx, common.NullInt64(albumID))
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, albumIDMusicTrackRowToDomain), nil
}

// ============================================================================
// Artist Entity Operations
// ============================================================================

// CreateArtist creates a new artist entity
func (r *Repository) CreateArtist(ctx context.Context, artist *media.Artist) error {
	if err := artist.IsValid(); err != nil {
		return err
	}

	createdArtist, err := r.Q().CreateArtist(ctx, buildCreateArtistParams(artist))
	if err != nil {
		return err
	}
	artist.ID = createdArtist.ID
	return nil
}

// GetArtistByID retrieves an artist by its ID
func (r *Repository) GetArtistByID(ctx context.Context, id int64) (*media.Artist, error) {
	row, err := r.Q().GetArtistByID(ctx, id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return artistToDomain(row), nil
}

// UpdateArtist updates an existing artist in the database
func (r *Repository) UpdateArtist(ctx context.Context, artist *media.Artist) error {
	return r.Q().UpdateArtist(ctx, buildUpdateArtistParams(artist))
}

// FindArtistByName finds an artist by library and name
func (r *Repository) FindArtistByName(ctx context.Context, libraryID int64, name string) (*media.Artist, error) {
	row, err := r.Q().FindArtistByName(ctx, unified.FindArtistByNameParams{
		LibraryID: libraryID,
		Name:      name,
	})
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return artistToDomain(row), nil
}

// ListArtistsByLibrary retrieves all artist entities in a library
func (r *Repository) ListArtistsByLibrary(ctx context.Context, libraryID int64) ([]*media.Artist, error) {
	rows, err := r.Q().ListArtistsByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, artistToDomain), nil
}

// SearchArtistsByName searches for artists by name with pagination
func (r *Repository) SearchArtistsByName(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]*media.Artist, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchArtistsByName(ctx, unified.SearchArtistsByNameParams{
		LibraryID: libraryID,
		Name:      searchPattern,
		SortName:  common.NullString(searchPattern),
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, artistToDomain), nil
}

// CountSearchArtistsByName returns the count of artists matching a search query
func (r *Repository) CountSearchArtistsByName(ctx context.Context, libraryID int64, query string) (int64, error) {
	searchPattern := "%" + query + "%"

	return r.Q().CountSearchArtistsByName(ctx, unified.CountSearchArtistsByNameParams{
		LibraryID: libraryID,
		Name:      searchPattern,
		SortName:  common.NullString(searchPattern),
	})
}

// ArtistWithCounts represents an artist with album and track counts from the database
type ArtistWithCounts struct {
	ID         int64
	LibraryID  int64
	Name       string
	AlbumCount int
	TrackCount int
	CreatedAt  time.Time
}

// GetArtistsWithCountsByLibraryPaginated retrieves artists with album/track counts using database-level aggregation
func (r *Repository) GetArtistsWithCountsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *domainCommon.PaginationParams) ([]ArtistWithCounts, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	sortBy := pagination.SortBy
	if sortBy == "" {
		sortBy = "title_asc"
	}

	if sortBy == "title_desc" {
		rows, err := r.Q().GetArtistsWithCountsByLibraryPaginatedDesc(ctx, unified.GetArtistsWithCountsByLibraryPaginatedDescParams{
			LibraryID: libraryID,
			Limit:     int64(pagination.Limit),
			Offset:    int64(pagination.Offset),
		})
		if err != nil {
			return nil, err
		}
		return mapSlice(rows, artistWithCountsDescToInternal), nil
	}

	rows, err := r.Q().GetArtistsWithCountsByLibraryPaginated(ctx, unified.GetArtistsWithCountsByLibraryPaginatedParams{
		LibraryID: libraryID,
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, artistWithCountsToInternal), nil
}

// SearchArtistsWithCountsByNamePaginated searches for artists by name and returns counts
func (r *Repository) SearchArtistsWithCountsByNamePaginated(ctx context.Context, libraryID int64, query string, pagination *domainCommon.PaginationParams) ([]ArtistWithCounts, error) {
	if pagination == nil {
		pagination = domainCommon.DefaultPaginationParams()
	}

	searchPattern := "%" + query + "%"

	rows, err := r.Q().SearchArtistsWithCountsByNamePaginated(ctx, unified.SearchArtistsWithCountsByNamePaginatedParams{
		LibraryID: libraryID,
		Name:      searchPattern,
		SortName:  common.NullString(searchPattern),
		Limit:     int64(pagination.Limit),
		Offset:    int64(pagination.Offset),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, searchArtistWithCountsRowToDomain), nil
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
				params := buildCreateArtistParams(artist)
				createdArtist, err := tx.Q().CreateArtist(ctx, params)
				if err != nil {
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
				params := buildCreateAlbumParams(album)
				createdAlbum, err := tx.Q().CreateAlbum(ctx, params)
				if err != nil {
					return fmt.Errorf("failed to create album in transaction: %w", err)
				}
				album.ID = createdAlbum.ID
				track.AlbumID = createdAlbum.ID
			}
		}

		// Step 3: Create base media record within transaction
		mediaParams := buildCreateMediaParams(&track.Media)
		createdMedia, err := tx.Q().CreateMedia(ctx, mediaParams)
		if err != nil {
			return fmt.Errorf("failed to create media in transaction: %w", err)
		}
		track.Media.ID = createdMedia.ID

		// Step 4: Create music track record within transaction
		trackParams := buildCreateMusicTrackParams(track)
		if err := tx.Q().CreateMusicTrack(ctx, trackParams); err != nil {
			return fmt.Errorf("failed to create music track in transaction: %w", err)
		}

		return nil
	})
}
