package mocks

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// MusicRepository is a mock implementation of media.MusicRepository for testing.
type MusicRepository struct {
	t         testing.TB
	mu        sync.RWMutex
	tracks    map[int64]*media.MusicTrack
	albums    map[int64]*media.Album
	artists   map[int64]*media.Artist
	nextID    int64
	nextAlbumID  int64
	nextArtistID int64

	// Error injection
	CreateErr error
	GetErr    error
	ListErr   error
	UpdateErr error
	SearchErr error
	CountErr  error
}

// NewMusicRepository creates a new mock music repository.
func NewMusicRepository(t testing.TB) *MusicRepository {
	return &MusicRepository{
		t:            t,
		tracks:       make(map[int64]*media.MusicTrack),
		albums:       make(map[int64]*media.Album),
		artists:      make(map[int64]*media.Artist),
		nextID:       1,
		nextAlbumID:  1,
		nextArtistID: 1,
	}
}

// WithTracks pre-populates the mock with music tracks.
func (r *MusicRepository) WithTracks(items ...*media.MusicTrack) *MusicRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		r.tracks[item.ID] = item
		if item.ID >= r.nextID {
			r.nextID = item.ID + 1
		}
	}
	return r
}

// WithCreateError injects an error for Create operations.
func (r *MusicRepository) WithCreateError(err error) *MusicRepository {
	r.CreateErr = err
	return r
}

// Implementation of media.MusicRepository interface

func (r *MusicRepository) CreateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if track.ID == 0 {
		track.ID = r.nextID
		r.nextID++
	}

	r.tracks[track.ID] = track
	return nil
}

func (r *MusicRepository) GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	track, exists := r.tracks[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return track, nil
}

func (r *MusicRepository) ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*media.MusicTrack, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.MusicTrack
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			result = append(result, track)
		}
	}

	return result, nil
}

func (r *MusicRepository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.MusicTrack
	for _, track := range r.tracks {
		if track.LibraryID == libraryID && track.Album == album {
			result = append(result, track)
		}
	}

	return result, nil
}

func (r *MusicRepository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.MusicTrack
	for _, track := range r.tracks {
		if track.LibraryID == libraryID && track.Artist == artist {
			result = append(result, track)
		}
	}

	return result, nil
}

func (r *MusicRepository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tracks[track.ID]; !exists {
		return sql.ErrNoRows
	}

	r.tracks[track.ID] = track
	return nil
}

func (r *MusicRepository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	if r.SearchErr != nil {
		return nil, r.SearchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.MusicTrack
	queryLower := strings.ToLower(query)

	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			titleMatch := strings.Contains(strings.ToLower(track.Title), queryLower)
			artistMatch := strings.Contains(strings.ToLower(track.Artist), queryLower)
			albumMatch := strings.Contains(strings.ToLower(track.Album), queryLower)

			if titleMatch || artistMatch || albumMatch {
				result = append(result, track)
			}
		}
	}

	return result, nil
}

func (r *MusicRepository) CountArtistsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	artists := make(map[string]bool)
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			artists[track.Artist] = true
		}
	}

	return int64(len(artists)), nil
}

func (r *MusicRepository) ListArtistsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]media.MusicArtist, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Aggregate tracks by artist
	artistMap := make(map[string]*media.MusicArtist)

	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			artist, exists := artistMap[track.Artist]
			if !exists {
				artist = &media.MusicArtist{
					RepresentativeID: track.ID,
					Artist:           track.Artist,
					AlbumCount:       0,
					TrackCount:       0,
				}
				artistMap[track.Artist] = artist
			}
			artist.TrackCount++
		}
	}

	// Count albums per artist
	for artistName, artist := range artistMap {
		albums := make(map[string]bool)
		for _, track := range r.tracks {
			if track.LibraryID == libraryID && track.Artist == artistName {
				albums[track.Album] = true
			}
		}
		artist.AlbumCount = int64(len(albums))
	}

	// Convert to slice
	var allArtists []media.MusicArtist
	for _, artist := range artistMap {
		allArtists = append(allArtists, *artist)
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allArtists) {
		return []media.MusicArtist{}, nil
	}

	if end > len(allArtists) {
		end = len(allArtists)
	}

	return allArtists[start:end], nil
}

func (r *MusicRepository) CountAlbumsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	albums := make(map[string]bool)
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			albums[track.Album] = true
		}
	}

	return int64(len(albums)), nil
}

func (r *MusicRepository) ListAlbumsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]media.MusicAlbum, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Aggregate tracks by album
	albumMap := make(map[string]*media.MusicAlbum)

	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			album, exists := albumMap[track.Album]
			if !exists {
				album = &media.MusicAlbum{
					Album:       track.Album,
					AlbumArtist: track.AlbumArtist,
					Year:        int64(track.Year),
					TrackCount:  0,
					Duration:    0,
				}
				albumMap[track.Album] = album
			}
			album.TrackCount++
			album.Duration += int64(track.Duration)
		}
	}

	// Convert to slice
	var allAlbums []media.MusicAlbum
	for _, album := range albumMap {
		allAlbums = append(allAlbums, *album)
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allAlbums) {
		return []media.MusicAlbum{}, nil
	}

	if end > len(allAlbums) {
		end = len(allAlbums)
	}

	return allAlbums[start:end], nil
}

func (r *MusicRepository) CountMusicTracksByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			count++
		}
	}

	return count, nil
}

func (r *MusicRepository) ListMusicTracksByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]*media.MusicTrack, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all tracks for this library
	var allTracks []*media.MusicTrack
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			allTracks = append(allTracks, track)
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allTracks) {
		return []*media.MusicTrack{}, nil
	}

	if end > len(allTracks) {
		end = len(allTracks)
	}

	return allTracks[start:end], nil
}

func (r *MusicRepository) ListArtistIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect representative IDs for each artist
	artistIDs := make(map[string]int64)
	for _, track := range r.tracks {
		if track.LibraryID == libraryID {
			if _, exists := artistIDs[track.Artist]; !exists {
				artistIDs[track.Artist] = track.ID
			}
		}
	}

	// Convert to slice
	var allIDs []int64
	for _, id := range artistIDs {
		allIDs = append(allIDs, id)
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allIDs) {
		return []int64{}, nil
	}

	if end > len(allIDs) {
		end = len(allIDs)
	}

	return allIDs[start:end], nil
}

// CreateAlbum creates a new album entity
func (r *MusicRepository) CreateAlbum(ctx context.Context, album *media.Album) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if album.ID == 0 {
		album.ID = r.nextAlbumID
		r.nextAlbumID++
	}

	r.albums[album.ID] = album
	return nil
}

// GetAlbumByID retrieves an album by its ID
func (r *MusicRepository) GetAlbumByID(ctx context.Context, id int64) (*media.Album, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	album, exists := r.albums[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return album, nil
}

// CreateArtist creates a new artist entity
func (r *MusicRepository) CreateArtist(ctx context.Context, artist *media.Artist) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if artist.ID == 0 {
		artist.ID = r.nextArtistID
		r.nextArtistID++
	}

	r.artists[artist.ID] = artist
	return nil
}

// GetArtistByID retrieves an artist by its ID
func (r *MusicRepository) GetArtistByID(ctx context.Context, id int64) (*media.Artist, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	artist, exists := r.artists[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return artist, nil
}

// CreateMusicTrackWithEntities atomically creates a music track along with artist and album entities if needed
func (r *MusicRepository) CreateMusicTrackWithEntities(ctx context.Context, track *media.MusicTrack, artist *media.Artist, album *media.Album) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Create artist if provided
	if artist != nil && artist.ID == 0 {
		artist.ID = r.nextArtistID
		r.nextArtistID++
		r.artists[artist.ID] = artist
	}

	// Create album if provided
	if album != nil && album.ID == 0 {
		album.ID = r.nextAlbumID
		r.nextAlbumID++
		r.albums[album.ID] = album
	}

	// Create track
	if track.ID == 0 {
		track.ID = r.nextID
		r.nextID++
	}

	// Link artist and album IDs if provided
	if artist != nil {
		track.ArtistID = artist.ID
	}
	if album != nil {
		track.AlbumID = album.ID
	}

	r.tracks[track.ID] = track
	return nil
}

// FindAlbumByTitle finds an album by library, title, and album artist
func (r *MusicRepository) FindAlbumByTitle(ctx context.Context, libraryID int64, title, albumArtist string) (*media.Album, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, album := range r.albums {
		if album.LibraryID == libraryID && album.Title == title && album.AlbumArtist == albumArtist {
			return album, nil
		}
	}

	return nil, sql.ErrNoRows
}

// ListAlbumsByLibrary retrieves all album entities in a library
func (r *MusicRepository) ListAlbumsByLibrary(ctx context.Context, libraryID int64) ([]*media.Album, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.Album
	for _, album := range r.albums {
		if album.LibraryID == libraryID {
			result = append(result, album)
		}
	}

	return result, nil
}

// FindArtistByName finds an artist by library and name
func (r *MusicRepository) FindArtistByName(ctx context.Context, libraryID int64, name string) (*media.Artist, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, artist := range r.artists {
		if artist.LibraryID == libraryID && artist.Name == name {
			return artist, nil
		}
	}

	return nil, sql.ErrNoRows
}

// ListArtistsByLibrary retrieves all artist entities in a library
func (r *MusicRepository) ListArtistsByLibrary(ctx context.Context, libraryID int64) ([]*media.Artist, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.Artist
	for _, artist := range r.artists {
		if artist.LibraryID == libraryID {
			result = append(result, artist)
		}
	}

	return result, nil
}

// ListMusicTracksByAlbumID retrieves all tracks for a specific album ID
func (r *MusicRepository) ListMusicTracksByAlbumID(ctx context.Context, albumID int64) ([]*media.MusicTrack, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.MusicTrack
	for _, track := range r.tracks {
		if track.AlbumID == albumID {
			result = append(result, track)
		}
	}

	return result, nil
}
