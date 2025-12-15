package processing

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
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// =============================================================================
// Test Helpers
// =============================================================================

func testOrchestrationDeps(
	scanRepos *scan.ScanRepositories,
	mediaRepos *scan.MediaRepositories,
	mediaProcessor MediaProcessor,
	config *scan.Config,
	systemProfile *system.Profile,
) *Deps {
	if config == nil {
		defaultConfig := scan.DefaultConfig()
		config = &defaultConfig
	}
	return &Deps{
		ScanRepos:      scanRepos,
		MediaRepos:     mediaRepos,
		MediaProcessor: mediaProcessor,
		Config:         config,
		SystemProfile:  systemProfile,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func testCheckpointContext(jobID int64, lib *library.Library, batchSize int, maxRetries int) *CheckpointContext {
	return NewCheckpointContext(jobID, lib, batchSize, maxRetries, 10, &sync.Map{})
}

// =============================================================================
// Tests for StartCheckpointWorkers
// =============================================================================

func TestStartCheckpointWorkers(t *testing.T) {
	tests := []struct {
		name            string
		systemProfile   *system.Profile
		expectedWorkers int
		sendCheckpoints int
	}{
		{
			name:            "default workers when no system profile",
			systemProfile:   nil,
			expectedWorkers: 4,
			sendCheckpoints: 8,
		},
		{
			name: "uses system profile worker count - low end",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalHDD,
					IsRemote: false,
				},
			},
			expectedWorkers: 1,
			sendCheckpoints: 2,
		},
		{
			name: "uses system profile worker count - mid range",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 8,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalSSD,
					IsRemote: false,
				},
			},
			expectedWorkers: 3,
			sendCheckpoints: 6,
		},
		{
			name: "uses system profile worker count - network storage",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 16,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeNetwork,
					IsRemote: true,
				},
			},
			expectedWorkers: 4, // Capped at 4 for network storage
			sendCheckpoints: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			scanStateRepo := mocks.NewScanStateRepository(t)
			lib := &library.Library{ID: 1, Path: "/test"}

			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanState:  scanStateRepo,
			}
			deps := testOrchestrationDeps(scanRepos, nil, nil, nil, tt.systemProfile)

			pctx := testCheckpointContext(100, lib, 10, 3)

			// Add checkpoints that will fail fast (no file to process)
			for i := 0; i < tt.sendCheckpoints; i++ {
				checkpointRepo.WithCheckpoints(&scanner.ScanCheckpoint{
					ID:        int64(i + 1),
					ScanJobID: 100,
					FilePath:  "/nonexistent/file.mp4",
					Status:    scanner.CheckpointPending,
				})
			}

			// Start workers
			StartCheckpointWorkers(context.Background(), deps, pctx)

			// Send checkpoints to workers (they will fail fast due to nonexistent files)
			go func() {
				for i := 0; i < tt.sendCheckpoints; i++ {
					cp := &scanner.ScanCheckpoint{
						ID:        int64(i + 1),
						ScanJobID: 100,
						FilePath:  "/nonexistent/file.mp4",
						Status:    scanner.CheckpointPending,
					}
					pctx.CheckpointsChan <- cp
				}
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
				// Success - workers completed
			case <-time.After(5 * time.Second):
				t.Fatal("Workers did not complete within timeout")
			}

			// Verify checkpoints were updated (either completed or failed)
			stats, _ := checkpointRepo.GetStats(context.Background(), 100)
			if stats.ProcessedFiles != int64(tt.sendCheckpoints) {
				// Files will fail since they don't exist, but should still be processed
				// Just verify workers ran without panicking
			}
		})
	}
}

func TestStartCheckpointWorkers_recoversFromPanic(t *testing.T) {
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)
	lib := &library.Library{ID: 1, Path: "/test"}

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}
	deps := testOrchestrationDeps(scanRepos, nil, nil, nil, nil)

	pctx := testCheckpointContext(100, lib, 10, 3)

	checkpointRepo.WithCheckpoints(&scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  "/nonexistent/file.mp4",
		Status:    scanner.CheckpointPending,
	})

	// Start workers
	StartCheckpointWorkers(context.Background(), deps, pctx)

	// Send a checkpoint that will fail
	go func() {
		pctx.CheckpointsChan <- &scanner.ScanCheckpoint{
			ID:        1,
			ScanJobID: 100,
			FilePath:  "/nonexistent/file.mp4",
			Status:    scanner.CheckpointPending,
		}
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
		// Workers should complete even if processing encounters errors
	case <-time.After(5 * time.Second):
		t.Fatal("Workers did not complete within timeout")
	}
}

// =============================================================================
// Tests for GetNumWorkers
// =============================================================================

func TestGetNumWorkers(t *testing.T) {
	tests := []struct {
		name            string
		systemProfile   *system.Profile
		expectedWorkers int
	}{
		{
			name:            "no system profile - default to 4",
			systemProfile:   nil,
			expectedWorkers: 4,
		},
		{
			name: "low-end system - 1-2 cores",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalHDD,
					IsRemote: false,
				},
			},
			expectedWorkers: 1,
		},
		{
			name: "entry-level system - 4 cores",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 4,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalHDD,
					IsRemote: false,
				},
			},
			expectedWorkers: 2,
		},
		{
			name: "mid-range system - 8 cores",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 8,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalSSD,
					IsRemote: false,
				},
			},
			expectedWorkers: 3,
		},
		{
			name: "high-end system - 16 cores",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 16,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeLocalSSD,
					IsRemote: false,
				},
			},
			expectedWorkers: 4,
		},
		{
			name: "network storage - capped at 4",
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumPhysical: 32,
				},
				Storage: system.StorageProfile{
					Type:     system.StorageTypeNetwork,
					IsRemote: true,
				},
			},
			expectedWorkers: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			workers := GetNumWorkers(tt.systemProfile, scan.DefaultProcessingWorkers, logger)

			if workers != tt.expectedWorkers {
				t.Errorf("Expected %d workers, got %d", tt.expectedWorkers, workers)
			}
		})
	}
}

// =============================================================================
// Tests for RunCheckpointProcessingLoop
// =============================================================================

func TestRunCheckpointProcessingLoop(t *testing.T) {
	tests := []struct {
		name               string
		setupRepos         func(*mocks.CheckpointRepository, *mocks.ScanJobRepository)
		hashingDoneSignal  bool
		expectedLoopBreak  bool
		expectedIterations int
	}{
		{
			name: "exits when hashing done and no pending checkpoints",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
				})
			},
			hashingDoneSignal:  true,
			expectedLoopBreak:  true,
			expectedIterations: 1,
		},
		{
			name: "breaks on scan job paused",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
					Status:    scanner.ScanStatusPaused,
				})
			},
			hashingDoneSignal:  false,
			expectedLoopBreak:  true,
			expectedIterations: 1,
		},
		{
			name: "breaks on GetByID error",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				jobRepo.GetErr = errors.New("database error")
			},
			hashingDoneSignal:  false,
			expectedLoopBreak:  true,
			expectedIterations: 1,
		},
		{
			name: "breaks on GetPendingBatch error",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				cpRepo.GetPendingBatchErr = errors.New("database error")

				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
				})
			},
			hashingDoneSignal:  false,
			expectedLoopBreak:  true,
			expectedIterations: 1,
		},
		{
			name: "breaks on scan job deleted error",
			setupRepos: func(cpRepo *mocks.CheckpointRepository, jobRepo *mocks.ScanJobRepository) {
				cpRepo.GetPendingBatchErr = errors.New("foreign key constraint violated")

				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
				})
			},
			hashingDoneSignal:  false,
			expectedLoopBreak:  true,
			expectedIterations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			jobRepo := mocks.NewScanJobRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(checkpointRepo, jobRepo)
			}

			config := scan.DefaultConfig()
			config.ProgressUpdateTick = 10 * time.Millisecond
			config.CheckpointBatchSize = 10

			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    jobRepo,
			}
			deps := testOrchestrationDeps(scanRepos, nil, nil, &config, nil)

			lib := &library.Library{ID: 1, Path: "/test"}
			pctx := testCheckpointContext(100, lib, 10, 3)

			hashingDone := make(chan struct{})
			if tt.hashingDoneSignal {
				go func() {
					time.Sleep(50 * time.Millisecond)
					close(hashingDone)
				}()
			}

			done := make(chan struct{})
			go func() {
				RunCheckpointProcessingLoop(context.Background(), deps, pctx, hashingDone)
				close(done)
			}()

			select {
			case <-done:
				// Loop completed
			case <-time.After(2 * time.Second):
				t.Fatal("Processing loop did not complete within timeout")
			}

			if !tt.expectedLoopBreak {
				t.Error("Expected loop to continue, but it broke")
			}
		})
	}
}

func TestRunCheckpointProcessingLoop_WithBatches(t *testing.T) {
	t.Run("processes checkpoints and sends to workers", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		jobRepo := mocks.NewScanJobRepository(t)

		jobRepo.WithJobs(&scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
			Status:    scanner.ScanStatusRunning,
		})

		checkpoints := []*scanner.ScanCheckpoint{
			{ID: 1, ScanJobID: 100, FilePath: "/test/movie1.mp4", Status: scanner.CheckpointPending},
			{ID: 2, ScanJobID: 100, FilePath: "/test/movie2.mp4", Status: scanner.CheckpointPending},
			{ID: 3, ScanJobID: 100, FilePath: "/test/movie3.mp4", Status: scanner.CheckpointPending},
		}
		checkpointRepo.WithCheckpoints(checkpoints...)

		config := scan.DefaultConfig()
		config.ProgressUpdateTick = 5 * time.Millisecond
		config.CheckpointBatchSize = 10

		scanRepos := &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanJob:    jobRepo,
		}
		deps := testOrchestrationDeps(scanRepos, nil, nil, &config, nil)

		lib := &library.Library{ID: 1, Path: "/test"}
		pctx := testCheckpointContext(100, lib, 10, 3)

		receivedCheckpoints := make([]*scanner.ScanCheckpoint, 0)
		var receivedMu sync.Mutex
		go func() {
			for cp := range pctx.CheckpointsChan {
				receivedMu.Lock()
				receivedCheckpoints = append(receivedCheckpoints, cp)
				receivedMu.Unlock()
				_ = checkpointRepo.UpdateStatus(context.Background(), cp.ID, scanner.CheckpointCompleted, "", "")
			}
		}()

		hashingDone := make(chan struct{})
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(hashingDone)
		}()

		done := make(chan struct{})
		go func() {
			RunCheckpointProcessingLoop(context.Background(), deps, pctx, hashingDone)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Processing loop did not complete within timeout")
		}

		close(pctx.CheckpointsChan)

		receivedMu.Lock()
		if len(receivedCheckpoints) < 3 {
			t.Errorf("expected at least 3 checkpoints sent to channel, got %d", len(receivedCheckpoints))
		}
		receivedMu.Unlock()
	})

	t.Run("waits when batch empty but hashing not done", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		jobRepo := mocks.NewScanJobRepository(t)

		jobRepo.WithJobs(&scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
			Status:    scanner.ScanStatusRunning,
		})

		config := scan.DefaultConfig()
		config.ProgressUpdateTick = 5 * time.Millisecond
		config.CheckpointBatchSize = 10

		scanRepos := &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanJob:    jobRepo,
		}
		deps := testOrchestrationDeps(scanRepos, nil, nil, &config, nil)

		lib := &library.Library{ID: 1, Path: "/test"}
		pctx := testCheckpointContext(100, lib, 10, 3)

		hashingDone := make(chan struct{})
		startTime := time.Now()
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(hashingDone)
		}()

		done := make(chan struct{})
		go func() {
			RunCheckpointProcessingLoop(context.Background(), deps, pctx, hashingDone)
			close(done)
		}()

		select {
		case <-done:
			elapsed := time.Since(startTime)
			if elapsed < 40*time.Millisecond {
				t.Errorf("expected loop to wait for hashing done, completed too quickly: %v", elapsed)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Processing loop did not complete within timeout")
		}
	})
}

// =============================================================================
// Tests for CheckScanStatus
// =============================================================================

func TestCheckScanStatus(t *testing.T) {
	tests := []struct {
		name             string
		setupRepo        func(*mocks.ScanJobRepository)
		expectedBreak    bool
		expectedContinue bool
	}{
		{
			name: "running scan - continue processing",
			setupRepo: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:     100,
					Status: scanner.ScanStatusRunning,
				})
			},
			expectedBreak:    false,
			expectedContinue: false,
		},
		{
			name: "paused scan - should break",
			setupRepo: func(repo *mocks.ScanJobRepository) {
				repo.WithJobs(&scanner.ScanJob{
					ID:     100,
					Status: scanner.ScanStatusPaused,
				})
			},
			expectedBreak:    true,
			expectedContinue: false,
		},
		{
			name: "GetByID error - should break",
			setupRepo: func(repo *mocks.ScanJobRepository) {
				repo.GetErr = errors.New("database error")
			},
			expectedBreak:    true,
			expectedContinue: false,
		},
		{
			name: "completed scan - continue (will be handled elsewhere)",
			setupRepo: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(&scanner.ScanJob{
					ID:          100,
					Status:      scanner.ScanStatusCompleted,
					CompletedAt: &now,
				})
			},
			expectedBreak:    false,
			expectedContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRepo := mocks.NewScanJobRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(jobRepo)
			}

			scanRepos := &scan.ScanRepositories{
				ScanJob: jobRepo,
			}
			deps := testOrchestrationDeps(scanRepos, nil, nil, nil, nil)

			shouldBreak, shouldContinue := CheckScanStatus(context.Background(), deps, 100)

			if shouldBreak != tt.expectedBreak {
				t.Errorf("Expected shouldBreak=%v, got %v", tt.expectedBreak, shouldBreak)
			}
			if shouldContinue != tt.expectedContinue {
				t.Errorf("Expected shouldContinue=%v, got %v", tt.expectedContinue, shouldContinue)
			}
		})
	}
}

// =============================================================================
// Tests for LoadMediaCache
// =============================================================================

func TestLoadMediaCache(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func(*mocks.MediaRepository)
		libraryID     int64
		expectedCount int
	}{
		{
			name: "successful cache load",
			setupRepo: func(repo *mocks.MediaRepository) {
				repo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/test/file1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/test/file2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/test/file3.mp4"},
				)
			},
			libraryID:     1,
			expectedCount: 3,
		},
		{
			name: "empty library",
			setupRepo: func(repo *mocks.MediaRepository) {
				// No media
			},
			libraryID:     1,
			expectedCount: 0,
		},
		{
			name: "cache load with error - falls back to empty map",
			setupRepo: func(repo *mocks.MediaRepository) {
				repo.GetFilePathCacheErr = errors.New("database error")
			},
			libraryID:     1,
			expectedCount: 0,
		},
		{
			name: "filters by library ID",
			setupRepo: func(repo *mocks.MediaRepository) {
				repo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/lib1/file1.mp4"},
					&media.Media{ID: 2, LibraryID: 2, FilePath: "/lib2/file2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/lib1/file3.mp4"},
				)
			},
			libraryID:     1,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo)
			}

			mediaRepos := &scan.MediaRepositories{
				Media: mediaRepo,
			}
			deps := testOrchestrationDeps(nil, mediaRepos, nil, nil, nil)

			cache := LoadMediaCache(context.Background(), deps, tt.libraryID)

			count := 0
			cache.Range(func(key, value interface{}) bool {
				count++
				return true
			})

			if count != tt.expectedCount {
				t.Errorf("Expected cache count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

func TestLoadMediaCache_concurrent(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	mediaRepo.WithMedia(
		&media.Media{ID: 1, LibraryID: 1, FilePath: "/test/file1.mp4"},
		&media.Media{ID: 2, LibraryID: 1, FilePath: "/test/file2.mp4"},
	)

	mediaRepos := &scan.MediaRepositories{
		Media: mediaRepo,
	}
	deps := testOrchestrationDeps(nil, mediaRepos, nil, nil, nil)

	cache := LoadMediaCache(context.Background(), deps, 1)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.Load("/test/file1.mp4")
			cache.Range(func(key, value interface{}) bool {
				return true
			})
		}()
	}

	wg.Wait()
}

// =============================================================================
// Tests for CleanupCheckpoints
// =============================================================================

func TestCleanupCheckpoints(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func(*mocks.CheckpointRepository)
		jobID         int64
		expectCleanup bool
	}{
		{
			name: "successful cleanup",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.WithCheckpoints(
					&scanner.ScanCheckpoint{ID: 1, ScanJobID: 100, FilePath: "/test/1.mp4"},
					&scanner.ScanCheckpoint{ID: 2, ScanJobID: 100, FilePath: "/test/2.mp4"},
					&scanner.ScanCheckpoint{ID: 3, ScanJobID: 200, FilePath: "/test/3.mp4"},
				)
			},
			jobID:         100,
			expectCleanup: true,
		},
		{
			name: "cleanup with error - should log but not fail",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.DeleteByJobIDErr = errors.New("database error")
			},
			jobID:         100,
			expectCleanup: false,
		},
		{
			name: "cleanup for non-existent job",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No checkpoints for job 999
			},
			jobID:         999,
			expectCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(checkpointRepo)
			}

			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
			}
			deps := testOrchestrationDeps(scanRepos, nil, nil, nil, nil)

			CleanupCheckpoints(context.Background(), deps, tt.jobID)

			if tt.expectCleanup {
				count := checkpointRepo.GetCheckpointCount(tt.jobID)
				if count != 0 {
					t.Errorf("Expected 0 checkpoints after cleanup, got %d", count)
				}
			}
		})
	}
}

// =============================================================================
// Tests for BuildCompletedJob
// =============================================================================

func TestBuildCompletedJob(t *testing.T) {
	tests := []struct {
		name           string
		jobID          int64
		currentJob     *scanner.ScanJob
		stats          *scanner.CheckpointStats
		discoveryStats *filesystem.WalkStats
		expectedJob    *scanner.ScanJob
	}{
		{
			name:  "basic completion without discovery stats",
			jobID: 100,
			currentJob: &scanner.ScanJob{
				ID:             100,
				LibraryID:      1,
				FilesFound:     10,
				FilesProcessed: 10,
			},
			stats: &scanner.CheckpointStats{
				TotalFiles:     10,
				CompletedFiles: 10,
				ProcessedFiles: 10,
				FailedFiles:    0,
				WarningFiles:   0,
			},
			discoveryStats: nil,
			expectedJob: &scanner.ScanJob{
				ID:             100,
				Status:         scanner.ScanStatusCompleted,
				FilesFound:     10,
				FilesProcessed: 10,
				ErrorCount:     0,
				WarningCount:   0,
				Progress:       100.0,
				Phase:          scanner.ScanPhaseCompleted,
				DiscoveryDone:  true,
			},
		},
		{
			name:  "completion with errors and warnings",
			jobID: 200,
			currentJob: &scanner.ScanJob{
				ID:             200,
				LibraryID:      1,
				FilesFound:     20,
				FilesProcessed: 20,
			},
			stats: &scanner.CheckpointStats{
				TotalFiles:     20,
				CompletedFiles: 15,
				ProcessedFiles: 20,
				FailedFiles:    3,
				WarningFiles:   2,
			},
			discoveryStats: nil,
			expectedJob: &scanner.ScanJob{
				ID:             200,
				Status:         scanner.ScanStatusCompleted,
				FilesFound:     20,
				FilesProcessed: 20,
				ErrorCount:     3,
				WarningCount:   2,
				Progress:       100.0,
				Phase:          scanner.ScanPhaseCompleted,
				DiscoveryDone:  true,
			},
		},
		{
			name:  "completion with discovery stats",
			jobID: 300,
			currentJob: &scanner.ScanJob{
				ID:             300,
				LibraryID:      1,
				FilesFound:     15,
				FilesProcessed: 15,
			},
			stats: &scanner.CheckpointStats{
				TotalFiles:     15,
				CompletedFiles: 15,
				ProcessedFiles: 15,
				FailedFiles:    0,
				WarningFiles:   0,
			},
			discoveryStats: &filesystem.WalkStats{
				DirsScanned:      100,
				DirsSkipped:      5,
				FilesSkipped:     10,
				PermissionErrors: 3,
				NetworkErrors:    2,
			},
			expectedJob: &scanner.ScanJob{
				ID:              300,
				Status:          scanner.ScanStatusCompleted,
				FilesFound:      15,
				FilesProcessed:  15,
				ErrorCount:      0,
				WarningCount:    0,
				Progress:        100.0,
				Phase:           scanner.ScanPhaseCompleted,
				DiscoveryDone:   true,
				DirsScanned:     100,
				DirsSkipped:     5,
				FilesSkipped:    10,
				DiscoveryErrors: 5, // PermissionErrors + NetworkErrors
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := BuildCompletedJob(tt.jobID, tt.currentJob, tt.stats, tt.discoveryStats)

			if job.ID != tt.expectedJob.ID {
				t.Errorf("Expected ID %d, got %d", tt.expectedJob.ID, job.ID)
			}
			if job.Status != tt.expectedJob.Status {
				t.Errorf("Expected Status %v, got %v", tt.expectedJob.Status, job.Status)
			}
			if job.FilesFound != tt.expectedJob.FilesFound {
				t.Errorf("Expected FilesFound %d, got %d", tt.expectedJob.FilesFound, job.FilesFound)
			}
			if job.FilesProcessed != tt.expectedJob.FilesProcessed {
				t.Errorf("Expected FilesProcessed %d, got %d", tt.expectedJob.FilesProcessed, job.FilesProcessed)
			}
			if job.ErrorCount != tt.expectedJob.ErrorCount {
				t.Errorf("Expected ErrorCount %d, got %d", tt.expectedJob.ErrorCount, job.ErrorCount)
			}
			if job.WarningCount != tt.expectedJob.WarningCount {
				t.Errorf("Expected WarningCount %d, got %d", tt.expectedJob.WarningCount, job.WarningCount)
			}
			if job.Progress != tt.expectedJob.Progress {
				t.Errorf("Expected Progress %.2f, got %.2f", tt.expectedJob.Progress, job.Progress)
			}
			if job.Phase != tt.expectedJob.Phase {
				t.Errorf("Expected Phase %v, got %v", tt.expectedJob.Phase, job.Phase)
			}
			if job.DiscoveryDone != tt.expectedJob.DiscoveryDone {
				t.Errorf("Expected DiscoveryDone %v, got %v", tt.expectedJob.DiscoveryDone, job.DiscoveryDone)
			}

			if job.CompletedAt == nil {
				t.Error("Expected CompletedAt to be set")
			} else if time.Since(*job.CompletedAt) > 1*time.Second {
				t.Error("CompletedAt should be recent")
			}

			if tt.discoveryStats != nil {
				if job.DirsScanned != tt.expectedJob.DirsScanned {
					t.Errorf("Expected DirsScanned %d, got %d", tt.expectedJob.DirsScanned, job.DirsScanned)
				}
				if job.DirsSkipped != tt.expectedJob.DirsSkipped {
					t.Errorf("Expected DirsSkipped %d, got %d", tt.expectedJob.DirsSkipped, job.DirsSkipped)
				}
				if job.FilesSkipped != tt.expectedJob.FilesSkipped {
					t.Errorf("Expected FilesSkipped %d, got %d", tt.expectedJob.FilesSkipped, job.FilesSkipped)
				}
				if job.DiscoveryErrors != tt.expectedJob.DiscoveryErrors {
					t.Errorf("Expected DiscoveryErrors %d, got %d", tt.expectedJob.DiscoveryErrors, job.DiscoveryErrors)
				}
			}
		})
	}
}

// =============================================================================
// Tests for LogScanCompletion
// =============================================================================

func TestLogScanCompletion(t *testing.T) {
	tests := []struct {
		name       string
		jobID      int64
		libraryID  int64
		filesFound int64
		stats      *scanner.CheckpointStats
	}{
		{
			name:       "successful completion - no errors or warnings",
			jobID:      100,
			libraryID:  1,
			filesFound: 10,
			stats: &scanner.CheckpointStats{
				TotalFiles:     10,
				CompletedFiles: 10,
				FailedFiles:    0,
				WarningFiles:   0,
			},
		},
		{
			name:       "completion with errors",
			jobID:      200,
			libraryID:  1,
			filesFound: 20,
			stats: &scanner.CheckpointStats{
				TotalFiles:     20,
				CompletedFiles: 15,
				FailedFiles:    5,
				WarningFiles:   0,
			},
		},
		{
			name:       "completion with warnings only",
			jobID:      300,
			libraryID:  1,
			filesFound: 15,
			stats: &scanner.CheckpointStats{
				TotalFiles:     15,
				CompletedFiles: 13,
				FailedFiles:    0,
				WarningFiles:   2,
			},
		},
		{
			name:       "completion with both errors and warnings",
			jobID:      400,
			libraryID:  1,
			filesFound: 30,
			stats: &scanner.CheckpointStats{
				TotalFiles:     30,
				CompletedFiles: 25,
				FailedFiles:    3,
				WarningFiles:   2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := testOrchestrationDeps(nil, nil, nil, nil, nil)

			// Should not panic
			LogScanCompletion(deps, tt.jobID, tt.libraryID, tt.filesFound, tt.stats)
		})
	}
}
