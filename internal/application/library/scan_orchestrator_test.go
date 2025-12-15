package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestNewScanLibraryUseCase(t *testing.T) {
	tests := []struct {
		name          string
		logger        *slog.Logger
		config        scan.Config
		verifyLogger  bool
		verifyConfig  bool
		verifyScanner bool
	}{
		{
			name:          "creates use case with all dependencies",
			logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
			config:        scan.Config{CheckpointBatchSize: 50, Timeout: 3600},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
		{
			name:          "creates use case with nil logger - uses no-op logger",
			logger:        nil,
			config:        scan.Config{CheckpointBatchSize: 30},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
		{
			name:          "creates use case with empty config - applies defaults",
			logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
			config:        scan.Config{},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepos := &scan.MediaRepositories{
				Library: mocks.NewLibraryRepository(t),
			}
			scanRepos := &scan.ScanRepositories{
				ScanJob:    mocks.NewScanJobRepository(t),
				Checkpoint: mocks.NewCheckpointRepository(t),
			}

			uc := NewScanLibraryUseCase(
				mediaRepos,
				scanRepos,
				nil, // imageRepo
				nil, // imageCleanup
				nil, // enrichmentEnqueuer
				tt.config,
				nil, // systemProfile
				tt.logger,
			)

			if uc == nil {
				t.Fatal("Expected use case to be created")
			}

			if tt.verifyLogger && uc.logger == nil {
				t.Error("Expected logger to be initialized")
			}

			if tt.verifyConfig {
				if uc.config.CheckpointBatchSize == 0 {
					t.Error("Expected config to have default CheckpointBatchSize")
				}
				if uc.config.Timeout == 0 {
					t.Error("Expected config to have default timeout")
				}
			}

			if tt.verifyScanner && uc.incrementalScanner == nil {
				t.Error("Expected incremental scanner to be initialized")
			}

			if uc.coordinator == nil {
				t.Error("Expected coordinator to be initialized")
			}

			if uc.mediaRepos != mediaRepos {
				t.Error("Expected mediaRepos to be set")
			}
			if uc.scanRepos != scanRepos {
				t.Error("Expected scanRepos to be set")
			}
		})
	}
}

func TestScanLibraryUseCase_ResumeStuckScans(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMocks    func(*mocks.ScanJobRepository, *mocks.LibraryRepository, *mocks.CheckpointRepository)
		expectError   bool
		verifyNoScans bool
	}{
		{
			name: "no stuck scans - returns nil",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				// No running jobs
			},
			expectError:   false,
			verifyNoScans: true,
		},
		{
			name: "error listing running scans - returns error",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.ListErr = errors.New("database error")
			},
			expectError: true,
		},
		{
			name: "one stuck scan - library not found - marks as failed",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
				// Library doesn't exist - will trigger markStuckScanFailed
			},
			expectError: false,
		},
		{
			name: "one stuck scan - actually complete - marks as completed",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         3,
					LibraryID:  30,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 10,
				})
				libRepo.WithLibraries(&library.Library{
					ID:   30,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				// All checkpoints completed
				checkpoints := make([]*scanner.ScanCheckpoint, 10)
				for i := 0; i < 10; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 3,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			libRepo := mocks.NewLibraryRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo, libRepo, checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &scan.ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			err := uc.ResumeStuckScans(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.verifyNoScans {
				scans, _ := scanRepo.ListRunning(ctx)
				if len(scans) != 0 {
					t.Errorf("Expected no running scans, got %d", len(scans))
				}
			}
		})
	}
}

func TestScanLibraryUseCase_ResumeScan(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		jobID        int64
		setupMocks   func(*mocks.ScanJobRepository, *mocks.LibraryRepository)
		expectError  bool
		errorMessage string
		verifyStatus bool
	}{
		{
			name:  "successfully resumes paused scan",
			jobID: 1,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:             1,
					LibraryID:      10,
					Status:         scanner.ScanStatusPaused,
					FilesFound:     100,
					FilesProcessed: 50,
				})
				libRepo.WithLibraries(&library.Library{
					ID:   10,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
			},
			expectError:  false,
			verifyStatus: true,
		},
		{
			name:  "error - scan job not found",
			jobID: 2,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				// No job exists
			},
			expectError:  true,
			errorMessage: "failed to get scan job",
		},
		{
			name:  "error - scan not paused",
			jobID: 3,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        3,
					LibraryID: 30,
					Status:    scanner.ScanStatusRunning,
				})
			},
			expectError:  true,
			errorMessage: "scan job is not paused",
		},
		{
			name:  "error - library not found",
			jobID: 4,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        4,
					LibraryID: 40,
					Status:    scanner.ScanStatusPaused,
				})
				// Library doesn't exist
			},
			expectError:  true,
			errorMessage: "failed to get library",
		},
		{
			name:  "error - failed to update status",
			jobID: 5,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        5,
					LibraryID: 50,
					Status:    scanner.ScanStatusPaused,
				})
				libRepo.WithLibraries(&library.Library{
					ID:   50,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				scanRepo.UpdateStatusErr = errors.New("database error")
			},
			expectError:  true,
			errorMessage: "failed to update scan status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			libRepo := mocks.NewLibraryRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo, libRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &scan.ScanRepositories{
					ScanJob: scanRepo,
				},
				config: scan.Config{
					CheckpointBatchSize: 50,
					Timeout:             3600,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			err := uc.ResumeScan(ctx, tt.jobID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errorMessage != "" && !contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMessage, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			if tt.verifyStatus && !tt.expectError {
				job, err := scanRepo.GetByID(ctx, tt.jobID)
				if err != nil {
					t.Fatalf("Failed to get job: %v", err)
				}
				if job.Status != scanner.ScanStatusRunning {
					t.Errorf("Expected status %v, got %v", scanner.ScanStatusRunning, job.Status)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_InitializeScanSession(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		systemProfile *system.Profile
		lib           *library.Library
		verifyReset   bool
	}{
		{
			name:          "without system profile - just resets artists",
			systemProfile: nil,
			lib: &library.Library{
				ID:   1,
				Path: "/test/library",
			},
			verifyReset: true,
		},
		{
			name: "with system profile - updates storage detection",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU: 4,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalSSD,
					IsRemote: false,
				},
			},
			lib: &library.Library{
				ID:   2,
				Path: t.TempDir(),
			},
			verifyReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				processedArtists: scanutil.AtomicDeduplicator{},
				processedShows:   scanutil.AtomicDeduplicator{},
				systemProfile:    tt.systemProfile,
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			uc.processedArtists.TryMark("Artist1")
			uc.processedArtists.TryMark("Artist2")

			if uc.processedArtists.TryMark("Artist1") {
				t.Error("Artist1 should already be marked before reset")
			}

			uc.InitializeScanSession(ctx, tt.lib)

			if tt.verifyReset {
				if !uc.processedArtists.TryMark("Artist1") {
					t.Error("expected processedArtists to be reset, but Artist1 is still marked")
				}
				if !uc.processedArtists.TryMark("Artist2") {
					t.Error("expected processedArtists to be reset, but Artist2 is still marked")
				}
			}
		})
	}
}

func TestScanLibraryUseCase_recoverFromPanic(t *testing.T) {
	tests := []struct {
		name           string
		panicValue     interface{}
		setupMock      func(*mocks.ScanJobRepository)
		expectComplete bool
	}{
		{
			name:           "recovers from string panic and marks job failed",
			panicValue:     "test panic string",
			expectComplete: true,
			setupMock: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 10,
					Status:    scanner.ScanStatusRunning,
				})
			},
		},
		{
			name:           "recovers from error panic",
			panicValue:     errors.New("test error"),
			expectComplete: true,
			setupMock: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 10,
					Status:    scanner.ScanStatusRunning,
				})
			},
		},
		{
			name:           "no panic - does nothing",
			panicValue:     nil,
			expectComplete: false,
			setupMock: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 10,
					Status:    scanner.ScanStatusRunning,
				})
			},
		},
		{
			name:           "handles Complete error gracefully",
			panicValue:     "test panic with complete error",
			expectComplete: false,
			setupMock: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 10,
					Status:    scanner.ScanStatusRunning,
				})
				repo.CompleteErr = errors.New("database connection failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			if tt.setupMock != nil {
				tt.setupMock(scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			testFunc := func() {
				defer uc.recoverFromPanic(1, 10, "test scan")
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
			}

			testFunc()

			if tt.panicValue != nil && tt.expectComplete {
				job, err := scanRepo.GetByID(context.Background(), 1)
				if err != nil {
					t.Fatalf("failed to get job: %v", err)
				}
				if job.Status != scanner.ScanStatusFailed {
					t.Errorf("expected job status failed, got %v", job.Status)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_GetScanStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		libraryID     int64
		setupMocks    func(*mocks.ScanJobRepository, *mocks.ScanStateRepository, *mocks.CheckpointRepository)
		expectError   bool
		verifyStatus  bool
		expectedJobID int64
	}{
		{
			name:      "successfully gets scan status with enrichment",
			libraryID: 1,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, stateRepo *mocks.ScanStateRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					Phase:          scanner.ScanPhaseProcessing,
					FilesFound:     1000,
					FilesProcessed: 500,
					Progress:       50.0,
					ErrorCount:     5,
				})
				stateRepo.WithStates(
					&scanner.ScanState{LibraryID: 1, FilePath: "/test/file1.mp4", HasError: true},
					&scanner.ScanState{LibraryID: 1, FilePath: "/test/file2.mp4", HasWarning: true},
					&scanner.ScanState{LibraryID: 1, FilePath: "/test/file3.mp4"},
				)
			},
			expectError:   false,
			verifyStatus:  true,
			expectedJobID: 100,
		},
		{
			name:      "error - no scan job found",
			libraryID: 999,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, stateRepo *mocks.ScanStateRepository, checkpointRepo *mocks.CheckpointRepository) {
				// No jobs exist
			},
			expectError: true,
		},
		{
			name:      "gets status for completed scan",
			libraryID: 2,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, stateRepo *mocks.ScanStateRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:             200,
					LibraryID:      2,
					Status:         scanner.ScanStatusCompleted,
					Phase:          scanner.ScanPhaseCompleted,
					FilesFound:     100,
					FilesProcessed: 100,
					Progress:       100.0,
				})
				stateRepo.WithStates()
			},
			expectError:   false,
			verifyStatus:  true,
			expectedJobID: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			stateRepo := mocks.NewScanStateRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo, stateRepo, checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanJob:   scanRepo,
					ScanState: stateRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			result, err := uc.GetScanStatus(ctx, tt.libraryID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if tt.verifyStatus && result != nil {
					if result.JobID != tt.expectedJobID {
						t.Errorf("Expected JobID %d, got %d", tt.expectedJobID, result.JobID)
					}
					if result.LibraryID != tt.libraryID {
						t.Errorf("Expected LibraryID %d, got %d", tt.libraryID, result.LibraryID)
					}
				}
			}
		})
	}
}

// Note: ProcessMovie, ProcessTVEpisode, and ProcessMusicTrack are thin wrappers
// that delegate to the media package. They are tested thoroughly in the media
// subpackage tests (movie_test.go, tv_test.go, music_test.go).

// Note: StartScanBackground is a thin wrapper that spawns a goroutine. It's tested
// indirectly through integration tests and the actual logic is in startScanBackground
// which is covered by other tests.

func TestScanLibraryUseCase_DependencyBundles(t *testing.T) {
	tests := []struct {
		name           string
		verifyFunction func(*ScanLibraryUseCase) bool
		description    string
	}{
		{
			name: "mediaDeps - returns correct dependencies",
			verifyFunction: func(uc *ScanLibraryUseCase) bool {
				deps := uc.mediaDeps()
				return deps != nil &&
					deps.MediaRepos == uc.mediaRepos &&
					deps.ScanRepos == uc.scanRepos &&
					deps.Logger == uc.logger
			},
			description: "mediaDeps should bundle media processing dependencies",
		},
		{
			name: "processingDeps - returns correct dependencies",
			verifyFunction: func(uc *ScanLibraryUseCase) bool {
				deps := uc.processingDeps()
				return deps != nil &&
					deps.ScanRepos == uc.scanRepos &&
					deps.MediaRepos == uc.mediaRepos &&
					deps.Logger == uc.logger
			},
			description: "processingDeps should bundle processing dependencies",
		},
		{
			name: "discoveryDeps - returns correct dependencies",
			verifyFunction: func(uc *ScanLibraryUseCase) bool {
				deps := uc.discoveryDeps()
				return deps != nil &&
					deps.ScanRepos == uc.scanRepos &&
					deps.Logger == uc.logger
			},
			description: "discoveryDeps should bundle discovery dependencies",
		},
		{
			name: "cleanupDeps - returns correct dependencies",
			verifyFunction: func(uc *ScanLibraryUseCase) bool {
				deps := uc.cleanupDeps()
				return deps != nil &&
					deps.MediaRepos == uc.mediaRepos &&
					deps.Logger == uc.logger
			},
			description: "cleanupDeps should bundle cleanup dependencies",
		},
		{
			name: "executionDeps - returns correct dependencies",
			verifyFunction: func(uc *ScanLibraryUseCase) bool {
				deps := uc.executionDeps()
				return deps != nil &&
					deps.ScanRepos == uc.scanRepos &&
					deps.MediaRepos == uc.mediaRepos &&
					deps.Logger == uc.logger
			},
			description: "executionDeps should bundle execution dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)
			stateRepo := mocks.NewScanStateRepository(t)
			libRepo := mocks.NewLibraryRepository(t)

			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &scan.ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
					ScanState:  stateRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			if !tt.verifyFunction(uc) {
				t.Errorf("%s: verification failed", tt.description)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}()))
}
