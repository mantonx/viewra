package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	domainLibrary "github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// discardLogger returns a logger that discards all output (for tests).
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Tests for processMovie

func TestScanLibraryUseCase_processMovie(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.MovieRepository)
		setupCache func(*sync.Map) // Pre-populate the existingMediaCache for update tests
		checkRepo  func(*testing.T, *mocks.MediaRepository, *mocks.MovieRepository)
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
			setupCache: func(cache *sync.Map) {
				// Pre-populate cache so processMovie knows media exists
				cache.Store("/movies/existing.mp4", int64(50))
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
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeMovies,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					Movie:   movieRepo,
				},
				logger: discardLogger(),
			}

			// Call processMovie with checkpoint
			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}
			_, _ = uc.processMovie(context.Background(), tt.libraryID, tt.result, checkpoint, existingMediaCache)

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
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.TVRepository)
		setupCache func(*sync.Map)
		checkRepo  func(*testing.T, *mocks.MediaRepository, *mocks.TVRepository)
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
			setupCache: func(cache *sync.Map) {
				cache.Store("/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4", int64(70))
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
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test TV Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeTV,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					TV:      tvRepo,
				},
				logger: discardLogger(),
			}

			// Call processTVEpisode with checkpoint
			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}
			_, _ = uc.processTVEpisode(context.Background(), tt.libraryID, tt.result, checkpoint, existingMediaCache)

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
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.MusicRepository)
		setupCache func(*sync.Map)
		checkRepo  func(*testing.T, *mocks.MediaRepository, *mocks.MusicRepository)
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
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/existing.mp3", int64(80))
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
		{
			name:      "track with artist and album entities created",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/new-artist/track01.mp3",
				Title:       "New Track",
				Artist:      "New Artist",
				Album:       "New Album",
				TrackNumber: &track3,
				Duration:    200,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				artists, _ := musicRepo.ListArtistsByLibrary(context.Background(), 3)
				if len(artists) != 1 {
					t.Errorf("Expected 1 artist created, got %d", len(artists))
				}
				albums, _ := musicRepo.ListAlbumsByLibrary(context.Background(), 3)
				if len(albums) != 1 {
					t.Errorf("Expected 1 album created, got %d", len(albums))
				}
			},
		},
		{
			name:      "track update links to existing artist and album",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track.mp3",
				Title:    "Track Title",
				Artist:   "Existing Artist",
				Album:    "Existing Album",
				Duration: 180,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Pre-create artist and album
				artist := &media.Artist{ID: 10, LibraryID: 3, Name: "Existing Artist"}
				album := &media.Album{ID: 20, LibraryID: 3, Title: "Existing Album", AlbumArtist: "Existing Artist"}
				musicRepo.CreateArtist(context.Background(), artist)
				musicRepo.CreateAlbum(context.Background(), album)

				// Pre-create media and track entries
				mediaRepo.WithMedia(&media.Media{
					ID:        90,
					LibraryID: 3,
					FilePath:  "/music/track.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        90,
						LibraryID: 3,
						FilePath:  "/music/track.mp3",
						Type:      "music_track",
					},
					Artist:   "Old Artist",
					Album:    "Old Album",
					ArtistID: 0,
					AlbumID:  0,
				})
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/track.mp3", int64(90))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track, got %d", len(tracks))
				}
				for _, track := range tracks {
					if track.ArtistID != 10 {
						t.Errorf("ArtistID = %v, want 10", track.ArtistID)
					}
					if track.AlbumID != 20 {
						t.Errorf("AlbumID = %v, want 20", track.AlbumID)
					}
				}
			},
		},
		{
			name:      "handle race condition with unique constraint error",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race.mp3",
				Title:    "Race Track",
				Artist:   "Race Artist",
				Duration: 180,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error on first create
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed: media.file_path"))
				// After error, we'll fetch existing media
				mediaRepo.WithMedia(&media.Media{
					ID:        95,
					LibraryID: 3,
					FilePath:  "/music/race.mp3",
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Should handle gracefully - update existing record
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test Music Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeMusic,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					Music:   musicRepo,
				},
				logger: discardLogger(),
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}
			_, _ = uc.processMusicTrack(context.Background(), tt.libraryID, tt.result, checkpoint, existingMediaCache)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, musicRepo)
			}
		})
	}
}

// Additional comprehensive tests for processMovie

func TestScanLibraryUseCase_processMovie_Comprehensive(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.MovieRepository)
		setupCache    func(*sync.Map)
		checkRepo     func(*testing.T, *mocks.MediaRepository, *mocks.MovieRepository)
		expectMediaID bool
		expectError   bool
	}{
		{
			name:      "skip audio file in movie library",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/soundtrack.mp3",
				Title:    "Soundtrack",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/soundtrack.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 0 {
					t.Errorf("Expected 0 movies (audio file skipped), got %d", len(movies))
				}
			},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "movie with nil checkpoint",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/test.mp4",
				Title:    "Test Movie",
				Year:     &year2020,
				Duration: 7200,
			},
			checkpoint: nil, // Test nil checkpoint handling
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie, got %d", len(movies))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "movie marked as extra",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/Movie-trailer.mp4",
				Title:    "Movie Trailer",
				Duration: 120,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/Movie-trailer.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie, got %d", len(movies))
					return
				}
				if !movies[0].Media.IsExtra {
					t.Errorf("Expected IsExtra=true for trailer file")
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "movie with video metadata",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath:        "/movies/hd-movie.mp4",
				Title:           "HD Movie",
				Duration:        7200,
				Width:           1920,
				Height:          1080,
				VideoCodec:      "h264",
				AudioCodec:      "aac",
				Bitrate:         8000000,
				FrameRate:       23.976,
				ContainerFormat: "mp4",
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/hd-movie.mp4",
				FileSize: 5368709120,
				FileHash: "hash123",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie, got %d", len(movies))
					return
				}
				movie := movies[0]
				if movie.Media.Width != 1920 {
					t.Errorf("Width = %v, want 1920", movie.Media.Width)
				}
				if movie.Media.Height != 1080 {
					t.Errorf("Height = %v, want 1080", movie.Media.Height)
				}
				if movie.Media.VideoCodec != "h264" {
					t.Errorf("VideoCodec = %v, want h264", movie.Media.VideoCodec)
				}
				if movie.Media.FileSize != 5368709120 {
					t.Errorf("FileSize = %v, want 5368709120", movie.Media.FileSize)
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "handle race condition - cache miss then database fetch",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/race-movie.mp4",
				Title:    "Race Movie",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/race-movie.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Simulate unique constraint error
				movieRepo.WithCreateError(errors.New("duplicate key value violates unique constraint"))
				// Pre-populate both media and movie that will be fetched after collision
				mediaRepo.WithMedia(&media.Media{
					ID:        100,
					LibraryID: 1,
					FilePath:  "/movies/race-movie.mp4",
					Title:     "Race Movie",
					Type:      "movie",
				})
				movieRepo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        100,
						LibraryID: 1,
						FilePath:  "/movies/race-movie.mp4",
						Title:     "Race Movie",
						Type:      "movie",
					},
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Should handle race condition gracefully
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie after handling race condition, got %d", len(movies))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "cache entry added after creation",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/new-movie.mp4",
				Title:    "New Movie",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/new-movie.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Verify cache was populated (would need access to cache to check)
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie created, got %d", len(movies))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeMovies,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					Movie:   movieRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := uc.processMovie(context.Background(), tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, movieRepo)
			}
		})
	}
}

// Additional comprehensive tests for processTVEpisode

func TestScanLibraryUseCase_processTVEpisode_Comprehensive(t *testing.T) {
	season1 := 1
	episode1 := 1
	episode5 := 5
	episodeEnd2 := 2 // For multi-episode file

	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.TVRepository)
		setupCache    func(*sync.Map)
		checkRepo     func(*testing.T, *mocks.MediaRepository, *mocks.TVRepository)
		expectMediaID bool
		expectError   bool
	}{
		{
			name:      "skip audio file in TV library",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/theme.mp3",
				Title:    "Theme Song",
				Duration: 60,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/theme.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 0 {
					t.Errorf("Expected 0 episodes (audio file skipped), got %d", len(episodes))
				}
			},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "episode with nil checkpoint",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E01.mp4",
				Title:         "Episode Title",
				ShowName:      "Show Name",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: nil, // Test nil checkpoint handling
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode, got %d", len(episodes))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "episode marked as extra",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E01-deleted.mp4",
				Title:         "Deleted Scene",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      300,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01-deleted.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode, got %d", len(episodes))
					return
				}
				if !episodes[0].Media.IsExtra {
					t.Errorf("Expected IsExtra=true for deleted scene file")
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "episode with show name from result",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/My Show/Season 01/S01E05.mp4",
				Title:         "Episode 5",
				ShowName:      "My Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/My Show/Season 01/S01E05.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode, got %d", len(episodes))
					return
				}
				if episodes[0].ShowTitle != "My Show" {
					t.Errorf("ShowTitle = %v, want My Show", episodes[0].ShowTitle)
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "episode with video metadata",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:        "/tv/Show/S01E01.mkv",
				Title:           "Pilot",
				ShowName:        "Show",
				SeasonNumber:    &season1,
				EpisodeNumber:   &episode1,
				Duration:        2700,
				Width:           1920,
				Height:          1080,
				VideoCodec:      "h264",
				AudioCodec:      "ac3",
				Bitrate:         5000000,
				FrameRate:       23.976,
				ContainerFormat: "matroska",
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01.mkv",
				FileSize: 2147483648,
				FileHash: "hash456",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode, got %d", len(episodes))
					return
				}
				ep := episodes[0]
				if ep.Media.Width != 1920 {
					t.Errorf("Width = %v, want 1920", ep.Media.Width)
				}
				if ep.Media.Height != 1080 {
					t.Errorf("Height = %v, want 1080", ep.Media.Height)
				}
				if ep.Media.ContainerFormat != "matroska" {
					t.Errorf("ContainerFormat = %v, want matroska", ep.Media.ContainerFormat)
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "handle race condition on create",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E01.mp4",
				Title:         "Race Episode",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Simulate unique constraint error
				tvRepo.WithCreateError(errors.New("UNIQUE constraint failed: tv_episodes"))
				// Pre-populate both media and episode for fetch after collision
				mediaRepo.WithMedia(&media.Media{
					ID:        200,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E01.mp4",
					Type:      "tv_episode",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        200,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E01.mp4",
						Type:      "tv_episode",
					},
					ShowTitle: "Show",
					Season:    1,
					Episode:   1,
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Should handle gracefully
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode after handling race condition, got %d", len(episodes))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "multi-episode file triggers processMultiEpisodeFile",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:         "/tv/Show/S01E01-E02.mp4",
				Title:            "Double Episode",
				ShowName:         "Show",
				SeasonNumber:     &season1,
				EpisodeNumber:    &episode1,
				EpisodeEndNumber: &episodeEnd2, // Triggers multi-episode path
				Duration:         5400,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01-E02.mp4",
				FileSize: 2048,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 2 {
					t.Errorf("Expected 2 episodes from multi-episode file, got %d", len(episodes))
				}
			},
			expectMediaID: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test TV Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeTV,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					TV:      tvRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := uc.processTVEpisode(context.Background(), tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, tvRepo)
			}
		})
	}
}

// Tests for processMultiEpisodeFile

func TestScanLibraryUseCase_processMultiEpisodeFile(t *testing.T) {
	tests := []struct {
		name         string
		libraryID    int64
		result       *scanner.ScanResult
		checkpoint   *scanner.ScanCheckpoint
		showTitle    string
		season       int
		episodeStart int
		episodeEnd   int
		episodeTitle string
		setupRepo    func(*mocks.MediaRepository, *mocks.TVRepository)
		setupCache   func(*sync.Map)
		checkRepo    func(*testing.T, *mocks.MediaRepository, *mocks.TVRepository)
		expectError  bool
	}{
		{
			name:      "create two episodes from S01E01-E02 file",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:        "/tv/Show/S01E01-E02.mp4",
				Duration:        5400,
				Width:           1920,
				Height:          1080,
				VideoCodec:      "h264",
				AudioCodec:      "aac",
				Bitrate:         5000000,
				FrameRate:       23.976,
				ContainerFormat: "mp4",
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01-E02.mp4",
				FileSize: 2048,
				FileHash: "hash",
			},
			showTitle:    "Test Show",
			season:       1,
			episodeStart: 1,
			episodeEnd:   2,
			episodeTitle: "Double Episode",
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 2 {
					t.Fatalf("Expected 2 episodes, got %d", len(episodes))
				}

				// Sort episodes by episode number (order from DB is not guaranteed)
				sort.Slice(episodes, func(i, j int) bool {
					return episodes[i].Episode < episodes[j].Episode
				})

				// Verify first episode uses real path
				if episodes[0].Media.FilePath != "/tv/Show/S01E01-E02.mp4" {
					t.Errorf("First episode FilePath = %v, want /tv/Show/S01E01-E02.mp4", episodes[0].Media.FilePath)
				}
				if episodes[0].Episode != 1 {
					t.Errorf("First episode number = %v, want 1", episodes[0].Episode)
				}
				if episodes[0].EpisodeTitle != "Double Episode (Part 1)" {
					t.Errorf("First episode title = %v, want 'Double Episode (Part 1)'", episodes[0].EpisodeTitle)
				}

				// Verify second episode uses virtual path
				if episodes[1].Media.FilePath != "/tv/Show/S01E01-E02.mp4#ep2" {
					t.Errorf("Second episode FilePath = %v, want /tv/Show/S01E01-E02.mp4#ep2", episodes[1].Media.FilePath)
				}
				if episodes[1].Episode != 2 {
					t.Errorf("Second episode number = %v, want 2", episodes[1].Episode)
				}
				if episodes[1].EpisodeTitle != "Double Episode (Part 2)" {
					t.Errorf("Second episode title = %v, want 'Double Episode (Part 2)'", episodes[1].EpisodeTitle)
				}

				// Both should have same metadata
				for i, ep := range episodes {
					if ep.ShowTitle != "Test Show" {
						t.Errorf("Episode %d ShowTitle = %v, want Test Show", i+1, ep.ShowTitle)
					}
					if ep.Season != 1 {
						t.Errorf("Episode %d Season = %v, want 1", i+1, ep.Season)
					}
					if ep.Media.Duration != 5400 {
						t.Errorf("Episode %d Duration = %v, want 5400", i+1, ep.Media.Duration)
					}
				}
			},
			expectError: false,
		},
		{
			name:      "create three episodes from S01E05-E07 file",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/S01E05-E07.mkv",
				Duration: 8100,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E05-E07.mkv",
				FileSize: 4096,
				FileHash: "hash2",
			},
			showTitle:    "Another Show",
			season:       1,
			episodeStart: 5,
			episodeEnd:   7,
			episodeTitle: "Triple Feature",
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 3 {
					t.Fatalf("Expected 3 episodes, got %d", len(episodes))
				}

				// Sort episodes by episode number (order from DB is not guaranteed)
				sort.Slice(episodes, func(i, j int) bool {
					return episodes[i].Episode < episodes[j].Episode
				})

				expectedPaths := []string{
					"/tv/Show/S01E05-E07.mkv",
					"/tv/Show/S01E05-E07.mkv#ep6",
					"/tv/Show/S01E05-E07.mkv#ep7",
				}
				expectedEpisodes := []int{5, 6, 7}
				expectedTitles := []string{
					"Triple Feature (Part 1)",
					"Triple Feature (Part 2)",
					"Triple Feature (Part 3)",
				}

				for i, ep := range episodes {
					if ep.Media.FilePath != expectedPaths[i] {
						t.Errorf("Episode %d FilePath = %v, want %v", i+1, ep.Media.FilePath, expectedPaths[i])
					}
					if ep.Episode != expectedEpisodes[i] {
						t.Errorf("Episode %d number = %v, want %d", i+1, ep.Episode, expectedEpisodes[i])
					}
					if ep.EpisodeTitle != expectedTitles[i] {
						t.Errorf("Episode %d title = %v, want %v", i+1, ep.EpisodeTitle, expectedTitles[i])
					}
				}
			},
			expectError: false,
		},
		{
			name:      "update existing multi-episode entries",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/S01E01-E02.mp4",
				Duration: 5400,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01-E02.mp4",
				FileSize: 2048,
				FileHash: "updated-hash",
			},
			showTitle:    "Test Show",
			season:       1,
			episodeStart: 1,
			episodeEnd:   2,
			episodeTitle: "Updated Episode",
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Pre-create existing episodes in both media and TV repos
				mediaRepo.WithMedia(&media.Media{
					ID:        300,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E01-E02.mp4",
					Type:      "tv_episode",
				})
				mediaRepo.WithMedia(&media.Media{
					ID:        301,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E01-E02.mp4#ep2",
					Type:      "tv_episode",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        300,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E01-E02.mp4",
						Type:      "tv_episode",
					},
					ShowTitle: "Test Show",
					Season:    1,
					Episode:   1,
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        301,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E01-E02.mp4#ep2",
						Type:      "tv_episode",
					},
					ShowTitle: "Test Show",
					Season:    1,
					Episode:   2,
				})
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/tv/Show/S01E01-E02.mp4", int64(300))
				cache.Store("/tv/Show/S01E01-E02.mp4#ep2", int64(301))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 2 {
					t.Fatalf("Expected 2 episodes, got %d", len(episodes))
				}
				// Verify episodes were updated, not recreated
				for _, ep := range episodes {
					if ep.Media.ID != 300 && ep.Media.ID != 301 {
						t.Errorf("Unexpected episode ID %d, expected 300 or 301", ep.Media.ID)
					}
				}
			},
			expectError: false,
		},
		{
			name:      "empty episode title",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/S02E01-E02.mp4",
				Duration: 5400,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S02E01-E02.mp4",
				FileSize: 2048,
				FileHash: "hash",
			},
			showTitle:    "Test Show",
			season:       2,
			episodeStart: 1,
			episodeEnd:   2,
			episodeTitle: "", // Empty title
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 2 {
					t.Fatalf("Expected 2 episodes, got %d", len(episodes))
				}
				// When title is empty, part numbers should not be appended
				for _, ep := range episodes {
					if ep.EpisodeTitle != "" {
						t.Errorf("Episode title = %v, want empty string", ep.EpisodeTitle)
					}
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test TV Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeTV,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					TV:      tvRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			_, err := uc.processMultiEpisodeFile(
				context.Background(),
				tt.libraryID,
				tt.result,
				tt.checkpoint,
				existingMediaCache,
				tt.showTitle,
				tt.season,
				tt.episodeStart,
				tt.episodeEnd,
				tt.episodeTitle,
			)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, tvRepo)
			}
		})
	}
}

// Tests for processMultiEpisodeFile error paths

func TestScanLibraryUseCase_processMultiEpisodeFile_ErrorPaths(t *testing.T) {
	t.Run("handles media update error gracefully", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		tvRepo := mocks.NewTVRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
			ID:   2,
			Name: "Test TV Library",
			Path: "/test",
			Type: domainLibrary.LibraryTypeTV,
		})

		// Pre-create existing episodes in cache
		mediaRepo.WithMedia(&media.Media{
			ID:        300,
			LibraryID: 2,
			FilePath:  "/tv/Show/S01E01-E02.mp4",
			Type:      "tv_episode",
		})
		// Simulate update failure
		mediaRepo.UpdateErr = errors.New("database error")

		uc := &ScanLibraryUseCase{
			mediaRepos: &MediaRepositories{
				Library: libraryRepo,
				Media:   mediaRepo,
				TV:      tvRepo,
			},
			logger: discardLogger(),
		}

		existingMediaCache := &sync.Map{}
		existingMediaCache.Store("/tv/Show/S01E01-E02.mp4", int64(300))

		result := &scanner.ScanResult{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			Duration: 5400,
		}
		checkpoint := &scanner.ScanCheckpoint{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			FileSize: 2048,
			FileHash: "hash",
		}

		// Should continue despite update error
		_, err := uc.processMultiEpisodeFile(
			context.Background(),
			2,
			result,
			checkpoint,
			existingMediaCache,
			"Test Show",
			1,
			1,
			2,
			"Episode Title",
		)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("handles TV episode update error gracefully", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		tvRepo := mocks.NewTVRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
			ID:   2,
			Name: "Test TV Library",
			Path: "/test",
			Type: domainLibrary.LibraryTypeTV,
		})

		// Pre-create existing episodes
		mediaRepo.WithMedia(&media.Media{
			ID:        300,
			LibraryID: 2,
			FilePath:  "/tv/Show/S01E01-E02.mp4",
			Type:      "tv_episode",
		})
		tvRepo.WithEpisodes(&media.TVEpisode{
			Media: media.Media{
				ID:        300,
				LibraryID: 2,
				FilePath:  "/tv/Show/S01E01-E02.mp4",
				Type:      "tv_episode",
			},
			ShowTitle: "Test Show",
			Season:    1,
			Episode:   1,
		})
		// Simulate TV episode update failure
		tvRepo.UpdateErr = errors.New("database error")

		uc := &ScanLibraryUseCase{
			mediaRepos: &MediaRepositories{
				Library: libraryRepo,
				Media:   mediaRepo,
				TV:      tvRepo,
			},
			logger: discardLogger(),
		}

		existingMediaCache := &sync.Map{}
		existingMediaCache.Store("/tv/Show/S01E01-E02.mp4", int64(300))

		result := &scanner.ScanResult{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			Duration: 5400,
		}
		checkpoint := &scanner.ScanCheckpoint{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			FileSize: 2048,
			FileHash: "hash",
		}

		// Should continue despite update error
		_, err := uc.processMultiEpisodeFile(
			context.Background(),
			2,
			result,
			checkpoint,
			existingMediaCache,
			"Test Show",
			1,
			1,
			2,
			"Episode Title",
		)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("handles UNIQUE constraint violation gracefully", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		tvRepo := mocks.NewTVRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
			ID:   2,
			Name: "Test TV Library",
			Path: "/test",
			Type: domainLibrary.LibraryTypeTV,
		})

		// Simulate UNIQUE constraint failure on create
		tvRepo.CreateErr = errors.New("UNIQUE constraint failed: media.file_path")

		uc := &ScanLibraryUseCase{
			mediaRepos: &MediaRepositories{
				Library: libraryRepo,
				Media:   mediaRepo,
				TV:      tvRepo,
			},
			logger: discardLogger(),
		}

		existingMediaCache := &sync.Map{}

		result := &scanner.ScanResult{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			Duration: 5400,
		}
		checkpoint := &scanner.ScanCheckpoint{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			FileSize: 2048,
			FileHash: "hash",
		}

		// Should continue despite UNIQUE constraint error
		_, err := uc.processMultiEpisodeFile(
			context.Background(),
			2,
			result,
			checkpoint,
			existingMediaCache,
			"Test Show",
			1,
			1,
			2,
			"Episode Title",
		)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("handles non-unique create error gracefully", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		tvRepo := mocks.NewTVRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
			ID:   2,
			Name: "Test TV Library",
			Path: "/test",
			Type: domainLibrary.LibraryTypeTV,
		})

		// Simulate non-UNIQUE error on create
		tvRepo.CreateErr = errors.New("database connection timeout")

		uc := &ScanLibraryUseCase{
			mediaRepos: &MediaRepositories{
				Library: libraryRepo,
				Media:   mediaRepo,
				TV:      tvRepo,
			},
			logger: discardLogger(),
		}

		existingMediaCache := &sync.Map{}

		result := &scanner.ScanResult{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			Duration: 5400,
		}
		checkpoint := &scanner.ScanCheckpoint{
			FilePath: "/tv/Show/S01E01-E02.mp4",
			FileSize: 2048,
			FileHash: "hash",
		}

		// Should continue despite create error
		_, err := uc.processMultiEpisodeFile(
			context.Background(),
			2,
			result,
			checkpoint,
			existingMediaCache,
			"Test Show",
			1,
			1,
			2,
			"Episode Title",
		)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

// Tests for enrichTVShowMetadataFromNFO

func TestScanLibraryUseCase_enrichTVShowMetadataFromNFO(t *testing.T) {
	tests := []struct {
		name            string
		showID          int64
		episodeFilePath string
		nfoContent      string // NFO file content to create
		setupRepo       func(*mocks.TVRepository)
		checkRepo       func(*testing.T, *mocks.TVRepository)
	}{
		{
			name:            "successfully enrich show with valid NFO",
			showID:          1,
			episodeFilePath: "/tv/Breaking Bad/Season 01/S01E01.mp4",
			nfoContent: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<tvshow>
	<title>Breaking Bad</title>
	<year>2008</year>
	<genre>Crime</genre>
	<genre>Drama</genre>
	<genre>Thriller</genre>
	<plot>A high school chemistry teacher turned methamphetamine producer.</plot>
	<mpaa>TV-MA</mpaa>
	<imdb>tt0903747</imdb>
	<tmdbid>1396</tmdbid>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 1,
					Title:     "Breaking Bad",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 1)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 2008 {
					t.Errorf("Year = %v, want 2008", show.Year)
				}
				if len(show.Genre) != 3 {
					t.Errorf("Genre count = %v, want 3", len(show.Genre))
				}
				if show.Plot == "" {
					t.Error("Plot should be populated")
				}
				if show.ContentRating != "TV-MA" {
					t.Errorf("ContentRating = %v, want TV-MA", show.ContentRating)
				}
				if show.IMDbID != "tt0903747" {
					t.Errorf("IMDbID = %v, want tt0903747", show.IMDbID)
				}
				if show.TMDbID != 1396 {
					t.Errorf("TMDbID = %v, want 1396", show.TMDbID)
				}
			},
		},
		{
			name:            "enrich with minimal NFO fields",
			showID:          2,
			episodeFilePath: "/tv/The Wire/Season 02/S02E03.mkv",
			nfoContent: `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
	<title>The Wire</title>
	<year>2002</year>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        2,
					LibraryID: 1,
					Title:     "The Wire",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 2)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 2002 {
					t.Errorf("Year = %v, want 2002", show.Year)
				}
				// Other fields should remain unchanged (empty/zero)
				if len(show.Genre) != 0 {
					t.Errorf("Genre should be empty, got %v", show.Genre)
				}
			},
		},
		{
			name:            "NFO with all supported fields",
			showID:          3,
			episodeFilePath: "/tv/Game of Thrones/Season 01/episode.mkv",
			nfoContent: `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
	<title>Game of Thrones</title>
	<year>2011</year>
	<genre>Action</genre>
	<genre>Adventure</genre>
	<genre>Drama</genre>
	<genre>Fantasy</genre>
	<plot>Nine noble families fight for control of the mythical land of Westeros.</plot>
	<contentrating>TV-MA</contentrating>
	<imdb>tt0944947</imdb>
	<tmdbid>1399</tmdbid>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        3,
					LibraryID: 1,
					Title:     "Game of Thrones",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 3)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 2011 {
					t.Errorf("Year = %v, want 2011", show.Year)
				}
				if len(show.Genre) != 4 {
					t.Errorf("Genre count = %v, want 4", len(show.Genre))
				}
				expectedGenres := []string{"Action", "Adventure", "Drama", "Fantasy"}
				for i, genre := range show.Genre {
					if genre != expectedGenres[i] {
						t.Errorf("Genre[%d] = %v, want %v", i, genre, expectedGenres[i])
					}
				}
				if !strings.Contains(show.Plot, "Westeros") {
					t.Errorf("Plot should contain 'Westeros', got: %v", show.Plot)
				}
				if show.ContentRating != "TV-MA" {
					t.Errorf("ContentRating = %v, want TV-MA", show.ContentRating)
				}
				if show.IMDbID != "tt0944947" {
					t.Errorf("IMDbID = %v, want tt0944947", show.IMDbID)
				}
				if show.TMDbID != 1399 {
					t.Errorf("TMDbID = %v, want 1399", show.TMDbID)
				}
			},
		},
		{
			name:            "NFO with IMDbID without tt prefix",
			showID:          4,
			episodeFilePath: "/tv/The Office/Season 01/episode.mp4",
			nfoContent: `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
	<title>The Office</title>
	<year>2005</year>
	<imdb>0386676</imdb>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        4,
					LibraryID: 1,
					Title:     "The Office",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 4)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				// cleanIMDbID should add tt prefix
				if show.IMDbID != "tt0386676" {
					t.Errorf("IMDbID = %v, want tt0386676", show.IMDbID)
				}
			},
		},
		{
			name:            "handle empty show directory - no NFO found",
			showID:          5,
			episodeFilePath: "",
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        5,
					LibraryID: 1,
					Title:     "Test Show",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				// Should handle gracefully without errors
				show, err := tvRepo.GetTVShowByID(context.Background(), 5)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				// Show should remain unchanged
				if show.Year != 0 {
					t.Errorf("Year should be 0 (unchanged), got %v", show.Year)
				}
			},
		},
		{
			name:            "show not found in repository",
			showID:          999,
			episodeFilePath: "/tv/NonExistentShow/Season 01/S01E01.mp4",
			setupRepo: func(tvRepo *mocks.TVRepository) {
				// No show in repository
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				// Should log warning but not crash
			},
		},
		{
			name:            "handle parse error in NFO",
			showID:          6,
			episodeFilePath: "/tv/Bad NFO Show/Season 01/episode.mp4",
			nfoContent:      `<?xml version="1.0"?><invalid>not a tvshow nfo</broken>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        6,
					LibraryID: 1,
					Title:     "Bad NFO Show",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				// Should log warning, show remains unchanged
				show, err := tvRepo.GetTVShowByID(context.Background(), 6)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 0 {
					t.Errorf("Year should be 0 (unchanged), got %v", show.Year)
				}
			},
		},
		{
			name:            "handle update error",
			showID:          7,
			episodeFilePath: "/tv/Update Error Show/Season 01/episode.mp4",
			nfoContent: `<?xml version="1.0"?>
<tvshow>
	<title>Update Error Show</title>
	<year>2020</year>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        7,
					LibraryID: 1,
					Title:     "Update Error Show",
				})
				// Inject update error
				tvRepo.UpdateErr = errors.New("database update failed")
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				// Should log warning but not crash
			},
		},
		{
			name:            "episode in season subdirectory",
			showID:          8,
			episodeFilePath: "/tv/Show Name/Season 02/Show Name - S02E05.mkv",
			nfoContent: `<?xml version="1.0"?>
<tvshow>
	<title>Show Name</title>
	<year>2015</year>
	<genre>Sci-Fi</genre>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        8,
					LibraryID: 1,
					Title:     "Show Name",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 8)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 2015 {
					t.Errorf("Year = %v, want 2015", show.Year)
				}
				if len(show.Genre) != 1 || show.Genre[0] != "Sci-Fi" {
					t.Errorf("Genre = %v, want [Sci-Fi]", show.Genre)
				}
			},
		},
		{
			name:            "episode without season subdirectory",
			showID:          9,
			episodeFilePath: "/tv/Flat Show/S01E01.mp4",
			nfoContent: `<?xml version="1.0"?>
<tvshow>
	<title>Flat Show</title>
	<year>2018</year>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:        9,
					LibraryID: 1,
					Title:     "Flat Show",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 9)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				if show.Year != 2018 {
					t.Errorf("Year = %v, want 2018", show.Year)
				}
			},
		},
		{
			name:            "preserve existing show metadata when NFO has zero values",
			showID:          10,
			episodeFilePath: "/tv/Preserve Show/Season 01/episode.mp4",
			nfoContent: `<?xml version="1.0"?>
<tvshow>
	<title>Preserve Show</title>
	<plot>New plot description</plot>
</tvshow>`,
			setupRepo: func(tvRepo *mocks.TVRepository) {
				tvRepo.WithShows(media.TVShow{
					ID:            10,
					LibraryID:     1,
					Title:         "Preserve Show",
					Year:          2010,
					Genre:         []string{"Drama", "Comedy"},
					IMDbID:        "tt1234567",
					TMDbID:        9999,
					ContentRating: "TV-14",
				})
			},
			checkRepo: func(t *testing.T, tvRepo *mocks.TVRepository) {
				show, err := tvRepo.GetTVShowByID(context.Background(), 10)
				if err != nil {
					t.Fatalf("Failed to get show: %v", err)
				}
				// Plot should be updated
				if !strings.Contains(show.Plot, "New plot") {
					t.Errorf("Plot should contain 'New plot', got: %v", show.Plot)
				}
				// Existing values should be preserved when NFO has zero values
				if show.Year != 2010 {
					t.Errorf("Year = %v, want 2010 (preserved)", show.Year)
				}
				if len(show.Genre) != 2 {
					t.Errorf("Genre count = %v, want 2 (preserved)", len(show.Genre))
				}
				if show.IMDbID != "tt1234567" {
					t.Errorf("IMDbID = %v, want tt1234567 (preserved)", show.IMDbID)
				}
				if show.TMDbID != 9999 {
					t.Errorf("TMDbID = %v, want 9999 (preserved)", show.TMDbID)
				}
				if show.ContentRating != "TV-14" {
					t.Errorf("ContentRating = %v, want TV-14 (preserved)", show.ContentRating)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvRepo := mocks.NewTVRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(tvRepo)
			}

			// Create temp directory structure if NFO content is provided
			var tempDir string
			if tt.nfoContent != "" && tt.episodeFilePath != "" {
				var err error
				tempDir, err = os.MkdirTemp("", "nfo-test-*")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tempDir)

				// Determine show directory structure
				episodeDir := filepath.Dir(tt.episodeFilePath)
				dirName := filepath.Base(episodeDir)
				var showDir string
				lowerDirName := strings.ToLower(dirName)
				if strings.HasPrefix(lowerDirName, "season") || (strings.HasPrefix(lowerDirName, "s") && len(dirName) <= 4) {
					// Episode is in a season subdir
					showDir = filepath.Join(tempDir, filepath.Base(filepath.Dir(episodeDir)))
					episodeDir = filepath.Join(showDir, dirName)
				} else {
					// Episode is directly in show directory
					showDir = filepath.Join(tempDir, dirName)
					episodeDir = showDir
				}

				// Create directory structure
				if err := os.MkdirAll(episodeDir, 0755); err != nil {
					t.Fatalf("Failed to create episode dir: %v", err)
				}

				// Create NFO file in show directory
				nfoPath := filepath.Join(showDir, "tvshow.nfo")
				if err := os.WriteFile(nfoPath, []byte(tt.nfoContent), 0644); err != nil {
					t.Fatalf("Failed to write NFO file: %v", err)
				}

				// Create episode file
				episodePath := filepath.Join(episodeDir, filepath.Base(tt.episodeFilePath))
				if err := os.WriteFile(episodePath, []byte("fake video"), 0644); err != nil {
					t.Fatalf("Failed to write episode file: %v", err)
				}

				// Update episode file path to use temp directory
				tt.episodeFilePath = episodePath
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					TV: tvRepo,
				},
				logger: discardLogger(),
			}

			// This function doesn't return an error, just logs warnings
			uc.enrichTVShowMetadataFromNFO(context.Background(), tt.showID, tt.episodeFilePath)

			if tt.checkRepo != nil {
				tt.checkRepo(t, tvRepo)
			}
		})
	}
}

// Additional tests for processMovie to improve coverage

func TestScanLibraryUseCase_processMovie_AdditionalCoverage(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.MovieRepository)
		setupCache    func(*sync.Map)
		expectMediaID bool
		expectError   bool
		checkError    func(*testing.T, error)
	}{
		{
			name:      "skip mp3 audio file",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/soundtrack.mp3",
				Title:    "Soundtrack",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/soundtrack.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "skip flac audio file",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/audio.flac",
				Title:    "Audio",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/audio.flac",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "skip m4a audio file",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/theme.m4a",
				Title:    "Theme",
				Duration: 60,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/theme.m4a",
				FileSize: 512,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "race condition - cache hit after unique constraint",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/race-cache-hit.mp4",
				Title:    "Race Movie",
				Year:     &year2020,
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/race-cache-hit.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Simulate unique constraint error on create
				movieRepo.WithCreateError(errors.New("UNIQUE constraint failed: media.file_path"))
				// Pre-populate media that will be in cache during race condition handling
				mediaRepo.WithMedia(&media.Media{
					ID:        150,
					LibraryID: 1,
					FilePath:  "/movies/race-cache-hit.mp4",
					Type:      "movie",
				})
				movieRepo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        150,
						LibraryID: 1,
						FilePath:  "/movies/race-cache-hit.mp4",
						Type:      "movie",
					},
				})
			},
			setupCache: func(cache *sync.Map) {
				// Pre-populate cache with the ID - simulates another worker adding it during race
				cache.Store("/movies/race-cache-hit.mp4", int64(150))
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "update failure after cache hit",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/update-fail.mp4",
				Title:    "Update Fail",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/update-fail.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        160,
					LibraryID: 1,
					FilePath:  "/movies/update-fail.mp4",
					Type:      "movie",
				})
				// Inject update error
				mediaRepo.UpdateErr = errors.New("database update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/movies/update-fail.mp4", int64(160))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
				}
			},
		},
		{
			name:      "movie update failure after cache hit",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/movie-update-fail.mp4",
				Title:    "Movie Update Fail",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/movie-update-fail.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        165,
					LibraryID: 1,
					FilePath:  "/movies/movie-update-fail.mp4",
					Type:      "movie",
				})
				movieRepo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        165,
						LibraryID: 1,
						FilePath:  "/movies/movie-update-fail.mp4",
						Type:      "movie",
					},
				})
				// Inject movie update error
				movieRepo.WithUpdateError(errors.New("movie update failed"))
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/movies/movie-update-fail.mp4", int64(165))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update movie metadata") {
					t.Errorf("Expected 'failed to update movie metadata' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - fetch failure after unique constraint",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/race-fetch-fail.mp4",
				Title:    "Race Fetch Fail",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/race-fetch-fail.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Simulate unique constraint error
				movieRepo.WithCreateError(errors.New("duplicate key value violates unique constraint"))
				// Inject fetch error - simulates failure to get existing media
				mediaRepo.GetByFilePathErr = errors.New("database fetch failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to fetch existing media after collision") {
					t.Errorf("Expected 'failed to fetch existing media after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - update failure after fetch",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/race-update-fail.mp4",
				Title:    "Race Update Fail",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/race-update-fail.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Simulate unique constraint error
				movieRepo.WithCreateError(errors.New("UNIQUE constraint failed"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        170,
					LibraryID: 1,
					FilePath:  "/movies/race-update-fail.mp4",
					Type:      "movie",
				})
				// Inject update error after collision handling
				mediaRepo.UpdateErr = errors.New("update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record after collision") {
					t.Errorf("Expected 'failed to update base media record after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - movie update failure after fetch",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/race-movie-update-fail.mp4",
				Title:    "Race Movie Update Fail",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/race-movie-update-fail.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Simulate unique constraint error
				movieRepo.WithCreateError(errors.New("duplicate key"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        175,
					LibraryID: 1,
					FilePath:  "/movies/race-movie-update-fail.mp4",
					Type:      "movie",
				})
				movieRepo.WithMovies(&media.Movie{
					Media: media.Media{
						ID:        175,
						LibraryID: 1,
						FilePath:  "/movies/race-movie-update-fail.mp4",
						Type:      "movie",
					},
				})
				// Inject movie update error after collision handling
				movieRepo.WithUpdateError(errors.New("movie update after collision failed"))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update movie metadata after collision") {
					t.Errorf("Expected 'failed to update movie metadata after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "non-unique constraint create error",
			libraryID: 1,
			result: &scanner.ScanResult{
				FilePath: "/movies/generic-error.mp4",
				Title:    "Generic Error",
				Duration: 7200,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/movies/generic-error.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Non-unique constraint error
				movieRepo.WithCreateError(errors.New("some other database error"))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to create base media record") {
					t.Errorf("Expected 'failed to create base media record' error, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeMovies,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					Movie:   movieRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := uc.processMovie(context.Background(), tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkError != nil && err != nil {
				tt.checkError(t, err)
			}
		})
	}
}

// Additional tests for processTVEpisode to improve coverage

func TestScanLibraryUseCase_processTVEpisode_AdditionalCoverage(t *testing.T) {
	season1 := 1
	episode1 := 1

	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.TVRepository)
		setupCache    func(*sync.Map)
		expectMediaID bool
		expectError   bool
		checkError    func(*testing.T, error)
	}{
		{
			name:      "skip mp3 audio file",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/theme.mp3",
				Title:    "Theme Song",
				Duration: 60,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/theme.mp3",
				FileSize: 512,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "skip flac audio file",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/audio.flac",
				Title:    "Audio",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/audio.flac",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "skip ogg audio file",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath: "/tv/Show/Season 01/soundtrack.ogg",
				Title:    "Soundtrack",
				Duration: 120,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/Season 01/soundtrack.ogg",
				FileSize: 768,
				FileHash: "hash",
			},
			setupRepo:     func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {},
			expectMediaID: false,
			expectError:   false,
		},
		{
			name:      "race condition - cache hit after unique constraint",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E01.mp4",
				Title:         "Episode",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E01.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Simulate unique constraint error
				tvRepo.WithCreateError(errors.New("UNIQUE constraint failed: tv_episodes"))
				// Pre-populate media
				mediaRepo.WithMedia(&media.Media{
					ID:        250,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E01.mp4",
					Type:      "tv_episode",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        250,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E01.mp4",
						Type:      "tv_episode",
					},
					ShowTitle: "Show",
					Season:    1,
					Episode:   1,
				})
			},
			setupCache: func(cache *sync.Map) {
				// Simulate cache hit during race condition
				cache.Store("/tv/Show/S01E01.mp4", int64(250))
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "update failure after cache hit",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E02.mp4",
				Title:         "Update Fail",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E02.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        260,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E02.mp4",
					Type:      "tv_episode",
				})
				// Inject update error
				mediaRepo.UpdateErr = errors.New("database update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/tv/Show/S01E02.mp4", int64(260))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
				}
			},
		},
		{
			name:      "tv episode update failure after cache hit",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E03.mp4",
				Title:         "Episode Update Fail",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E03.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        265,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E03.mp4",
					Type:      "tv_episode",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        265,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E03.mp4",
						Type:      "tv_episode",
					},
					Season:  1,
					Episode: 1,
				})
				// Inject TV episode update error
				tvRepo.UpdateErr = errors.New("episode update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/tv/Show/S01E03.mp4", int64(265))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update TV episode metadata") {
					t.Errorf("Expected 'failed to update TV episode metadata' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - fetch failure after unique constraint",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E04.mp4",
				Title:         "Race Fetch Fail",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E04.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Simulate unique constraint error
				tvRepo.WithCreateError(errors.New("duplicate key value violates unique constraint"))
				// Inject fetch error
				mediaRepo.GetByFilePathErr = errors.New("database fetch failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to fetch existing media after collision") {
					t.Errorf("Expected 'failed to fetch existing media after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - update failure after fetch",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E05.mp4",
				Title:         "Race Update Fail",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E05.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Simulate unique constraint error
				tvRepo.WithCreateError(errors.New("UNIQUE constraint failed"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        270,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E05.mp4",
					Type:      "tv_episode",
				})
				// Inject update error after collision
				mediaRepo.UpdateErr = errors.New("update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record after collision") {
					t.Errorf("Expected 'failed to update base media record after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - tv episode update failure after fetch",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E06.mp4",
				Title:         "Race Episode Update Fail",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E06.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Simulate unique constraint error
				tvRepo.WithCreateError(errors.New("duplicate key"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        275,
					LibraryID: 2,
					FilePath:  "/tv/Show/S01E06.mp4",
					Type:      "tv_episode",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        275,
						LibraryID: 2,
						FilePath:  "/tv/Show/S01E06.mp4",
						Type:      "tv_episode",
					},
					Season:  1,
					Episode: 1,
				})
				// Inject episode update error after collision
				tvRepo.UpdateErr = errors.New("episode update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update TV episode metadata after collision") {
					t.Errorf("Expected 'failed to update TV episode metadata after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "non-unique constraint create error",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Show/S01E07.mp4",
				Title:         "Generic Error",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/tv/Show/S01E07.mp4",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				// Non-unique constraint error
				tvRepo.WithCreateError(errors.New("some other database error"))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to create base media record") {
					t.Errorf("Expected 'failed to create base media record' error, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test TV Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeTV,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					TV:      tvRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := uc.processTVEpisode(context.Background(), tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkError != nil && err != nil {
				tt.checkError(t, err)
			}
		})
	}
}

// Additional tests for processMusicTrack to improve coverage

func TestScanLibraryUseCase_processMusicTrack_AdditionalCoverage(t *testing.T) {
	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.MusicRepository)
		setupCache    func(*sync.Map)
		expectMediaID bool
		expectError   bool
		checkError    func(*testing.T, error)
	}{
		{
			name:      "race condition - cache hit after unique constraint",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track.mp3",
				Title:    "Track",
				Artist:   "Artist",
				Album:    "Album",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/track.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed: media.file_path"))
				// Pre-populate media
				mediaRepo.WithMedia(&media.Media{
					ID:        350,
					LibraryID: 3,
					FilePath:  "/music/track.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        350,
						LibraryID: 3,
						FilePath:  "/music/track.mp3",
						Type:      "music_track",
					},
					Artist: "Artist",
					Album:  "Album",
				})
			},
			setupCache: func(cache *sync.Map) {
				// Simulate cache hit during race condition
				cache.Store("/music/track.mp3", int64(350))
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "update failure after cache hit",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/update-fail.mp3",
				Title:    "Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        360,
					LibraryID: 3,
					FilePath:  "/music/update-fail.mp3",
					Type:      "music_track",
				})
				// Inject update error
				mediaRepo.UpdateErr = errors.New("database update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/update-fail.mp3", int64(360))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
				}
			},
		},
		{
			name:      "music track update failure after cache hit",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track-update-fail.mp3",
				Title:    "Track Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/track-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        365,
					LibraryID: 3,
					FilePath:  "/music/track-update-fail.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        365,
						LibraryID: 3,
						FilePath:  "/music/track-update-fail.mp3",
						Type:      "music_track",
					},
				})
				// Inject music track update error
				musicRepo.UpdateErr = errors.New("track update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/track-update-fail.mp3", int64(365))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update music track metadata") {
					t.Errorf("Expected 'failed to update music track metadata' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - fetch failure after unique constraint",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-fetch-fail.mp3",
				Title:    "Race Fetch Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-fetch-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("duplicate key value violates unique constraint"))
				// Inject fetch error
				mediaRepo.GetByFilePathErr = errors.New("database fetch failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to fetch existing media after collision") {
					t.Errorf("Expected 'failed to fetch existing media after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - update failure after fetch",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-update-fail.mp3",
				Title:    "Race Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        370,
					LibraryID: 3,
					FilePath:  "/music/race-update-fail.mp3",
					Type:      "music_track",
				})
				// Inject update error after collision
				mediaRepo.UpdateErr = errors.New("update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record after collision") {
					t.Errorf("Expected 'failed to update base media record after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - music track update failure after fetch",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-track-update-fail.mp3",
				Title:    "Race Track Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-track-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("duplicate key"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        375,
					LibraryID: 3,
					FilePath:  "/music/race-track-update-fail.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        375,
						LibraryID: 3,
						FilePath:  "/music/race-track-update-fail.mp3",
						Type:      "music_track",
					},
				})
				// Inject track update error after collision
				musicRepo.UpdateErr = errors.New("track update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update music track metadata after collision") {
					t.Errorf("Expected 'failed to update music track metadata after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "non-unique constraint create error",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/generic-error.mp3",
				Title:    "Generic Error",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/generic-error.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Non-unique constraint error
				musicRepo.WithCreateError(errors.New("some other database error"))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to create base media record") {
					t.Errorf("Expected 'failed to create base media record' error, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t).WithLibraries(&domainLibrary.Library{
				ID:   tt.libraryID,
				Name: "Test Music Library",
				Path: "/test",
				Type: domainLibrary.LibraryTypeMusic,
			})

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
					Music:   musicRepo,
				},
				logger: discardLogger(),
			}

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := uc.processMusicTrack(context.Background(), tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkError != nil && err != nil {
				tt.checkError(t, err)
			}
		})
	}
}
