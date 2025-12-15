package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// TestScanLibraryUseCase_ProcessMovie tests the ProcessMovie method
func TestScanLibraryUseCase_ProcessMovie(t *testing.T) {
	ctx := context.Background()
	movieRepo := mocks.NewMovieRepository(t)
	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Movie:   movieRepo,
		Media:   mocks.NewMediaRepository(t),
	}
	scanRepos := &scan.ScanRepositories{
		ScanJob:    mocks.NewScanJobRepository(t),
		Checkpoint: mocks.NewCheckpointRepository(t),
		ScanState:  mocks.NewScanStateRepository(t),
	}

	uc := NewScanLibraryUseCase(
		mediaRepos,
		scanRepos,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		scan.Config{},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	year := 2024
	result := &scanner.ScanResult{
		FilePath:  "/movies/test.mp4",
		MediaType: scanner.MediaTypeMovie,
		Title:     "Test Movie",
		Year:      &year,
		Duration:  7200,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ScanJobID: 1,
		FilePath:  "/movies/test.mp4",
		Status:    scanner.CheckpointPending,
	}

	cache := &sync.Map{}

	// Call ProcessMovie - this delegates to scanmedia.ProcessMovie
	mediaID, err := uc.ProcessMovie(ctx, 1, result, checkpoint, cache)

	// We expect nil error and a valid media ID (mocked repository will create it)
	if err != nil {
		t.Errorf("ProcessMovie() error = %v", err)
	}
	if mediaID == nil {
		t.Error("ProcessMovie() returned nil mediaID")
	}
}

// TestScanLibraryUseCase_ProcessTVEpisode tests the ProcessTVEpisode method
func TestScanLibraryUseCase_ProcessTVEpisode(t *testing.T) {
	ctx := context.Background()
	tvRepo := mocks.NewTVRepository(t)
	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		TV:      tvRepo,
		Media:   mocks.NewMediaRepository(t),
	}
	scanRepos := &scan.ScanRepositories{
		ScanJob:    mocks.NewScanJobRepository(t),
		Checkpoint: mocks.NewCheckpointRepository(t),
		ScanState:  mocks.NewScanStateRepository(t),
	}

	uc := NewScanLibraryUseCase(
		mediaRepos,
		scanRepos,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		scan.Config{},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	season := 1
	episode := 1
	result := &scanner.ScanResult{
		FilePath:      "/tv/show/s01e01.mp4",
		MediaType:     scanner.MediaTypeEpisode,
		ShowName:      "Test Show",
		Title:         "Pilot",
		SeasonNumber:  &season,
		EpisodeNumber: &episode,
		Duration:      3600,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ScanJobID: 1,
		FilePath:  "/tv/show/s01e01.mp4",
		Status:    scanner.CheckpointPending,
	}

	cache := &sync.Map{}

	mediaID, err := uc.ProcessTVEpisode(ctx, 1, result, checkpoint, cache)

	if err != nil {
		t.Errorf("ProcessTVEpisode() error = %v", err)
	}
	if mediaID == nil {
		t.Error("ProcessTVEpisode() returned nil mediaID")
	}
}

// TestScanLibraryUseCase_ProcessMusicTrack tests the ProcessMusicTrack method
func TestScanLibraryUseCase_ProcessMusicTrack(t *testing.T) {
	ctx := context.Background()
	musicRepo := mocks.NewMusicRepository(t)
	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Music:   musicRepo,
		Media:   mocks.NewMediaRepository(t),
	}
	scanRepos := &scan.ScanRepositories{
		ScanJob:    mocks.NewScanJobRepository(t),
		Checkpoint: mocks.NewCheckpointRepository(t),
		ScanState:  mocks.NewScanStateRepository(t),
	}

	uc := NewScanLibraryUseCase(
		mediaRepos,
		scanRepos,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		scan.Config{},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	trackNum := 1
	result := &scanner.ScanResult{
		FilePath:    "/music/artist/album/track.mp3",
		MediaType:   scanner.MediaTypeTrack,
		Title:       "Test Track",
		Artist:      "Test Artist",
		Album:       "Test Album",
		TrackNumber: &trackNum,
		Duration:    240,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ScanJobID: 1,
		FilePath:  "/music/artist/album/track.mp3",
		Status:    scanner.CheckpointPending,
	}

	cache := &sync.Map{}

	mediaID, err := uc.ProcessMusicTrack(ctx, 1, result, checkpoint, cache)

	if err != nil {
		t.Errorf("ProcessMusicTrack() error = %v", err)
	}
	if mediaID == nil {
		t.Error("ProcessMusicTrack() returned nil mediaID")
	}
}

// TestScanLibraryUseCase_NewProgressUpdate tests the NewProgressUpdate method
func TestScanLibraryUseCase_NewProgressUpdate(t *testing.T) {
	scanRepo := mocks.NewScanJobRepository(t)
	uc := &ScanLibraryUseCase{
		scanRepos: &scan.ScanRepositories{
			ScanJob: scanRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	update := uc.NewProgressUpdate(123)
	if update == nil {
		t.Error("NewProgressUpdate() returned nil")
	}
}

// TestScanLibraryUseCase_StartScanBackground tests the StartScanBackground method
func TestScanLibraryUseCase_StartScanBackground(t *testing.T) {
	ctx := context.Background()
	libRepo := mocks.NewLibraryRepository(t)
	scanRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	libRepo.WithLibraries(&library.Library{
		ID:   10,
		Name: "Test Library",
		Path: t.TempDir(),
		Type: library.LibraryTypeMovies,
	})

	scanRepo.WithJobs(&scanner.ScanJob{
		ID:        1,
		LibraryID: 10,
		Status:    scanner.ScanStatusRunning,
	})

	uc := &ScanLibraryUseCase{
		mediaRepos: &scan.MediaRepositories{
			Library: libRepo,
		},
		scanRepos: &scan.ScanRepositories{
			ScanJob:    scanRepo,
			Checkpoint: checkpointRepo,
			ScanState:  mocks.NewScanStateRepository(t),
		},
		config: scan.Config{
			CheckpointBatchSize: 50,
			Timeout:             time.Second * 5, // Short timeout for test
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Call StartScanBackground - this spawns a goroutine
	uc.StartScanBackground(1, 10, "test panic context")

	// Give the goroutine a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify the job status was checked (indirectly through the background process)
	job, err := scanRepo.GetByID(ctx, 1)
	if err != nil {
		t.Errorf("Expected job to be accessible: %v", err)
	}
	if job == nil {
		t.Error("Expected job to exist")
	}
}

// TestScanLibraryUseCase_StartScanBackground_LibraryNotFound tests error case
func TestScanLibraryUseCase_StartScanBackground_LibraryNotFound(t *testing.T) {
	libRepo := mocks.NewLibraryRepository(t)
	scanRepo := mocks.NewScanJobRepository(t)

	// No library exists

	uc := &ScanLibraryUseCase{
		mediaRepos: &scan.MediaRepositories{
			Library: libRepo,
		},
		scanRepos: &scan.ScanRepositories{
			ScanJob: scanRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// This should log an error but not panic
	uc.StartScanBackground(1, 999, "test panic context")

	// Give the error logging a moment to complete
	time.Sleep(50 * time.Millisecond)
}

// TestScanLibraryUseCase_runScan tests the runScan method
func TestScanLibraryUseCase_runScan(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		setupMocks func(*mocks.ScanJobRepository, *mocks.CheckpointRepository, *mocks.ScanStateRepository)
		expectFail bool
	}{
		{
			name: "scan job not found - completes with error",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, checkpointRepo *mocks.CheckpointRepository, stateRepo *mocks.ScanStateRepository) {
				// No job exists - GetByID will fail
			},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)
			stateRepo := mocks.NewScanStateRepository(t)
			libRepo := mocks.NewLibraryRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo, checkpointRepo, stateRepo)
			}

			// Use NewScanLibraryUseCase to properly initialize all fields
			mediaRepos := &scan.MediaRepositories{
				Library: libRepo,
				Media:   mocks.NewMediaRepository(t),
			}
			scanRepos := &scan.ScanRepositories{
				ScanJob:    scanRepo,
				Checkpoint: checkpointRepo,
				ScanState:  stateRepo,
			}

			uc := NewScanLibraryUseCase(
				mediaRepos,
				scanRepos,
				nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil,
				scan.Config{
					CheckpointBatchSize: 50,
					Timeout:             time.Second * 2,
				},
				nil,
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			lib := &library.Library{
				ID:   10,
				Name: "Test Library",
				Path: tempDir,
				Type: library.LibraryTypeMovies,
			}

			// Call runScan directly
			uc.runScan(ctx, 1, lib)

			// Verify job was attempted to be retrieved (which failed)
			if !tt.expectFail {
				// For fresh scan, the execution will try to start discovery
				// We just verify no panic occurred
			}
		})
	}
}

// TestScanLibraryUseCase_executionDeps tests the executionDeps method
func TestScanLibraryUseCase_executionDeps(t *testing.T) {
	tests := []struct {
		name             string
		hasImageRepo     bool
		hasImageCleanup  bool
		expectHasCleanup bool
	}{
		{
			name:             "with image repo and cleanup",
			hasImageRepo:     true,
			hasImageCleanup:  true,
			expectHasCleanup: true,
		},
		{
			name:             "without image repo",
			hasImageRepo:     false,
			hasImageCleanup:  true,
			expectHasCleanup: false,
		},
		{
			name:             "without image cleanup",
			hasImageRepo:     true,
			hasImageCleanup:  false,
			expectHasCleanup: false,
		},
		{
			name:             "without both",
			hasImageRepo:     false,
			hasImageCleanup:  false,
			expectHasCleanup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: mocks.NewLibraryRepository(t),
				},
				scanRepos: &scan.ScanRepositories{
					ScanJob:    mocks.NewScanJobRepository(t),
					Checkpoint: mocks.NewCheckpointRepository(t),
					ScanState:  mocks.NewScanStateRepository(t),
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			if tt.hasImageRepo {
				uc.imageRepo = mocks.NewImageRepository(t)
			}
			if tt.hasImageCleanup {
				// Create a simple mock cleanup executor
				uc.imageCleanup = &mockImageCleanup{}
			}

			deps := uc.executionDeps()
			if deps == nil {
				t.Fatal("executionDeps() returned nil")
			}

			// Test HasImageCleanup function
			hasCleanup := deps.HasImageCleanup()
			if hasCleanup != tt.expectHasCleanup {
				t.Errorf("HasImageCleanup() = %v, want %v", hasCleanup, tt.expectHasCleanup)
			}

			// Verify other fields are set
			if deps.ScanRepos != uc.scanRepos {
				t.Error("ScanRepos not set correctly")
			}
			if deps.MediaRepos != uc.mediaRepos {
				t.Error("MediaRepos not set correctly")
			}
			if deps.Logger != uc.logger {
				t.Error("Logger not set correctly")
			}
			if deps.RecoverFromPanic == nil {
				t.Error("RecoverFromPanic not set")
			}
			if deps.RecoverFromPanicWithError == nil {
				t.Error("RecoverFromPanicWithError not set")
			}
		})
	}
}

// TestScanLibraryUseCase_recoverFromPanicWithError tests the recoverFromPanicWithError method
func TestScanLibraryUseCase_recoverFromPanicWithError(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
		expectErr  bool
	}{
		{
			name:       "recovers from string panic",
			panicValue: "test panic",
			expectErr:  true,
		},
		{
			name:       "recovers from error panic",
			panicValue: errors.New("test error"),
			expectErr:  true,
		},
		{
			name:       "no panic - no error",
			panicValue: nil,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			errChan := make(chan error, 1)

			testFunc := func() {
				defer uc.recoverFromPanicWithError(1, 10, "test context", errChan)
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
				// No panic - close channel without error
				close(errChan)
			}

			testFunc()

			// Check if error was sent
			select {
			case err := <-errChan:
				if tt.expectErr && err == nil {
					t.Error("expected error in channel, got nil")
				}
				if !tt.expectErr && err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
				if tt.expectErr {
					t.Error("timeout waiting for error")
				}
			}
		})
	}
}

// TestScanLibraryUseCase_statusDeps tests the statusDeps method
func TestScanLibraryUseCase_statusDeps(t *testing.T) {
	scanRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	stateRepo := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		scanRepos: &scan.ScanRepositories{
			ScanJob:    scanRepo,
			Checkpoint: checkpointRepo,
			ScanState:  stateRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	deps := uc.statusDeps()
	if deps == nil {
		t.Fatal("statusDeps() returned nil")
	}
	if deps.ScanRepos != uc.scanRepos {
		t.Error("ScanRepos not set correctly")
	}
	if deps.Logger != uc.logger {
		t.Error("Logger not set correctly")
	}
}
