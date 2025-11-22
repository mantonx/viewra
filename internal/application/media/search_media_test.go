package media

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Tests
func TestSearchMediaUseCase_SearchMovies(t *testing.T) {
	tests := []struct {
		name          string
		libraryID     int64
		query         string
		expectedCount int
		wantErr       bool
		setup         func(*mocks.MovieRepository)
	}{
		{
			name:          "search with results",
			libraryID:     1,
			query:         "Inception",
			expectedCount: 1,
			wantErr:       false,
			setup: func(repo *mocks.MovieRepository) {
				repo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        1,
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
			setup: func(repo *mocks.MovieRepository) {
				repo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        1,
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
			setup:         func(repo *mocks.MovieRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movieRepo := mocks.NewMovieRepository(t)
			if tt.setup != nil {
				tt.setup(movieRepo)
			}

			uc := NewSearchMediaUseCase(
				mocks.NewMediaRepository(t),
				movieRepo,
				mocks.NewTVRepository(t),
				mocks.NewMusicRepository(t),
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
	tvRepo := mocks.NewTVRepository(t)
	tvRepo.WithEpisodes(&media.TVEpisode{
		Media: media.Media{
			ID:        1,
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
		mocks.NewMediaRepository(t),
		mocks.NewMovieRepository(t),
		tvRepo,
		mocks.NewMusicRepository(t),
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
	musicRepo := mocks.NewMusicRepository(t)
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        1,
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
		mocks.NewMediaRepository(t),
		mocks.NewMovieRepository(t),
		mocks.NewTVRepository(t),
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
