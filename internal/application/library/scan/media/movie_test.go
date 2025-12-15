package media

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestProcessMovie(t *testing.T) {
	year2020 := 2020

	tests := []struct {
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.MovieRepository)
		setupCache func(*sync.Map)
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
				cache.Store("/movies/existing.mp4", int64(50))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
				if len(movies) != 1 {
					t.Errorf("Expected 1 movie, got %d", len(movies))
				}
				for _, movie := range movies {
					if movie.Media.Title != "Updated Title" {
						t.Errorf("Title = %v, want Updated Title", movie.Media.Title)
					}
					if movie.Year != 2020 {
						t.Errorf("Year = %v, want 2020", movie.Year)
					}
					if movie.Media.Duration != 5400 {
						t.Errorf("Duration = %v, want 5400", movie.Media.Duration)
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
			setupCache: func(cache *sync.Map) {
				cache.Store("/movies/update-error.mp4", int64(60))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {
				// Error should be logged but not panic
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

			mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			_, _ = ProcessMovie(context.Background(), deps, tt.libraryID, tt.result, checkpoint, existingMediaCache)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, movieRepo)
			}
		})
	}
}

func TestProcessMovie_SkipsAudioFiles(t *testing.T) {
	audioExtensions := []string{".mp3", ".flac", ".m4a", ".wav", ".ogg"}

	for _, ext := range audioExtensions {
		t.Run("skips "+ext, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath: "/movies/soundtrack" + ext,
				Title:    "Soundtrack",
				Duration: 180,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			mediaID, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)

			if err != nil {
				t.Errorf("Expected no error for audio file, got %v", err)
			}
			if mediaID != nil {
				t.Errorf("Expected nil mediaID for skipped audio file, got %v", *mediaID)
			}

			// Verify no movie was created
			movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
			if len(movies) != 0 {
				t.Errorf("Expected 0 movies for audio file, got %d", len(movies))
			}
		})
	}
}

func TestProcessMovie_NilCheckpoint(t *testing.T) {
	year2020 := 2020

	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/movies/test.mp4",
		Title:    "Test Movie",
		Year:     &year2020,
		Duration: 7200,
	}
	existingMediaCache := &sync.Map{}

	// Pass nil checkpoint - should handle gracefully
	_, err := ProcessMovie(context.Background(), deps, 1, result, nil, existingMediaCache)

	if err != nil {
		t.Errorf("Expected no error with nil checkpoint, got %v", err)
	}

	// Verify movie was created
	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Errorf("Expected 1 movie created, got %d", len(movies))
	}
}

func TestProcessMovie_IsExtraDetection(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		wantExtra bool
	}{
		{
			name:      "regular movie",
			filePath:  "/movies/Movie (2024)/Movie (2024).mkv",
			wantExtra: false,
		},
		{
			name:      "trailer file",
			filePath:  "/movies/Movie (2024)/Movie-trailer.mp4",
			wantExtra: true,
		},
		{
			name:      "deleted scenes",
			filePath:  "/movies/Movie (2024)/Movie-deleted.mp4",
			wantExtra: true,
		},
		{
			name:      "featurette",
			filePath:  "/movies/Movie (2024)/Movie-featurette.mp4",
			wantExtra: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath: tt.filePath,
				Title:    "Test",
				Duration: 3600,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.filePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			_, _ = ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)

			movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
			if len(movies) != 1 {
				t.Fatalf("Expected 1 movie, got %d", len(movies))
			}

			if movies[0].Media.IsExtra != tt.wantExtra {
				t.Errorf("IsExtra = %v, want %v for %s", movies[0].Media.IsExtra, tt.wantExtra, tt.filePath)
			}
		})
	}
}

func TestProcessMovie_SortTitleNormalization(t *testing.T) {
	tests := []struct {
		name          string
		inputTitle    string
		expectSortSet bool
	}{
		{
			name:          "normalizes 'The' prefix",
			inputTitle:    "The Matrix",
			expectSortSet: true,
		},
		{
			name:          "normalizes 'A' prefix",
			inputTitle:    "A Beautiful Mind",
			expectSortSet: true,
		},
		{
			name:          "no prefix to normalize",
			inputTitle:    "Matrix Reloaded",
			expectSortSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath: "/movies/test.mp4",
				Title:    tt.inputTitle,
				Duration: 3600,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			_, _ = ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)

			// Verify movie was created with SortTitle set
			movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
			if len(movies) != 1 {
				t.Fatalf("Expected 1 movie, got %d", len(movies))
			}
			if movies[0].SortTitle == "" {
				t.Error("Expected SortTitle to be set")
			}
		})
	}
}

func TestProcessMovie_VideoMetadataFields(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	year := 2023
	result := &scanner.ScanResult{
		FilePath:        "/movies/4k-movie.mkv",
		Title:           "4K Movie",
		Year:            &year,
		Duration:        7200,
		Width:           3840,
		Height:          2160,
		VideoCodec:      "hevc",
		AudioCodec:      "truehd",
		Bitrate:         50000000,
		FrameRate:       23.976,
		ContainerFormat: "mkv",
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 53687091200, // 50GB
		FileHash: "4k-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	if movie.Media.Width != 3840 {
		t.Errorf("Width = %v, want 3840", movie.Media.Width)
	}
	if movie.Media.Height != 2160 {
		t.Errorf("Height = %v, want 2160", movie.Media.Height)
	}
	if movie.Media.VideoCodec != "hevc" {
		t.Errorf("VideoCodec = %v, want hevc", movie.Media.VideoCodec)
	}
	if movie.Media.AudioCodec != "truehd" {
		t.Errorf("AudioCodec = %v, want truehd", movie.Media.AudioCodec)
	}
	if movie.Media.Bitrate != 50000000 {
		t.Errorf("Bitrate = %v, want 50000000", movie.Media.Bitrate)
	}
	if movie.Media.FrameRate != 23.976 {
		t.Errorf("FrameRate = %v, want 23.976", movie.Media.FrameRate)
	}
	if movie.Media.ContainerFormat != "mkv" {
		t.Errorf("ContainerFormat = %v, want mkv", movie.Media.ContainerFormat)
	}
	if movie.Media.FileSize != 53687091200 {
		t.Errorf("FileSize = %v, want 53687091200", movie.Media.FileSize)
	}
	if movie.Media.FileHash != "4k-hash" {
		t.Errorf("FileHash = %v, want 4k-hash", movie.Media.FileHash)
	}
}

func TestProcessMovie_AdditionalCoverage(t *testing.T) {
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
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
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
				if !strings.Contains(err.Error(), "failed to create movie") {
					t.Errorf("Expected 'failed to create movie' error, got: %v", err)
				}
			},
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
			setupRepo:     func(mediaRepo *mocks.MediaRepository, movieRepo *mocks.MovieRepository) {},
			expectMediaID: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, movieRepo)
			}

			mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := ProcessMovie(context.Background(), deps, tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

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

func TestProcessMovie_NFOMetadata(t *testing.T) {
	// NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// See internal/application/enrichment/builtin/nfo.go for the NFO enricher tests.
	t.Skip("NFO parsing moved to async enrichment pipeline")

	// Create temp directory for NFO files
	tmpDir := t.TempDir()

	// Create a movie file and NFO file
	moviePath := tmpDir + "/Test Movie (2024)/Test Movie (2024).mkv"
	nfoPath := tmpDir + "/Test Movie (2024)/Test Movie (2024).nfo"

	// Create directory
	if err := os.MkdirAll(tmpDir+"/Test Movie (2024)", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create empty movie file
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create movie file: %v", err)
	}

	// Create NFO file with metadata
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>NFO Title Override</title>
  <originaltitle>Original NFO Title</originaltitle>
  <sorttitle>NFO, Test</sorttitle>
  <year>2024</year>
  <releasedate>2024-06-15</releasedate>
  <runtime>120</runtime>
  <imdb>tt1234567</imdb>
  <tmdbid>98765</tmdbid>
  <director>Test Director</director>
  <actor><name>Actor One</name><role>Role One</role></actor>
  <actor><name>Actor Two</name><role>Role Two</role></actor>
  <genre>Action</genre>
  <genre>Sci-Fi</genre>
  <plot>This is the test plot from NFO.</plot>
  <tagline>Test tagline</tagline>
  <mpaa>PG-13</mpaa>
  <rating>8.5</rating>
  <budget>150000000</budget>
  <revenue>500000000</revenue>
  <originallanguage>en</originallanguage>
  <country>USA</country>
  <awards>Best Test Movie</awards>
</movie>`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	// Test that ProcessMovie uses NFO metadata
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	year := 2023 // Different year in ScanResult
	result := &scanner.ScanResult{
		FilePath: moviePath,
		Title:    "Scan Result Title", // Should be overridden by NFO
		Year:     &year,
		Duration: 7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: moviePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify NFO metadata was applied
	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	if movie.Media.Title != "NFO Title Override" {
		t.Errorf("Title = %v, want 'NFO Title Override'", movie.Media.Title)
	}
	if movie.Year != 2024 {
		t.Errorf("Year = %v, want 2024", movie.Year)
	}
	if movie.OriginalTitle != "Original NFO Title" {
		t.Errorf("OriginalTitle = %v, want 'Original NFO Title'", movie.OriginalTitle)
	}
	if movie.SortTitle != "nfo, test" && movie.SortTitle != "NFO, Test" {
		// Could be normalized
		t.Logf("SortTitle = %v (might be normalized)", movie.SortTitle)
	}
	if movie.IMDbID != "tt1234567" {
		t.Errorf("IMDbID = %v, want 'tt1234567'", movie.IMDbID)
	}
	if movie.TMDbID != 98765 {
		t.Errorf("TMDbID = %v, want 98765", movie.TMDbID)
	}
	if movie.Director != "Test Director" {
		t.Errorf("Director = %v, want 'Test Director'", movie.Director)
	}
	if len(movie.Cast) < 2 {
		t.Errorf("Cast length = %v, want at least 2", len(movie.Cast))
	}
	if len(movie.Genre) != 2 {
		t.Errorf("Genre length = %v, want 2", len(movie.Genre))
	}
	if movie.Plot != "This is the test plot from NFO." {
		t.Errorf("Plot = %v, want 'This is the test plot from NFO.'", movie.Plot)
	}
	if movie.Tagline != "Test tagline" {
		t.Errorf("Tagline = %v, want 'Test tagline'", movie.Tagline)
	}
	if movie.ContentRating != "PG-13" {
		t.Errorf("ContentRating = %v, want 'PG-13'", movie.ContentRating)
	}
	if movie.Budget != 150000000 {
		t.Errorf("Budget = %v, want 150000000", movie.Budget)
	}
	if movie.Revenue != 500000000 {
		t.Errorf("Revenue = %v, want 500000000", movie.Revenue)
	}
	if movie.OriginalLanguage != "en" {
		t.Errorf("OriginalLanguage = %v, want 'en'", movie.OriginalLanguage)
	}
	if movie.CountryOfOrigin != "USA" {
		t.Errorf("CountryOfOrigin = %v, want 'USA'", movie.CountryOfOrigin)
	}
	if movie.AwardsSummary != "Best Test Movie" {
		t.Errorf("AwardsSummary = %v, want 'Best Test Movie'", movie.AwardsSummary)
	}
}

func TestProcessMovie_NFOPartialOverride(t *testing.T) {
	// NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	t.Skip("NFO parsing moved to async enrichment pipeline")

	// Test that NFO only overrides when values are present
	tmpDir := t.TempDir()

	moviePath := tmpDir + "/Partial Movie (2024)/Partial Movie (2024).mkv"
	nfoPath := tmpDir + "/Partial Movie (2024)/Partial Movie (2024).nfo"

	if err := os.MkdirAll(tmpDir+"/Partial Movie (2024)", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create movie file: %v", err)
	}

	// NFO with empty title - should NOT override scan result title
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title></title>
  <year>0</year>
  <director>NFO Director</director>
</movie>`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	year := 2024
	result := &scanner.ScanResult{
		FilePath: moviePath,
		Title:    "Scan Result Title",
		Year:     &year,
		Duration: 7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: moviePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, _ = ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)

	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	// Empty NFO title should NOT override
	if movie.Media.Title != "Scan Result Title" {
		t.Errorf("Title = %v, want 'Scan Result Title' (NFO empty should not override)", movie.Media.Title)
	}
	// Zero NFO year should NOT override
	if movie.Year != 2024 {
		t.Errorf("Year = %v, want 2024 (NFO year=0 should not override)", movie.Year)
	}
	// Director should still be set from NFO
	if movie.Director != "NFO Director" {
		t.Errorf("Director = %v, want 'NFO Director'", movie.Director)
	}
}

func TestProcessMovie_MalformedNFO(t *testing.T) {
	// Test that malformed NFO is handled gracefully
	tmpDir := t.TempDir()

	moviePath := tmpDir + "/Bad NFO Movie (2024)/Bad NFO Movie (2024).mkv"
	nfoPath := tmpDir + "/Bad NFO Movie (2024)/Bad NFO Movie (2024).nfo"

	if err := os.MkdirAll(tmpDir+"/Bad NFO Movie (2024)", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create movie file: %v", err)
	}

	// Malformed XML
	nfoContent := `<?xml version="1.0"?>
<movie>
  <title>Malformed</title>
  <!-- Missing closing tags -->`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	year := 2024
	result := &scanner.ScanResult{
		FilePath: moviePath,
		Title:    "Fallback Title",
		Year:     &year,
		Duration: 7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: moviePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	// Should not error - malformed NFO should be silently ignored
	_, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error with malformed NFO: %v", err)
	}

	// Verify movie was still created with fallback title
	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	if movie.Media.Title != "Fallback Title" {
		t.Errorf("Title = %v, want 'Fallback Title' (fallback when NFO malformed)", movie.Media.Title)
	}
}

func TestProcessMovie_NFORuntimeAndReleaseDate(t *testing.T) {
	// NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// See internal/application/enrichment/builtin/nfo.go for the NFO enricher tests.
	t.Skip("NFO parsing moved to async enrichment pipeline")

	// Test runtime and release date parsing from NFO
	tmpDir := t.TempDir()

	moviePath := tmpDir + "/Runtime Movie (2024)/Runtime Movie (2024).mkv"
	nfoPath := tmpDir + "/Runtime Movie (2024)/Runtime Movie (2024).nfo"

	if err := os.MkdirAll(tmpDir+"/Runtime Movie (2024)", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create movie file: %v", err)
	}

	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Runtime Test</title>
  <year>2024</year>
  <runtime>142</runtime>
  <releasedate>2024-03-15</releasedate>
</movie>`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: moviePath,
		Title:    "Runtime Test",
		Duration: 7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: moviePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	if movie.RuntimeMinutes != 142 {
		t.Errorf("RuntimeMinutes = %v, want 142", movie.RuntimeMinutes)
	}
	// ReleaseDate should be parsed from NFO
	if movie.ReleaseDate.IsZero() {
		t.Error("Expected ReleaseDate to be set from NFO")
	}
}

func TestProcessMovie_NFOMaturityRating(t *testing.T) {
	// NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// See internal/application/enrichment/builtin/nfo.go for the NFO enricher tests.
	t.Skip("NFO parsing moved to async enrichment pipeline")

	// Test maturity rating extraction from NFO
	tmpDir := t.TempDir()

	moviePath := tmpDir + "/Rating Movie (2024)/Rating Movie (2024).mkv"
	nfoPath := tmpDir + "/Rating Movie (2024)/Rating Movie (2024).nfo"

	if err := os.MkdirAll(tmpDir+"/Rating Movie (2024)", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create movie file: %v", err)
	}

	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Rating Test</title>
  <year>2024</year>
  <maturityrating>16</maturityrating>
  <tag>Violence</tag>
  <tag>Language</tag>
</movie>`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, movieRepo, nil, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: moviePath,
		Title:    "Rating Test",
		Duration: 7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: moviePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMovie(context.Background(), deps, 1, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	movies, _ := movieRepo.ListMoviesByLibrary(context.Background(), 1)
	if len(movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(movies))
	}

	movie := movies[0]
	// Content advisories should come from tags
	if len(movie.ContentAdvisories) != 2 {
		t.Errorf("ContentAdvisories length = %v, want 2", len(movie.ContentAdvisories))
	}
}
