package library

import (
	"context"
	"errors"
	"testing"

	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/domain/scanner"
)

// Mock media repositories for testing

type mockMediaRepository struct {
	media         map[int64]*media.Media
	nextID        int64
	getByFilePathErr error
}

func newMockMediaRepository() *mockMediaRepository {
	return &mockMediaRepository{
		media:  make(map[int64]*media.Media),
		nextID: 1,
	}
}

func (m *mockMediaRepository) Create(ctx context.Context, med *media.Media) error {
	med.ID = m.nextID
	m.nextID++
	m.media[med.ID] = med
	return nil
}

func (m *mockMediaRepository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	med, ok := m.media[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return med, nil
}

func (m *mockMediaRepository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	if m.getByFilePathErr != nil {
		return nil, m.getByFilePathErr
	}

	for _, med := range m.media {
		if med.LibraryID == libraryID && med.FilePath == filePath {
			return med, nil
		}
	}
	return nil, media.ErrMediaNotFound
}

func (m *mockMediaRepository) ListAll(ctx context.Context) ([]*media.Media, error) {
	result := make([]*media.Media, 0, len(m.media))
	for _, med := range m.media {
		result = append(result, med)
	}
	return result, nil
}

func (m *mockMediaRepository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	var result []*media.Media
	for _, med := range m.media {
		if med.LibraryID == libraryID {
			result = append(result, med)
		}
	}
	return result, nil
}

func (m *mockMediaRepository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	return nil, nil
}

func (m *mockMediaRepository) Update(ctx context.Context, med *media.Media) error {
	if _, ok := m.media[med.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.media[med.ID] = med
	return nil
}

func (m *mockMediaRepository) Delete(ctx context.Context, id int64) error {
	delete(m.media, id)
	return nil
}

func (m *mockMediaRepository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	for _, med := range m.media {
		if med.LibraryID == libraryID && med.FilePath == filePath {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockMediaRepository) Count(ctx context.Context, libraryID int64) (int64, error) {
	var count int64
	for _, med := range m.media {
		if med.LibraryID == libraryID {
			count++
		}
	}
	return count, nil
}

func (m *mockMediaRepository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	return 0, nil
}

type mockMovieRepository struct {
	movies       map[int64]*media.Movie
	nextID       int64
	createErr    error
	updateErr    error
}

func newMockMovieRepository() *mockMovieRepository {
	return &mockMovieRepository{
		movies: make(map[int64]*media.Movie),
		nextID: 100,
	}
}

func (m *mockMovieRepository) CreateMovie(ctx context.Context, movie *media.Movie) error {
	if m.createErr != nil {
		return m.createErr
	}
	movie.Media.ID = m.nextID
	m.nextID++
	m.movies[movie.Media.ID] = movie
	return nil
}

func (m *mockMovieRepository) GetMovieByID(ctx context.Context, id int64) (*media.Movie, error) {
	movie, ok := m.movies[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return movie, nil
}

func (m *mockMovieRepository) ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*media.Movie, error) {
	return nil, nil
}

func (m *mockMovieRepository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.movies[movie.Media.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.movies[movie.Media.ID] = movie
	return nil
}

func (m *mockMovieRepository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	return nil, nil
}

type mockTVRepository struct {
	episodes  map[int64]*media.TVEpisode
	nextID    int64
	createErr error
	updateErr error
}

func newMockTVRepository() *mockTVRepository {
	return &mockTVRepository{
		episodes: make(map[int64]*media.TVEpisode),
		nextID:   200,
	}
}

func (m *mockTVRepository) CreateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	if m.createErr != nil {
		return m.createErr
	}
	episode.Media.ID = m.nextID
	m.nextID++
	m.episodes[episode.Media.ID] = episode
	return nil
}

func (m *mockTVRepository) GetTVEpisodeByID(ctx context.Context, id int64) (*media.TVEpisode, error) {
	episode, ok := m.episodes[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return episode, nil
}

func (m *mockTVRepository) ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*media.TVEpisode, error) {
	return nil, nil
}

func (m *mockTVRepository) ListTVEpisodesByShow(ctx context.Context, libraryID int64, showTitle string) ([]*media.TVEpisode, error) {
	return nil, nil
}

func (m *mockTVRepository) UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.episodes[episode.Media.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.episodes[episode.Media.ID] = episode
	return nil
}

func (m *mockTVRepository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	return nil, nil
}

type mockMusicRepository struct {
	tracks    map[int64]*media.MusicTrack
	nextID    int64
	createErr error
	updateErr error
}

func newMockMusicRepository() *mockMusicRepository {
	return &mockMusicRepository{
		tracks: make(map[int64]*media.MusicTrack),
		nextID: 300,
	}
}

func (m *mockMusicRepository) CreateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	if m.createErr != nil {
		return m.createErr
	}
	track.Media.ID = m.nextID
	m.nextID++
	m.tracks[track.Media.ID] = track
	return nil
}

func (m *mockMusicRepository) GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error) {
	track, ok := m.tracks[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return track, nil
}

func (m *mockMusicRepository) ListMusicTracksByLibrary(ctx context.Context, libraryID int64) ([]*media.MusicTrack, error) {
	return nil, nil
}

func (m *mockMusicRepository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	return nil, nil
}

func (m *mockMusicRepository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	return nil, nil
}

func (m *mockMusicRepository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.tracks[track.Media.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.tracks[track.Media.ID] = track
	return nil
}

func (m *mockMusicRepository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	return nil, nil
}

// Tests for processMovie

func TestScanLibraryUseCase_processMovie(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name      string
		libraryID int64
		result    *scanner.ScanResult
		setupRepo func(*mockMediaRepository, *mockMovieRepository)
		checkRepo func(*testing.T, *mockMediaRepository, *mockMovieRepository)
	}{
		{
			name:      "create new movie",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/test.mp4",
				Title:    "Test Movie",
				Year:     &year2020,
				Duration: 7200,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				// No existing movie
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				if len(movieRepo.movies) != 1 {
					t.Errorf("Expected 1 movie created, got %d", len(movieRepo.movies))
				}
				for _, movie := range movieRepo.movies {
					if movie.Media.Title != "Test Movie" {
						t.Errorf("Title = %v, want Test Movie", movie.Media.Title)
					}
					if movie.Year != 2020 {
						t.Errorf("Year = %v, want 2020", movie.Year)
					}
					if movie.Media.Duration != 7200 {
						t.Errorf("Duration = %v, want 7200", movie.Media.Duration)
					}
				}
			},
		},
		{
			name:      "update existing movie",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/existing.mp4",
				Title:    "Updated Title",
				Year:     &year2020,
				Duration: 5400,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				// Create existing media entry in both repos
				mediaRepo.media[50] = &media.Media{
					ID:        50,
					LibraryID: 1,
					Title:     "Original Title",
					FilePath:  "/movies/existing.mp4",
					Duration:  3600,
				}
				movieRepo.movies[50] = &media.Movie{
					Media: media.Media{
						ID:        50,
						LibraryID: 1,
						Title:     "Original Title",
						FilePath:  "/movies/existing.mp4",
						Duration:  3600,
					},
					Year: 2019,
				}
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				if len(movieRepo.movies) != 1 {
					t.Errorf("Expected 1 movie updated, got %d", len(movieRepo.movies))
				}
				for _, movie := range movieRepo.movies {
					if movie.Media.Title != "Updated Title" {
						t.Errorf("Title = %v, want Updated Title", movie.Media.Title)
					}
					if movie.Media.ID != 50 {
						t.Errorf("ID = %v, want 50 (existing)", movie.Media.ID)
					}
				}
			},
		},
		{
			name:      "movie with nil year",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/no-year.mp4",
				Title:    "No Year Movie",
				Year:     nil,
				Duration: 3600,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				if len(movieRepo.movies) != 1 {
					t.Errorf("Expected 1 movie created, got %d", len(movieRepo.movies))
				}
				for _, movie := range movieRepo.movies {
					if movie.Year != 0 {
						t.Errorf("Year = %v, want 0 (default)", movie.Year)
					}
				}
			},
		},
		{
			name:      "handle create error gracefully",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/error.mp4",
				Title:    "Error Movie",
				Duration: 3600,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				movieRepo.createErr = errors.New("database error")
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				// Should not panic, error logged but processing continues
				if len(movieRepo.movies) != 0 {
					t.Errorf("Expected 0 movies due to error, got %d", len(movieRepo.movies))
				}
			},
		},
		{
			name:      "handle update error gracefully",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/update-error.mp4",
				Title:    "Update Error",
				Duration: 3600,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				mediaRepo.media[60] = &media.Media{
					ID:        60,
					LibraryID: 1,
					FilePath:  "/movies/update-error.mp4",
				}
				movieRepo.updateErr = errors.New("update failed")
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, movieRepo *mockMovieRepository) {
				// Should not panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := newMockMediaRepository()
			movieRepo := newMockMovieRepository()

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				movieRepo: movieRepo,
			}

			// Call processMovie
			uc.processMovie(context.Background(), tt.libraryID, tt.result)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, movieRepo)
			}
		})
	}
}

// Tests for processTVEpisode

func TestScanLibraryUseCase_processTVEpisode(t *testing.T) {
	season1 := 1
	episode5 := 5

	tests := []struct {
		name      string
		libraryID int64
		result    *scanner.ScanResult
		setupRepo func(*mockMediaRepository, *mockTVRepository)
		checkRepo func(*testing.T, *mockMediaRepository, *mockTVRepository)
	}{
		{
			name:      "create new TV episode",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/show/s01e05.mp4",
				Title:         "Episode Title",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
				if len(tvRepo.episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(tvRepo.episodes))
				}
				for _, ep := range tvRepo.episodes {
					if ep.Media.Title != "Episode Title" {
						t.Errorf("Title = %v, want Episode Title", ep.Media.Title)
					}
					if ep.Season != 1 {
						t.Errorf("Season = %v, want 1", ep.Season)
					}
					if ep.Episode != 5 {
						t.Errorf("Episode = %v, want 5", ep.Episode)
					}
				}
			},
		},
		{
			name:      "update existing TV episode",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/existing.mp4",
				Title:         "Updated Episode",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2800,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
				mediaRepo.media[70] = &media.Media{
					ID:        70,
					LibraryID: 2,
					FilePath:  "/tv/existing.mp4",
					Title:     "Old Title",
				}
				tvRepo.episodes[70] = &media.TVEpisode{
					Media: media.Media{
						ID:        70,
						LibraryID: 2,
						FilePath:  "/tv/existing.mp4",
						Title:     "Old Title",
					},
					Season:  1,
					Episode: 1,
				}
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
				if len(tvRepo.episodes) != 1 {
					t.Errorf("Expected 1 episode updated, got %d", len(tvRepo.episodes))
				}
				for _, ep := range tvRepo.episodes {
					if ep.Media.ID != 70 {
						t.Errorf("ID = %v, want 70 (existing)", ep.Media.ID)
					}
					if ep.Media.Title != "Updated Episode" {
						t.Errorf("Title = %v, want Updated Episode", ep.Media.Title)
					}
				}
			},
		},
		{
			name:      "episode with nil season/episode numbers",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/no-numbers.mp4",
				Title:         "No Numbers",
				SeasonNumber:  nil,
				EpisodeNumber: nil,
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, tvRepo *mockTVRepository) {
				if len(tvRepo.episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(tvRepo.episodes))
				}
				for _, ep := range tvRepo.episodes {
					if ep.Season != 0 {
						t.Errorf("Season = %v, want 0 (default)", ep.Season)
					}
					if ep.Episode != 0 {
						t.Errorf("Episode = %v, want 0 (default)", ep.Episode)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := newMockMediaRepository()
			tvRepo := newMockTVRepository()

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				tvRepo:    tvRepo,
			}

			uc.processTVEpisode(context.Background(), tt.libraryID, tt.result)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, tvRepo)
			}
		})
	}
}

// Tests for processMusicTrack

func TestScanLibraryUseCase_processMusicTrack(t *testing.T) {
	track3 := 3
	year2021 := 2021

	tests := []struct {
		name      string
		libraryID int64
		result    *scanner.ScanResult
		setupRepo func(*mockMediaRepository, *mockMusicRepository)
		checkRepo func(*testing.T, *mockMediaRepository, *mockMusicRepository)
	}{
		{
			name:      "create new music track",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/album/track03.mp3",
				Title:       "Song Title",
				Artist:      "Artist Name",
				Album:       "Album Name",
				TrackNumber: &track3,
				Year:        &year2021,
				Duration:    180,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
				if len(musicRepo.tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(musicRepo.tracks))
				}
				for _, track := range musicRepo.tracks {
					if track.Media.Title != "Song Title" {
						t.Errorf("Title = %v, want Song Title", track.Media.Title)
					}
					if track.Artist != "Artist Name" {
						t.Errorf("Artist = %v, want Artist Name", track.Artist)
					}
					if track.Album != "Album Name" {
						t.Errorf("Album = %v, want Album Name", track.Album)
					}
					if track.TrackNumber != 3 {
						t.Errorf("TrackNumber = %v, want 3", track.TrackNumber)
					}
					if track.Year != 2021 {
						t.Errorf("Year = %v, want 2021", track.Year)
					}
				}
			},
		},
		{
			name:      "update existing music track",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/existing.mp3",
				Title:    "Updated Song",
				Artist:   "Updated Artist",
				Album:    "Updated Album",
				Duration: 200,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
				mediaRepo.media[80] = &media.Media{
					ID:        80,
					LibraryID: 3,
					FilePath:  "/music/existing.mp3",
					Title:     "Old Song",
				}
				musicRepo.tracks[80] = &media.MusicTrack{
					Media: media.Media{
						ID:        80,
						LibraryID: 3,
						FilePath:  "/music/existing.mp3",
						Title:     "Old Song",
					},
					Artist: "Old Artist",
					Album:  "Old Album",
				}
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
				if len(musicRepo.tracks) != 1 {
					t.Errorf("Expected 1 track updated, got %d", len(musicRepo.tracks))
				}
				for _, track := range musicRepo.tracks {
					if track.Media.ID != 80 {
						t.Errorf("ID = %v, want 80 (existing)", track.Media.ID)
					}
					if track.Media.Title != "Updated Song" {
						t.Errorf("Title = %v, want Updated Song", track.Media.Title)
					}
					if track.Artist != "Updated Artist" {
						t.Errorf("Artist = %v, want Updated Artist", track.Artist)
					}
				}
			},
		},
		{
			name:      "track with minimal metadata",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/minimal.mp3",
				Title:       "Minimal Track",
				Artist:      "",
				Album:       "",
				TrackNumber: nil,
				Year:        nil,
				Duration:    150,
			},
			setupRepo: func(mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mockMediaRepository, musicRepo *mockMusicRepository) {
				if len(musicRepo.tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(musicRepo.tracks))
				}
				for _, track := range musicRepo.tracks {
					if track.TrackNumber != 0 {
						t.Errorf("TrackNumber = %v, want 0 (default)", track.TrackNumber)
					}
					if track.Year != 0 {
						t.Errorf("Year = %v, want 0 (default)", track.Year)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := newMockMediaRepository()
			musicRepo := newMockMusicRepository()

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				musicRepo: musicRepo,
			}

			uc.processMusicTrack(context.Background(), tt.libraryID, tt.result)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, musicRepo)
			}
		})
	}
}
