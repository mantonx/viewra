package library

import (
	"context"
	"errors"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Tests for processMovie

func TestScanLibraryUseCase_processMovie(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name      string
		libraryID int64
		result    *scanner.ScanResult
		setupRepo func(*mocks.MediaRepository, *mocks.MovieRepository)
		checkRepo func(*testing.T, *mocks.MediaRepository, *mocks.MovieRepository)
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// No existing movie
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie created, got %d", len(movies))
				}
				for _, movie := range movies {
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Create existing media entry in both repos
				mediaRepo.WithMedia(&media.Media{
					ID:        50,
					LibraryID: 1,
					Title:     "Original Title",
					FilePath:  "/movies/existing.mp4",
					Duration:  3600,
				})
				movieRepo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        50,
						LibraryID: 1,
						Title:     "Original Title",
						FilePath:  "/movies/existing.mp4",
						Duration:  3600,
					},
					Year: 2019,
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie updated, got %d", len(movies))
				}
				for _, movie := range movies {
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie created, got %d", len(movies))
				}
				for _, movie := range movies {
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movieRepo.WithCreateError(errors.New("database error"))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Should not panic, error logged but processing continues
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 0 {
					t.Errorf("Expected 0 movies due to error, got %d", len(movies))
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        60,
					LibraryID: 1,
					FilePath:  "/movies/update-error.mp4",
				})
				movieRepo.WithUpdateError(errors.New("update failed"))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Should not panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				movieRepo: movieRepo,
			}

			// Call processMovie with checkpoint
			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			uc.processMovie(context.Background(), tt.libraryID, tt.result, checkpoint)

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
		setupRepo func(*mocks.MediaRepository, *mocks.TVRepository)
		checkRepo func(*testing.T, *mocks.MediaRepository, *mocks.TVRepository)
	}{
		{
			name:      "create new TV episode",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/The Show/Season 01/The Show - S01E05 - Episode Title.mp4",
				Title:         "Episode Title",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(episodes))
				}
				for _, ep := range episodes {
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
				FilePath:      "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
				Title:         "Updated Episode",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2800,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        70,
					LibraryID: 2,
					FilePath:  "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
					Title:     "Old Title",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        70,
						LibraryID: 2,
						FilePath:  "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
						Title:     "Old Title",
					},
					Season:  1,
					Episode: 5,
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode updated, got %d", len(episodes))
				}
				for _, ep := range episodes {
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
				FilePath:      "/tv/Another Show/Season 1/Another Show - S01E01 - No Numbers.mp4",
				Title:         "No Numbers",
				SeasonNumber:  nil, // Will be parsed from filename
				EpisodeNumber: nil, // Will be parsed from filename
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(episodes))
				}
				for _, ep := range episodes {
					// Parser will extract S01E01 from filename
					if ep.Season != 1 {
						t.Errorf("Season = %v, want 1 (parsed from filename)", ep.Season)
					}
					if ep.Episode != 1 {
						t.Errorf("Episode = %v, want 1 (parsed from filename)", ep.Episode)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				tvRepo:    tvRepo,
			}

			// Call processTVEpisode with checkpoint
			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			uc.processTVEpisode(context.Background(), tt.libraryID, tt.result, checkpoint)

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
		setupRepo func(*mocks.MediaRepository, *mocks.MusicRepository)
		checkRepo func(*testing.T, *mocks.MediaRepository, *mocks.MusicRepository)
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				for _, track := range tracks {
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        80,
					LibraryID: 3,
					FilePath:  "/music/existing.mp3",
					Title:     "Old Song",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        80,
						LibraryID: 3,
						FilePath:  "/music/existing.mp3",
						Title:     "Old Song",
					},
					Artist: "Old Artist",
					Album:  "Old Album",
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track updated, got %d", len(tracks))
				}
				for _, track := range tracks {
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
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				for _, track := range tracks {
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
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepo: mediaRepo,
				musicRepo: musicRepo,
			}

			uc.processMusicTrack(context.Background(), tt.libraryID, tt.result, nil)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, musicRepo)
			}
		})
	}
}
