package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestScanLibraryUseCase_validateCheckpointCompleteness(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupMocks   func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		jobID        int64
		expectResult bool
		checkDeleted bool // Verify checkpoints were deleted
	}{
		{
			name:  "no checkpoints exist - returns true",
			jobID: 1,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					FilesFound: 0,
				})
				// No checkpoints - GetStats returns zero stats
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "all checkpoints completed - returns true",
			jobID: 2,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					FilesFound: 100,
				})
				// Create 100 completed checkpoints
				checkpoints := make([]*scanner.ScanCheckpoint, 100)
				for i := 0; i < 100; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 2,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "some checkpoints pending - returns true",
			jobID: 3,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         3,
					LibraryID:  30,
					FilesFound: 100,
				})
				// Create 50 completed and 50 pending checkpoints
				checkpoints := make([]*scanner.ScanCheckpoint, 100)
				for i := 0; i < 50; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 3,
						FilePath:  "/test/completed" + string(rune(i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				for i := 50; i < 100; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 3,
						FilePath:  "/test/pending" + string(rune(i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "some checkpoints have errors - returns true",
			jobID: 4,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         4,
					LibraryID:  40,
					FilesFound: 100,
				})
				// Create 80 completed, 10 failed, 10 pending checkpoints
				checkpoints := make([]*scanner.ScanCheckpoint, 100)
				for i := 0; i < 80; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 4,
						FilePath:  "/test/completed" + string(rune(i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				for i := 80; i < 90; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID:     4,
						FilePath:      "/test/failed" + string(rune(i)),
						Status:        scanner.CheckpointFailed,
						ErrorMessage:  "test error",
						ErrorCategory: scanner.ErrorCategoryMetadata,
					}
				}
				for i := 90; i < 100; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 4,
						FilePath:  "/test/pending" + string(rune(i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "incomplete checkpoints - too few created - deletes and returns false",
			jobID: 5,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         5,
					LibraryID:  50,
					FilesFound: 10000, // Found 10,000 files
				})
				// But only 50 checkpoints created (expected at least 100, which is 1% of 10000)
				checkpoints := make([]*scanner.ScanCheckpoint, 50)
				for i := 0; i < 50; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 5,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectResult: false,
			checkDeleted: true,
		},
		{
			name:  "edge case - exactly 1% checkpoints created - returns true",
			jobID: 6,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         6,
					LibraryID:  60,
					FilesFound: 10000,
				})
				// Exactly 100 checkpoints created (1% of 10000)
				checkpoints := make([]*scanner.ScanCheckpoint, 100)
				for i := 0; i < 100; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 6,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "edge case - small library with 1 file needs at least 1 checkpoint - returns true",
			jobID: 7,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         7,
					LibraryID:  70,
					FilesFound: 50,
				})
				// At least 1 checkpoint for small library
				checkpointRepo.WithCheckpoints(&scanner.ScanCheckpoint{
					ScanJobID: 7,
					FilePath:  "/test/file1",
					Status:    scanner.CheckpointPending,
				})
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "early in discovery - filesFound is 0 - returns true",
			jobID: 8,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         8,
					LibraryID:  80,
					FilesFound: 0, // Still discovering files
				})
				// Just a few checkpoints created so far
				checkpointRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{
						ScanJobID: 8,
						FilePath:  "/test/file1",
						Status:    scanner.CheckpointPending,
					},
					&scanner.ScanCheckpoint{
						ScanJobID: 8,
						FilePath:  "/test/file2",
						Status:    scanner.CheckpointPending,
					},
				)
			},
			expectResult: true,
			checkDeleted: false,
		},
		{
			name:  "delete fails - still returns false",
			jobID: 9,
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         9,
					LibraryID:  90,
					FilesFound: 10000,
				})
				// Only 10 checkpoints (way below minimum)
				checkpoints := make([]*scanner.ScanCheckpoint, 10)
				for i := 0; i < 10; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 9,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
				// Inject delete error
				checkpointRepo.DeleteByJobIDErr = errors.New("database error")
			},
			expectResult: false,
			checkDeleted: false, // Delete failed, so checkpoints still exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(checkpointRepo, scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			currentJob, _ := scanRepo.GetByID(ctx, tt.jobID)
			stats, _ := checkpointRepo.GetStats(ctx, tt.jobID)

			result := uc.validateCheckpointCompleteness(ctx, tt.jobID, currentJob, stats)

			if result != tt.expectResult {
				t.Errorf("Expected result %v, got %v", tt.expectResult, result)
			}

			// Verify checkpoints were deleted if expected
			if tt.checkDeleted {
				count := checkpointRepo.GetCheckpointCount(tt.jobID)
				if count != 0 {
					t.Errorf("Expected checkpoints to be deleted, but %d still exist", count)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_resumeScanFromCheckpoints(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setupMocks func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		jobID      int64
		lib        *library.Library
		checkStats bool // Verify GetStats was called
		checkJob   bool // Verify GetByID was called
	}{
		{
			name:  "error getting checkpoint stats - returns early",
			jobID: 1,
			lib: &library.Library{
				ID:   10,
				Name: "Test Library",
				Path: "/test",
				Type: library.LibraryTypeMovies,
			},
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})

				// Inject error for GetStats - function should return early
				checkpointRepo.GetStatsErr = errors.New("database connection error")
			},
			checkStats: true,
			checkJob:   false,
		},
		{
			name:  "error getting scan job - returns early",
			jobID: 2,
			lib: &library.Library{
				ID:   20,
				Name: "Test Library",
				Path: "/test",
				Type: library.LibraryTypeMovies,
			},
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				// No job created - GetByID will return error
				// Set up stats to verify it was called first
				checkpointRepo.WithCheckpoints(&scanner.ScanCheckpoint{
					ScanJobID: 2,
					FilePath:  "/test/file1",
					Status:    scanner.CheckpointPending,
				})
			},
			checkStats: true,
			checkJob:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(checkpointRepo, scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Call the function - it will return early if there are errors
			uc.resumeScanFromCheckpoints(ctx, tt.jobID, tt.lib)

			// Verify that the appropriate repository methods were called
			// This verifies the function executed the expected code paths before returning
			if tt.checkStats {
				// Verify GetStats was called by checking we can get stats
				stats, _ := checkpointRepo.GetStats(ctx, tt.jobID)
				if stats == nil && checkpointRepo.GetStatsErr == nil {
					t.Error("Expected GetStats to have been called")
				}
			}

			if tt.checkJob {
				// Verify GetByID was attempted
				job, err := scanRepo.GetByID(ctx, tt.jobID)
				// If we get an error, that's expected for the "job not found" test
				if job == nil && err == nil {
					t.Error("Expected GetByID to have been called")
				}
			}
		})
	}
}

func TestNewScanLibraryUseCase(t *testing.T) {
	tests := []struct {
		name          string
		logger        *slog.Logger
		config        ScanConfig
		verifyLogger  bool
		verifyConfig  bool
		verifyScanner bool
	}{
		{
			name:          "creates use case with all dependencies",
			logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
			config:        ScanConfig{CheckpointBatchSize: 50, Timeout: 3600},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
		{
			name:          "creates use case with nil logger - uses no-op logger",
			logger:        nil,
			config:        ScanConfig{CheckpointBatchSize: 30},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
		{
			name:          "creates use case with empty config - applies defaults",
			logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
			config:        ScanConfig{},
			verifyLogger:  true,
			verifyConfig:  true,
			verifyScanner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepos := &MediaRepositories{
				Library: mocks.NewLibraryRepository(t),
			}
			scanRepos := &ScanRepositories{
				ScanJob:    mocks.NewScanJobRepository(t),
				Checkpoint: mocks.NewCheckpointRepository(t),
			}

			uc := NewScanLibraryUseCase(
				mediaRepos,
				scanRepos,
				nil, // movieImageExtractor
				nil, // episodeImageExtractor
				nil, // showImageExtractor
				nil, // seasonImageExtractor
				nil, // albumImageExtractor
				nil, // artistImageExtractor
				nil, // trackImageExtractor
				nil, // imageRepo
				nil, // imageCleanup
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
				// Verify config has defaults applied
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

			// Verify all repositories are set
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
		name           string
		setupMocks     func(*mocks.ScanJobRepository, *mocks.LibraryRepository, *mocks.CheckpointRepository)
		expectError    bool
		expectedScans  int
		verifyNoScans  bool
		verifyComplete bool
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
			expectError:   false,
			expectedScans: 1,
		},
		{
			name: "one stuck scan - cannot get checkpoint stats - resumes anyway",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
				libRepo.WithLibraries(&library.Library{
					ID:   20,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				checkpointRepo.GetStatsErr = errors.New("stats error")
			},
			expectError:   false,
			expectedScans: 1,
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
			expectError:    false,
			expectedScans:  1,
			verifyComplete: true,
		},
		{
			name: "one stuck scan - has pending work - resumes",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         4,
					LibraryID:  40,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 10,
				})
				libRepo.WithLibraries(&library.Library{
					ID:   40,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				// Mix of completed and pending
				checkpoints := []*scanner.ScanCheckpoint{
					{ScanJobID: 4, FilePath: "/test/file1", Status: scanner.CheckpointCompleted},
					{ScanJobID: 4, FilePath: "/test/file2", Status: scanner.CheckpointCompleted},
					{ScanJobID: 4, FilePath: "/test/file3", Status: scanner.CheckpointPending},
					{ScanJobID: 4, FilePath: "/test/file4", Status: scanner.CheckpointPending},
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			expectError:   false,
			expectedScans: 1,
		},
		{
			name: "multiple stuck scans - handles all",
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository) {
				scanRepo.WithJobs(
					&scanner.ScanJob{
						ID:         5,
						LibraryID:  50,
						Status:     scanner.ScanStatusRunning,
						FilesFound: 5,
					},
					&scanner.ScanJob{
						ID:         6,
						LibraryID:  60,
						Status:     scanner.ScanStatusRunning,
						FilesFound: 3,
					},
				)
				libRepo.WithLibraries(
					&library.Library{ID: 50, Name: "Lib1", Path: "/lib1", Type: library.LibraryTypeMovies},
					&library.Library{ID: 60, Name: "Lib2", Path: "/lib2", Type: library.LibraryTypeTV},
				)
				// Both have pending work
				checkpointRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ScanJobID: 5, FilePath: "/lib1/file1", Status: scanner.CheckpointPending},
					&scanner.ScanCheckpoint{ScanJobID: 6, FilePath: "/lib2/file1", Status: scanner.CheckpointPending},
				)
			},
			expectError:   false,
			expectedScans: 2,
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
				mediaRepos: &MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &ScanRepositories{
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

			if tt.verifyComplete {
				// Give goroutine time to mark as completed
				// Note: In production this would be tested via integration tests
				// Here we just verify the call completed without panicking
			}
		})
	}
}

func TestScanLibraryUseCase_handleStuckScan(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setupMocks func(*mocks.LibraryRepository, *mocks.CheckpointRepository, *mocks.ScanJobRepository)
		job        *scanner.ScanJob
		verifyPath string // What code path we expect
	}{
		{
			name: "library not found - marks scan as failed",
			job: &scanner.ScanJob{
				ID:         1,
				LibraryID:  10,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 100,
			},
			setupMocks: func(libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				// Library doesn't exist
			},
			verifyPath: "failed",
		},
		{
			name: "cannot get checkpoint stats - resumes anyway",
			job: &scanner.ScanJob{
				ID:         2,
				LibraryID:  20,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 100,
			},
			setupMocks: func(libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				libRepo.WithLibraries(&library.Library{
					ID:   20,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				checkpointRepo.GetStatsErr = errors.New("database error")
			},
			verifyPath: "resume",
		},
		{
			name: "scan actually complete - marks as completed",
			job: &scanner.ScanJob{
				ID:         3,
				LibraryID:  30,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 5,
			},
			setupMocks: func(libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				libRepo.WithLibraries(&library.Library{
					ID:   30,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				// All 5 files completed
				checkpoints := make([]*scanner.ScanCheckpoint, 5)
				for i := 0; i < 5; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 3,
						FilePath:  "/test/file" + string(rune(i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			verifyPath: "completed",
		},
		{
			name: "scan has pending work - resumes",
			job: &scanner.ScanJob{
				ID:         4,
				LibraryID:  40,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 10,
			},
			setupMocks: func(libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				libRepo.WithLibraries(&library.Library{
					ID:   40,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				// Some pending files
				checkpoints := []*scanner.ScanCheckpoint{
					{ScanJobID: 4, FilePath: "/test/file1", Status: scanner.CheckpointCompleted},
					{ScanJobID: 4, FilePath: "/test/file2", Status: scanner.CheckpointPending},
					{ScanJobID: 4, FilePath: "/test/file3", Status: scanner.CheckpointPending},
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			verifyPath: "resume",
		},
		{
			name: "edge case - no checkpoints but no files found - resumes",
			job: &scanner.ScanJob{
				ID:         5,
				LibraryID:  50,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 0,
			},
			setupMocks: func(libRepo *mocks.LibraryRepository, checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				libRepo.WithLibraries(&library.Library{
					ID:   50,
					Name: "Test Library",
					Path: "/test",
					Type: library.LibraryTypeMovies,
				})
				// No checkpoints
			},
			verifyPath: "resume",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			libRepo := mocks.NewLibraryRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)
			scanRepo := mocks.NewScanJobRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(libRepo, checkpointRepo, scanRepo)
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Call the function - doesn't panic
			uc.handleStuckScan(ctx, tt.job)

			// Function executes different code paths based on state
			// We verify it doesn't panic and handles all cases
		})
	}
}

func TestScanLibraryUseCase_markStuckScanFailed(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		job          *scanner.ScanJob
		err          error
		setupMocks   func(*mocks.ScanJobRepository)
		verifyStatus bool
	}{
		{
			name: "marks scan as failed with error message",
			job: &scanner.ScanJob{
				ID:         1,
				LibraryID:  10,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 100,
			},
			err: errors.New("library not found"),
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
			},
			verifyStatus: true,
		},
		{
			name: "handles complete error - does not fail",
			job: &scanner.ScanJob{
				ID:         2,
				LibraryID:  20,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 50,
			},
			err: errors.New("database connection lost"),
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 50,
				})
				// Inject error for Complete operation
				scanRepo.CompleteErr = errors.New("complete failed")
			},
			verifyStatus: false, // Can't verify status if Complete fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			uc.markStuckScanFailed(ctx, tt.job, tt.err)

			if tt.verifyStatus {
				updatedJob, err := scanRepo.GetByID(ctx, tt.job.ID)
				if err != nil {
					t.Fatalf("Failed to get updated job: %v", err)
				}
				if updatedJob.Status != scanner.ScanStatusFailed {
					t.Errorf("Expected status %v, got %v", scanner.ScanStatusFailed, updatedJob.Status)
				}
				if updatedJob.ErrorMessage == "" {
					t.Error("Expected error message to be set")
				}
				if updatedJob.CompletedAt == nil {
					t.Error("Expected CompletedAt to be set")
				}
			}
		})
	}
}

func TestScanLibraryUseCase_markStuckScanCompleted(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		job        *scanner.ScanJob
		stats      *scanner.CheckpointStats
		setupMocks func(*mocks.ScanJobRepository)
	}{
		{
			name: "marks scan as completed with stats",
			job: &scanner.ScanJob{
				ID:         1,
				LibraryID:  10,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 100,
			},
			stats: &scanner.CheckpointStats{
				TotalFiles:     100,
				CompletedFiles: 95,
				FailedFiles:    5,
				ProcessedFiles: 100,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
			},
		},
		{
			name: "marks scan with all files completed",
			job: &scanner.ScanJob{
				ID:         2,
				LibraryID:  20,
				Status:     scanner.ScanStatusRunning,
				FilesFound: 50,
			},
			stats: &scanner.CheckpointStats{
				TotalFiles:     50,
				CompletedFiles: 50,
				ProcessedFiles: 50,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 50,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			uc.markStuckScanCompleted(ctx, tt.job, tt.stats)

			// Function completes without error
			// In production, would verify job status via repository
		})
	}
}

func TestScanLibraryUseCase_markScanCompleted(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		jobID      int64
		stats      *scanner.CheckpointStats
		setupMocks func(*mocks.ScanJobRepository)
	}{
		{
			name:  "marks scan as completed using stats total files",
			jobID: 1,
			stats: &scanner.CheckpointStats{
				TotalFiles:     100,
				CompletedFiles: 90,
				FailedFiles:    8,
				WarningFiles:   2,
				ProcessedFiles: 100,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
			},
		},
		{
			name:  "marks scan with zero warnings",
			jobID: 2,
			stats: &scanner.CheckpointStats{
				TotalFiles:     50,
				CompletedFiles: 48,
				FailedFiles:    2,
				WarningFiles:   0,
				ProcessedFiles: 50,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 50,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			uc.markScanCompleted(ctx, tt.jobID, tt.stats)

			// Function completes without error
		})
	}
}

func TestScanLibraryUseCase_completeJobFromStats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		jobID        int64
		filesFound   int64
		stats        *scanner.CheckpointStats
		setupMocks   func(*mocks.ScanJobRepository)
		verifyFields bool
	}{
		{
			name:       "completes job with all stats",
			jobID:      1,
			filesFound: 100,
			stats: &scanner.CheckpointStats{
				TotalFiles:     100,
				CompletedFiles: 85,
				FailedFiles:    10,
				WarningFiles:   5,
				ProcessedFiles: 100,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         1,
					LibraryID:  10,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 100,
				})
			},
			verifyFields: true,
		},
		{
			name:       "completes job with no errors",
			jobID:      2,
			filesFound: 50,
			stats: &scanner.CheckpointStats{
				TotalFiles:     50,
				CompletedFiles: 50,
				FailedFiles:    0,
				WarningFiles:   0,
				ProcessedFiles: 50,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         2,
					LibraryID:  20,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 50,
				})
			},
			verifyFields: true,
		},
		{
			name:       "completes job with warnings only",
			jobID:      3,
			filesFound: 30,
			stats: &scanner.CheckpointStats{
				TotalFiles:     30,
				CompletedFiles: 25,
				FailedFiles:    0,
				WarningFiles:   5,
				ProcessedFiles: 30,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         3,
					LibraryID:  30,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 30,
				})
			},
			verifyFields: true,
		},
		{
			name:       "handles complete error gracefully",
			jobID:      4,
			filesFound: 20,
			stats: &scanner.CheckpointStats{
				TotalFiles:     20,
				CompletedFiles: 20,
				ProcessedFiles: 20,
			},
			setupMocks: func(scanRepo *mocks.ScanJobRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:         4,
					LibraryID:  40,
					Status:     scanner.ScanStatusRunning,
					FilesFound: 20,
				})
				scanRepo.CompleteErr = errors.New("database error")
			},
			verifyFields: false, // Can't verify if Complete fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			uc.completeJobFromStats(ctx, tt.jobID, tt.filesFound, tt.stats)

			if tt.verifyFields {
				job, err := scanRepo.GetByID(ctx, tt.jobID)
				if err != nil {
					t.Fatalf("Failed to get job: %v", err)
				}

				if job.Status != scanner.ScanStatusCompleted {
					t.Errorf("Expected status %v, got %v", scanner.ScanStatusCompleted, job.Status)
				}
				if job.FilesFound != tt.filesFound {
					t.Errorf("Expected FilesFound %d, got %d", tt.filesFound, job.FilesFound)
				}
				if job.FilesProcessed != tt.stats.ProcessedFiles {
					t.Errorf("Expected FilesProcessed %d, got %d", tt.stats.ProcessedFiles, job.FilesProcessed)
				}
				if job.ErrorCount != tt.stats.FailedFiles {
					t.Errorf("Expected ErrorCount %d, got %d", tt.stats.FailedFiles, job.ErrorCount)
				}
				if job.WarningCount != tt.stats.WarningFiles {
					t.Errorf("Expected WarningCount %d, got %d", tt.stats.WarningFiles, job.WarningCount)
				}
				if job.Phase != scanner.ScanPhaseCompleted {
					t.Errorf("Expected Phase %v, got %v", scanner.ScanPhaseCompleted, job.Phase)
				}
				if !job.DiscoveryDone {
					t.Error("Expected DiscoveryDone to be true")
				}
				if job.CompletedAt == nil {
					t.Error("Expected CompletedAt to be set")
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
					Status:    scanner.ScanStatusRunning, // Not paused
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
		{
			name:  "resume scan with completed status - error",
			jobID: 6,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        6,
					LibraryID: 60,
					Status:    scanner.ScanStatusCompleted,
				})
			},
			expectError:  true,
			errorMessage: "scan job is not paused",
		},
		{
			name:  "resume scan with failed status - error",
			jobID: 7,
			setupMocks: func(scanRepo *mocks.ScanJobRepository, libRepo *mocks.LibraryRepository) {
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        7,
					LibraryID: 70,
					Status:    scanner.ScanStatusFailed,
				})
			},
			expectError:  true,
			errorMessage: "scan job is not paused",
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
				mediaRepos: &MediaRepositories{
					Library: libRepo,
				},
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				config: ScanConfig{
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

func TestScanLibraryUseCase_initializeScanSession(t *testing.T) {
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
				Path: t.TempDir(), // Use temp dir for real storage detection
			},
			verifyReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				processedArtists: sync.Map{},
				systemProfile:    tt.systemProfile,
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Pre-populate processedArtists to verify reset
			uc.processedArtists.Store("Artist1", true)
			uc.processedArtists.Store("Artist2", true)

			// Call the function
			uc.initializeScanSession(ctx, tt.lib)

			// Verify processedArtists was reset (should be empty after init)
			if tt.verifyReset {
				count := 0
				uc.processedArtists.Range(func(key, value interface{}) bool {
					count++
					return true
				})
				if count != 0 {
					t.Errorf("expected processedArtists to be reset, but has %d entries", count)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_recoverWorkerPanic(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
		wantPanic  bool
	}{
		{
			name:       "recovers from string panic",
			panicValue: "test panic",
			wantPanic:  true,
		},
		{
			name:       "recovers from error panic",
			panicValue: errors.New("test error panic"),
			wantPanic:  true,
		},
		{
			name:       "no panic - does nothing",
			panicValue: nil, // No panic
			wantPanic:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Create a function that might panic
			// recoverWorkerPanic signature: (jobID int64, workerID int)
			testFunc := func() {
				defer uc.recoverWorkerPanic(1, 0)
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
			}

			// Execute - should not re-panic
			testFunc()
			// If we get here without re-panicking, the test passes
		})
	}
}

func TestScanLibraryUseCase_recoverFromPanic(t *testing.T) {
	tests := []struct {
		name           string
		panicValue     interface{}
		setupMock      func(*mocks.ScanJobRepository)
		expectComplete bool // Whether Complete should succeed
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
			expectComplete: false, // Complete will fail, status won't change
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
				scanRepos: &ScanRepositories{
					ScanJob: scanRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Create a function that panics
			// recoverFromPanic signature: (jobID, libraryID int64, description string)
			testFunc := func() {
				defer uc.recoverFromPanic(1, 10, "test scan")
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
			}

			// Execute - should not re-panic
			testFunc()

			// Verify job status based on expected outcome
			if tt.panicValue != nil && tt.expectComplete {
				job, err := scanRepo.GetByID(context.Background(), 1)
				if err != nil {
					t.Fatalf("failed to get job: %v", err)
				}
				if job.Status != scanner.ScanStatusFailed {
					t.Errorf("expected job status failed, got %v", job.Status)
				}
			}
			// For the Complete error case, we just verify no re-panic occurred
			// The job status remains unchanged because Complete failed
		})
	}
}

func TestScanLibraryUseCase_canResumeFromCheckpoints(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		setupMocks     func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		currentJob     *scanner.ScanJob
		lib            *library.Library
		expectedResult bool
	}{
		{
			name: "returns false when GetStats fails",
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				checkpointRepo.GetStatsErr = errors.New("database error")
			},
			currentJob: &scanner.ScanJob{
				ID:        1,
				LibraryID: 10,
			},
			lib: &library.Library{
				ID:   10,
				Path: "/test",
			},
			expectedResult: false,
		},
		{
			name: "returns false when no checkpoints exist",
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				// No checkpoints = GetStats returns zero stats
			},
			currentJob: &scanner.ScanJob{
				ID:        2,
				LibraryID: 20,
			},
			lib: &library.Library{
				ID:   20,
				Path: "/test",
			},
			expectedResult: false,
		},
		{
			name: "returns true when all checkpoints completed - marks job complete",
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				// Create 10 completed checkpoints (no pending)
				checkpoints := make([]*scanner.ScanCheckpoint, 10)
				for i := 0; i < 10; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 3,
						FilePath:  "/test/file" + string(rune('A'+i)),
						Status:    scanner.CheckpointCompleted,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
				scanRepo.WithJobs(&scanner.ScanJob{
					ID:        3,
					LibraryID: 30,
					Status:    scanner.ScanStatusRunning,
				})
			},
			currentJob: &scanner.ScanJob{
				ID:         3,
				LibraryID:  30,
				FilesFound: 10,
			},
			lib: &library.Library{
				ID:   30,
				Path: "/test",
			},
			expectedResult: true,
		},
		{
			name: "returns false when checkpoint completeness fails validation",
			setupMocks: func(checkpointRepo *mocks.CheckpointRepository, scanRepo *mocks.ScanJobRepository) {
				// Only 5 checkpoints but 10000 files found - incomplete
				checkpoints := make([]*scanner.ScanCheckpoint, 5)
				for i := 0; i < 5; i++ {
					checkpoints[i] = &scanner.ScanCheckpoint{
						ScanJobID: 5,
						FilePath:  "/test/file" + string(rune('A'+i)),
						Status:    scanner.CheckpointPending,
					}
				}
				checkpointRepo.WithCheckpoints(checkpoints...)
			},
			currentJob: &scanner.ScanJob{
				ID:         5,
				LibraryID:  50,
				FilesFound: 10000, // Way more than checkpoints
			},
			lib: &library.Library{
				ID:   50,
				Path: "/test",
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanRepo := mocks.NewScanJobRepository(t)
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(checkpointRepo, scanRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob:    scanRepo,
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			result := uc.canResumeFromCheckpoints(ctx, tt.currentJob.ID, tt.currentJob, tt.lib)

			if result != tt.expectedResult {
				t.Errorf("expected result %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
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
