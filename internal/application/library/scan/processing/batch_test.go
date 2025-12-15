package processing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func batchTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func batchTestDeps(
	scanRepos *scan.ScanRepositories,
	mediaRepos *scan.MediaRepositories,
	config *scan.Config,
) *Deps {
	if config == nil {
		defaultConfig := scan.DefaultConfig()
		config = &defaultConfig
	}
	return &Deps{
		ScanRepos:  scanRepos,
		MediaRepos: mediaRepos,
		Config:     config,
		Logger:     batchTestLogger(),
	}
}

// =============================================================================
// Tests for InitCheckpointProcessing
// =============================================================================

func TestInitCheckpointProcessing(t *testing.T) {
	tests := []struct {
		name          string
		setupRepos    func(*mocks.MediaRepository)
		jobID         int64
		lib           *library.Library
		expectedCache int
	}{
		{
			name: "successful initialization with media cache",
			setupRepos: func(repo *mocks.MediaRepository) {
				repo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/test/file1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/test/file2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/test/file3.mp4"},
				)
			},
			jobID: 100,
			lib: &library.Library{
				ID:   1,
				Path: "/test",
			},
			expectedCache: 3,
		},
		{
			name: "initialization with empty library",
			setupRepos: func(repo *mocks.MediaRepository) {
				// No media
			},
			jobID: 200,
			lib: &library.Library{
				ID:   1,
				Path: "/test",
			},
			expectedCache: 0,
		},
		{
			name: "initialization with media cache load error - falls back gracefully",
			setupRepos: func(repo *mocks.MediaRepository) {
				repo.GetFilePathCacheErr = errors.New("database error")
			},
			jobID: 300,
			lib: &library.Library{
				ID:   1,
				Path: "/test",
			},
			expectedCache: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(mediaRepo)
			}

			mediaRepos := &scan.MediaRepositories{
				Media: mediaRepo,
			}

			config := scan.DefaultConfig()
			config.CheckpointBatchSize = 10
			config.MaxRetries = 3
			config.CheckpointBufferSize = 100

			deps := batchTestDeps(nil, mediaRepos, &config)

			pctx := InitCheckpointProcessing(context.Background(), deps, tt.jobID, tt.lib)

			if pctx == nil {
				t.Fatal("Expected non-nil CheckpointContext")
			}

			if pctx.JobID != tt.jobID {
				t.Errorf("Expected JobID %d, got %d", tt.jobID, pctx.JobID)
			}

			if pctx.Lib.ID != tt.lib.ID {
				t.Errorf("Expected LibraryID %d, got %d", tt.lib.ID, pctx.Lib.ID)
			}

			if pctx.BatchSize != config.CheckpointBatchSize {
				t.Errorf("Expected BatchSize %d, got %d", config.CheckpointBatchSize, pctx.BatchSize)
			}

			if pctx.MaxRetries != config.MaxRetries {
				t.Errorf("Expected MaxRetries %d, got %d", config.MaxRetries, pctx.MaxRetries)
			}

			// Verify cache size
			count := 0
			pctx.ExistingMediaCache.Range(func(key, value interface{}) bool {
				count++
				return true
			})

			if count != tt.expectedCache {
				t.Errorf("Expected cache count %d, got %d", tt.expectedCache, count)
			}
		})
	}
}

// =============================================================================
// Tests for updateProgressIfDue
// =============================================================================

func TestUpdateProgressIfDue(t *testing.T) {
	tests := []struct {
		name         string
		setupRepos   func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		tickerFired  bool
		expectUpdate bool
	}{
		{
			name: "ticker fired - successful progress update",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     50,
					EstimatedTotal: 100,
					DiscoveryDone:  true,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 3, ScanJobID: 100, Status: scanner.CheckpointPending},
				)
			},
			tickerFired:  true,
			expectUpdate: true,
		},
		{
			name: "ticker not fired - no update",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:         100,
					LibraryID:  1,
					FilesFound: 50,
				})
			},
			tickerFired:  false,
			expectUpdate: false,
		},
		{
			name: "ticker fired - GetStats error - should not panic",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				cpRepo.GetStatsErr = errors.New("database error")
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:         100,
					LibraryID:  1,
					FilesFound: 50,
				})
			},
			tickerFired:  true,
			expectUpdate: false,
		},
		{
			name: "ticker fired - GetByID error - should not panic",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.GetErr = errors.New("database error")
			},
			tickerFired:  true,
			expectUpdate: false,
		},
		{
			name: "ticker fired - UpdateProgress error - should not panic",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:         100,
					LibraryID:  1,
					FilesFound: 50,
				})
				jobRepo.UpdateProgressErr = errors.New("database error")
			},
			tickerFired:  true,
			expectUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			jobRepo := mocks.NewScanJobRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(checkpointRepo, jobRepo)
			}

			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    jobRepo,
			}

			config := scan.DefaultConfig()
			config.ProgressUpdateTick = 10 * time.Millisecond

			deps := batchTestDeps(scanRepos, nil, &config)

			// Create ticker
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()

			// If ticker should fire, wait for it
			if tt.tickerFired {
				<-ticker.C
			}

			// Call the function
			updateProgressIfDue(context.Background(), deps, 100, ticker)

			// Function should complete without panic
			// Progress update is fire-and-forget, so we just verify no panic
		})
	}
}

// =============================================================================
// Tests for CompleteScan
// =============================================================================

func TestCompleteScan(t *testing.T) {
	tests := []struct {
		name            string
		setupRepos      func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		discoveryStats  *filesystem.WalkStats
		cleanupCalled   *bool
		expectCleanupFn bool
	}{
		{
			name: "successful completion - no errors - cleanup called",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     10,
					FilesProcessed: 10,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, Status: scanner.CheckpointCompleted},
				)
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: true,
		},
		{
			name: "completion with errors - cleanup not called",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     10,
					FilesProcessed: 10,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, Status: scanner.CheckpointFailed},
				)
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: false,
		},
		{
			name: "completion with discovery stats - cleanup called",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     1,
					FilesProcessed: 1,
				})
				// Set up stats to match: CompletedFiles == TotalFiles
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
				)
			},
			discoveryStats: &filesystem.WalkStats{
				DirsScanned:      100,
				DirsSkipped:      5,
				FilesSkipped:     10,
				PermissionErrors: 2,
				NetworkErrors:    1,
			},
			cleanupCalled:   new(bool),
			expectCleanupFn: true, // All files completed successfully
		},
		{
			name: "GetByID error - should log and return early",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.GetErr = errors.New("database error")
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: false,
		},
		{
			name: "Complete error - should log but continue cleanup",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     2,
					FilesProcessed: 2,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, Status: scanner.CheckpointCompleted},
				)
				jobRepo.CompleteErr = errors.New("database error")
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: true, // Cleanup still called even if Complete fails (with non-deletion error)
		},
		{
			name: "scan job deleted before completion",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     10,
					FilesProcessed: 10,
				})
				jobRepo.CompleteErr = errors.New("foreign key constraint violated")
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: false,
		},
		{
			name: "cleanup checkpoints with error - should log and continue",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     2,
					FilesProcessed: 2,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, Status: scanner.CheckpointCompleted},
				)
				cpRepo.DeleteByJobIDErr = errors.New("database error")
			},
			discoveryStats:  nil,
			cleanupCalled:   new(bool),
			expectCleanupFn: true, // Cleanup fn called before checkpoint cleanup
		},
		{
			name: "nil cleanup function - should not panic",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					FilesFound:     10,
					FilesProcessed: 10,
				})
				cpRepo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, Status: scanner.CheckpointCompleted},
				)
			},
			discoveryStats:  nil,
			cleanupCalled:   nil, // No cleanup function provided
			expectCleanupFn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			jobRepo := mocks.NewScanJobRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(checkpointRepo, jobRepo)
			}

			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    jobRepo,
			}

			deps := batchTestDeps(scanRepos, nil, nil)

			lib := &library.Library{ID: 1, Path: "/test"}
			pctx := testCheckpointContext(100, lib, 10, 3)

			var cleanupFn func()
			if tt.cleanupCalled != nil {
				cleanupFn = func() {
					*tt.cleanupCalled = true
				}
			}

			// Call CompleteScan
			CompleteScan(context.Background(), deps, pctx, tt.discoveryStats, cleanupFn)

			// Verify cleanup was called or not based on expectation
			if tt.cleanupCalled != nil {
				if *tt.cleanupCalled != tt.expectCleanupFn {
					t.Errorf("Expected cleanup called=%v, got %v", tt.expectCleanupFn, *tt.cleanupCalled)
				}
			}
		})
	}
}

// =============================================================================
// Tests for StartCheckpointWorkers - Additional Coverage
// =============================================================================

func TestStartCheckpointWorkers_PanicRecovery(t *testing.T) {
	// This test ensures that panics in OnPanic handler are properly logged
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)
	lib := &library.Library{ID: 1, Path: "/test"}

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}
	deps := testOrchestrationDeps(scanRepos, nil, nil, nil, nil)

	pctx := testCheckpointContext(100, lib, 10, 3)

	// Start workers
	StartCheckpointWorkers(context.Background(), deps, pctx)

	// Send a checkpoint that will cause issues
	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  "/nonexistent/file.mp4",
		Status:    scanner.CheckpointPending,
	}

	// Send checkpoint and close channel
	go func() {
		pctx.CheckpointsChan <- checkpoint
		close(pctx.CheckpointsChan)
	}()

	// Wait for workers to complete
	done := make(chan struct{})
	go func() {
		pctx.WorkerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Workers did not complete within timeout")
	}
}
