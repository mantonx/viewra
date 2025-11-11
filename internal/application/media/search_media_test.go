package media

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viewra/viewra/internal/domain/media"
)

// Mock movie repository
type mockMovieRepository struct {
	movies map[int64]*media.Movie
	nextID int64
}

func newMockMovieRepository() *mockMovieRepository {
	return &mockMovieRepository{
		movies: make(map[int64]*media.Movie),
		nextID: 1,
	}
}

func (m *mockMovieRepository) CreateMovie(ctx context.Context, movie *media.Movie) error {
	movie.ID = m.nextID
	m.nextID++
	m.movies[movie.ID] = movie
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
	var result []*media.Movie
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			result = append(result, movie)
		}
	}
	return result, nil
}

func (m *mockMovieRepository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	if _, ok := m.movies[movie.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.movies[movie.ID] = movie
	return nil
}

func (m *mockMovieRepository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	var result []*media.Movie
	query = strings.ToLower(query)
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID && strings.Contains(strings.ToLower(movie.Title), query) {
			result = append(result, movie)
		}
	}
	return result, nil
}

// Mock TV repository
type mockTVRepository struct {
	episodes map[int64]*media.TVEpisode
	nextID   int64
}

func newMockTVRepository() *mockTVRepository {
	return &mockTVRepository{
		episodes: make(map[int64]*media.TVEpisode),
		nextID:   1,
	}
}

func (m *mockTVRepository) CreateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	episode.ID = m.nextID
	m.nextID++
	m.episodes[episode.ID] = episode
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
	var result []*media.TVEpisode
	for _, episode := range m.episodes {
		if episode.LibraryID == libraryID {
			result = append(result, episode)
		}
	}
	return result, nil
}

func (m *mockTVRepository) ListTVEpisodesByShow(ctx context.Context, libraryID int64, showTitle string) ([]*media.TVEpisode, error) {
	var result []*media.TVEpisode
	for _, episode := range m.episodes {
		if episode.LibraryID == libraryID && episode.ShowTitle == showTitle {
			result = append(result, episode)
		}
	}
	return result, nil
}

func (m *mockTVRepository) UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	if _, ok := m.episodes[episode.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.episodes[episode.ID] = episode
	return nil
}

func (m *mockTVRepository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	var result []*media.TVEpisode
	query = strings.ToLower(query)
	for _, episode := range m.episodes {
		if episode.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(episode.ShowTitle), query) ||
				strings.Contains(strings.ToLower(episode.EpisodeTitle), query) {
				result = append(result, episode)
			}
		}
	}
	return result, nil
}

// Mock music repository
type mockMusicRepository struct {
	tracks map[int64]*media.MusicTrack
	nextID int64
}

func newMockMusicRepository() *mockMusicRepository {
	return &mockMusicRepository{
		tracks: make(map[int64]*media.MusicTrack),
		nextID: 1,
	}
}

func (m *mockMusicRepository) CreateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	track.ID = m.nextID
	m.nextID++
	m.tracks[track.ID] = track
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
	var result []*media.MusicTrack
	for _, track := range m.tracks {
		if track.LibraryID == libraryID {
			result = append(result, track)
		}
	}
	return result, nil
}

func (m *mockMusicRepository) ListMusicTracksByAlbum(ctx context.Context, libraryID int64, album string) ([]*media.MusicTrack, error) {
	var result []*media.MusicTrack
	for _, track := range m.tracks {
		if track.LibraryID == libraryID && track.Album == album {
			result = append(result, track)
		}
	}
	return result, nil
}

func (m *mockMusicRepository) ListMusicTracksByArtist(ctx context.Context, libraryID int64, artist string) ([]*media.MusicTrack, error) {
	var result []*media.MusicTrack
	for _, track := range m.tracks {
		if track.LibraryID == libraryID && track.Artist == artist {
			result = append(result, track)
		}
	}
	return result, nil
}

func (m *mockMusicRepository) UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error {
	if _, ok := m.tracks[track.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.tracks[track.ID] = track
	return nil
}

func (m *mockMusicRepository) SearchMusicTracks(ctx context.Context, libraryID int64, query string) ([]*media.MusicTrack, error) {
	var result []*media.MusicTrack
	query = strings.ToLower(query)
	for _, track := range m.tracks {
		if track.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(track.Title), query) ||
				strings.Contains(strings.ToLower(track.Artist), query) ||
				strings.Contains(strings.ToLower(track.Album), query) {
				result = append(result, track)
			}
		}
	}
	return result, nil
}

// Tests
func TestSearchMediaUseCase_SearchMovies(t *testing.T) {
	tests := []struct {
		name          string
		libraryID     int64
		query         string
		expectedCount int
		wantErr       bool
		setup         func(*mockMovieRepository)
	}{
		{
			name:          "search with results",
			libraryID:     1,
			query:         "Inception",
			expectedCount: 1,
			wantErr:       false,
			setup: func(repo *mockMovieRepository) {
				_ = repo.CreateMovie(context.Background(), &media.Movie{
					Media: media.Media{
						LibraryID: 1,
						Title:     "Inception",
						FilePath:  "movies/inception.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				})
			},
		},
		{
			name:          "search with no results",
			libraryID:     1,
			query:         "NonExistent",
			expectedCount: 0,
			wantErr:       false,
			setup: func(repo *mockMovieRepository) {
				_ = repo.CreateMovie(context.Background(), &media.Movie{
					Media: media.Media{
						LibraryID: 1,
						Title:     "Inception",
						FilePath:  "movies/inception.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				})
			},
		},
		{
			name:          "empty query",
			libraryID:     1,
			query:         "",
			expectedCount: 0,
			wantErr:       true,
			setup:         func(repo *mockMovieRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movieRepo := newMockMovieRepository()
			if tt.setup != nil {
				tt.setup(movieRepo)
			}

			uc := NewSearchMediaUseCase(
				newMockMediaRepository(),
				movieRepo,
				newMockTVRepository(),
				newMockMusicRepository(),
			)

			resp, err := uc.SearchMovies(context.Background(), tt.libraryID, tt.query)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchMovies() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SearchMovies() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.expectedCount {
				t.Errorf("SearchMovies() total = %v, want %v", resp.Total, tt.expectedCount)
			}
		})
	}
}

func TestSearchMediaUseCase_SearchTVEpisodes(t *testing.T) {
	tvRepo := newMockTVRepository()
	_ = tvRepo.CreateTVEpisode(context.Background(), &media.TVEpisode{
		Media: media.Media{
			LibraryID: 1,
			Title:     "Breaking Bad S01E01",
			FilePath:  "tv/breaking-bad/s01e01.mp4",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ShowTitle:    "Breaking Bad",
		EpisodeTitle: "Pilot",
		Season:       1,
		Episode:      1,
	})

	uc := NewSearchMediaUseCase(
		newMockMediaRepository(),
		newMockMovieRepository(),
		tvRepo,
		newMockMusicRepository(),
	)

	resp, err := uc.SearchTVEpisodes(context.Background(), 1, "Breaking")

	if err != nil {
		t.Errorf("SearchTVEpisodes() unexpected error = %v", err)
		return
	}

	if resp.Total != 1 {
		t.Errorf("SearchTVEpisodes() total = %v, want 1", resp.Total)
	}
}

func TestSearchMediaUseCase_SearchMusicTracks(t *testing.T) {
	musicRepo := newMockMusicRepository()
	_ = musicRepo.CreateMusicTrack(context.Background(), &media.MusicTrack{
		Media: media.Media{
			LibraryID: 1,
			Title:     "Bohemian Rhapsody",
			FilePath:  "music/queen/bohemian-rhapsody.mp3",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Artist: "Queen",
		Album:  "A Night at the Opera",
	})

	uc := NewSearchMediaUseCase(
		newMockMediaRepository(),
		newMockMovieRepository(),
		newMockTVRepository(),
		musicRepo,
	)

	resp, err := uc.SearchMusicTracks(context.Background(), 1, "Queen")

	if err != nil {
		t.Errorf("SearchMusicTracks() unexpected error = %v", err)
		return
	}

	if resp.Total != 1 {
		t.Errorf("SearchMusicTracks() total = %v, want 1", resp.Total)
	}
}
