package music

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
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
		sqlc_sqlite.SearchMusicTracksRow
}

// extractMusicTrackFields extracts common fields from any music track row type
func extractMusicTrackFields[T musicTrackRow](row T) musicTrackFields {
	// Use type assertion to access fields that are common across all row types
	var fields musicTrackFields

	// All row types have the same structure, so we can use type switch
	switch r := any(row).(type) {
	case sqlc_sqlite.ListMusicTracksByLibraryRow:
		fields = musicTrackFields{
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
	case sqlc_sqlite.ListMusicTracksByAlbumRow:
		fields = musicTrackFields{
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
	case sqlc_sqlite.ListMusicTracksByArtistRow:
		fields = musicTrackFields{
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
	case sqlc_sqlite.SearchMusicTracksRow:
		fields = musicTrackFields{
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

	return fields
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
