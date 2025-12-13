package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Test wrapper to allow mocking processFileWithCheckpoint
type testCheckpointWorkerUseCase struct {
	*ScanLibraryUseCase
	processFileFunc func(context.Context, *library.Library, *scanner.ScanCheckpoint, *sync.Map) (bool, error)
}

func (t *testCheckpointWorkerUseCase) processFileWithCheckpoint(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (bool, error) {
	if t.processFileFunc != nil {
		return t.processFileFunc(ctx, lib, checkpoint, existingMediaCache)
	}
	return t.ScanLibraryUseCase.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)
}

// Tests for updateCheckpointStatus

func TestScanLibraryUseCase_updateCheckpointStatus(t *testing.T) {
	tests := []struct {
		name      string
		setupRepo func(*mocks.CheckpointRepository)
		checkpoint *scanner.ScanCheckpoint
		status    scanner.CheckpointStatus
		message   string
		category  scanner.ErrorCategory
		action    string
		wantAbort bool
	}{
		{
			name: "successful status update",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        1,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointPending,
			},
			status:    scanner.CheckpointCompleted,
			message:   "",
			category:  "",
			action:    "mark checkpoint as completed",
			wantAbort: false,
		},
		{
			name: "update with error message and category",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        2,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			status:    scanner.CheckpointFailed,
			message:   "ffmpeg error",
			category:  scanner.ErrorCategoryFFmpeg,
			action:    "mark checkpoint as failed",
			wantAbort: false,
		},
		{
			name: "scan job deleted - should abort",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// Return a "foreign key constraint" error (indicates deletion)
				repo.UpdateStatusErr = errors.New("foreign key constraint violated")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        3,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointPending,
			},
			status:    scanner.CheckpointCompleted,
			message:   "",
			category:  "",
			action:    "mark checkpoint as completed",
			wantAbort: true,
		},
		{
			name: "other database error - should not abort",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// Return a generic database error (not deletion-related)
				repo.UpdateStatusErr = errors.New("database is locked")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        4,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointPending,
			},
			status:    scanner.CheckpointCompleted,
			message:   "",
			category:  "",
			action:    "mark checkpoint as completed",
			wantAbort: false,
		},
		{
			name: "update to warning status",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        5,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			status:    scanner.CheckpointWarning,
			message:   "metadata extraction incomplete",
			category:  scanner.ErrorCategoryFFmpeg,
			action:    "mark checkpoint as warning",
			wantAbort: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)

			if tt.setupRepo != nil {
				tt.setupRepo(checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			gotAbort := uc.updateCheckpointStatus(
				context.Background(),
				tt.checkpoint,
				tt.status,
				tt.message,
				tt.category,
				tt.action,
			)

			if gotAbort != tt.wantAbort {
				t.Errorf("updateCheckpointStatus() gotAbort = %v, want %v", gotAbort, tt.wantAbort)
			}
		})
	}
}

// Tests for handleCheckpointSuccess

func TestScanLibraryUseCase_handleCheckpointSuccess(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mocks.CheckpointRepository)
		checkpoint *scanner.ScanCheckpoint
		checkResult func(*testing.T, map[string]bool)
	}{
		{
			name: "successful completion",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        1,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			checkResult: func(t *testing.T, foundFiles map[string]bool) {
				// File should be added to foundFiles
				if !foundFiles["/test/movie.mp4"] {
					t.Error("File should be added to foundFiles map")
				}
			},
		},
		{
			name: "update status fails but continues",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.UpdateStatusErr = errors.New("database error")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        2,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			checkResult: func(t *testing.T, foundFiles map[string]bool) {
				// File should still be added to foundFiles (error logged but processing continues)
				if !foundFiles["/test/movie.mp4"] {
					t.Error("File should be added to foundFiles even if status update fails")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)

			if tt.setupRepo != nil {
				tt.setupRepo(checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			foundFilesMu := &sync.Mutex{}
			foundFiles := make(map[string]bool)

			uc.handleCheckpointSuccess(
				context.Background(),
				tt.checkpoint,
				foundFilesMu,
				foundFiles,
			)

			if tt.checkResult != nil {
				tt.checkResult(t, foundFiles)
			}
		})
	}
}

// Tests for handleCheckpointWarning

func TestScanLibraryUseCase_handleCheckpointWarning(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mocks.CheckpointRepository)
		checkpoint *scanner.ScanCheckpoint
		checkResult func(*testing.T, map[string]bool)
	}{
		{
			name: "successful warning handling",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        1,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			checkResult: func(t *testing.T, foundFiles map[string]bool) {
				// File should be added to foundFiles
				if !foundFiles["/test/movie.mp4"] {
					t.Error("File should be added to foundFiles map")
				}
			},
		},
		{
			name: "update status fails - job deleted",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.UpdateStatusErr = errors.New("foreign key constraint violated")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        2,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			checkResult: func(t *testing.T, foundFiles map[string]bool) {
				// File should NOT be added to foundFiles (aborted early)
				if foundFiles["/test/movie.mp4"] {
					t.Error("File should not be added to foundFiles when job is deleted")
				}
			},
		},
		{
			name: "update status fails - other error",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.UpdateStatusErr = errors.New("database error")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        3,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointProcessing,
			},
			checkResult: func(t *testing.T, foundFiles map[string]bool) {
				// File should still be added to foundFiles (error logged)
				if !foundFiles["/test/movie.mp4"] {
					t.Error("File should be added to foundFiles even if update fails")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)

			if tt.setupRepo != nil {
				tt.setupRepo(checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			foundFilesMu := &sync.Mutex{}
			foundFiles := make(map[string]bool)

			uc.handleCheckpointWarning(
				context.Background(),
				tt.checkpoint,
				foundFilesMu,
				foundFiles,
			)

			if tt.checkResult != nil {
				tt.checkResult(t, foundFiles)
			}
		})
	}
}

// Tests for handleCheckpointError

func TestScanLibraryUseCase_handleCheckpointError(t *testing.T) {
	tests := []struct {
		name       string
		setupRepos func(*mocks.CheckpointRepository, *mocks.ScanStateRepository)
		lib        *library.Library
		checkpoint *scanner.ScanCheckpoint
		err        error
		maxRetries int
	}{
		{
			name: "non-transient error - mark as failed",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				// Pre-populate scan state for SetError
				scanStateRepo.WithStates(&scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/test/movie.mp4",
				})
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         1,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("invalid video format"),
			maxRetries: 3,
		},
		{
			name: "transient error - should retry",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				// No setup needed
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         2,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("database is locked"),
			maxRetries: 3,
		},
		{
			name: "transient error - max retries exceeded",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				// Pre-populate scan state for SetError
				scanStateRepo.WithStates(&scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/test/movie.mp4",
				})
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         3,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 3,
			},
			err:        errors.New("timeout"),
			maxRetries: 3,
		},
		{
			name: "scan job deleted during error handling",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				checkpointRepo.UpdateStatusErr = errors.New("foreign key constraint violated")
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         4,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("processing failed"),
			maxRetries: 3,
		},
		{
			name: "scan state set error fails gracefully",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				// Pre-populate scan state
				scanStateRepo.WithStates(&scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/test/movie.mp4",
				})
				// SetError will fail but should be handled gracefully
				scanStateRepo.SetErrorErr = errors.New("database error")
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         5,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("ffmpeg error"),
			maxRetries: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)
			scanStateRepo := mocks.NewScanStateRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(checkpointRepo, scanStateRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
					ScanState:  scanStateRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Use a context with short timeout to avoid long sleeps in retry logic
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			uc.handleCheckpointError(
				ctx,
				tt.lib,
				tt.checkpoint,
				tt.err,
				tt.maxRetries,
			)
		})
	}
}

// Tests for retryCheckpoint

func TestScanLibraryUseCase_retryCheckpoint(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mocks.CheckpointRepository)
		checkpoint *scanner.ScanCheckpoint
		err        error
		maxRetries int
		checkRepo  func(*testing.T, *mocks.CheckpointRepository, *scanner.ScanCheckpoint)
	}{
		{
			name: "first retry",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				// No error injection
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         1,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("timeout"),
			maxRetries: 3,
			checkRepo: func(t *testing.T, repo *mocks.CheckpointRepository, checkpoint *scanner.ScanCheckpoint) {
				if checkpoint.RetryCount != 1 {
					t.Errorf("RetryCount = %v, want 1", checkpoint.RetryCount)
				}
			},
		},
		{
			name: "update retry count fails - job deleted",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.UpdateRetryCountErr = errors.New("foreign key constraint violated")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         2,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("timeout"),
			maxRetries: 3,
			checkRepo: func(t *testing.T, repo *mocks.CheckpointRepository, checkpoint *scanner.ScanCheckpoint) {
				// Should abort early, retry count incremented in memory
				if checkpoint.RetryCount != 1 {
					t.Errorf("RetryCount = %v, want 1 (incremented in memory)", checkpoint.RetryCount)
				}
			},
		},
		{
			name: "update retry count fails - other error",
			setupRepo: func(repo *mocks.CheckpointRepository) {
				repo.UpdateRetryCountErr = errors.New("database error")
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:         3,
				ScanJobID:  100,
				FilePath:   "/test/movie.mp4",
				Status:     scanner.CheckpointProcessing,
				RetryCount: 0,
			},
			err:        errors.New("timeout"),
			maxRetries: 3,
			checkRepo: func(t *testing.T, repo *mocks.CheckpointRepository, checkpoint *scanner.ScanCheckpoint) {
				// Should log error but continue with retry
				if checkpoint.RetryCount != 1 {
					t.Errorf("RetryCount = %v, want 1", checkpoint.RetryCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)

			if tt.setupRepo != nil {
				tt.setupRepo(checkpointRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Use a context with short timeout to avoid waiting for the full sleep
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Run retryCheckpoint in a goroutine to allow timeout
			done := make(chan struct{})
			go func() {
				uc.retryCheckpoint(ctx, tt.checkpoint, tt.err, tt.maxRetries)
				close(done)
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				// Completed
			case <-time.After(200 * time.Millisecond):
				// Timed out (expected for tests with sleep)
			}

			if tt.checkRepo != nil {
				tt.checkRepo(t, checkpointRepo, tt.checkpoint)
			}
		})
	}
}

// Tests for processCheckpointWorker
// Note: These tests focus on the parts of processCheckpointWorker that can be tested
// without a full coordinator setup (early abort, error handling paths).
// Full integration tests with actual file processing would require coordinator mocking.

func TestScanLibraryUseCase_processCheckpointWorker(t *testing.T) {
	tests := []struct {
		name       string
		setupRepos func(*mocks.CheckpointRepository, *mocks.ScanStateRepository)
		lib        *library.Library
		checkpoint *scanner.ScanCheckpoint
		maxRetries int
		checkResult func(*testing.T, *mocks.CheckpointRepository, map[string]bool)
	}{
		{
			name: "mark as processing fails - job deleted - should abort early",
			setupRepos: func(checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository) {
				// Fail the initial "mark as processing" update
				checkpointRepo.UpdateStatusErr = errors.New("foreign key constraint violated")
			},
			lib: &library.Library{
				ID:   1,
				Name: "Test Library",
				Type: library.LibraryTypeMovies,
			},
			checkpoint: &scanner.ScanCheckpoint{
				ID:        1,
				ScanJobID: 100,
				FilePath:  "/test/movie.mp4",
				Status:    scanner.CheckpointPending,
			},
			maxRetries: 3,
			checkResult: func(t *testing.T, checkpointRepo *mocks.CheckpointRepository, foundFiles map[string]bool) {
				// Should abort early, file not added to foundFiles
				if foundFiles["/test/movie.mp4"] {
					t.Error("File should not be added to foundFiles when job is deleted")
				}
				// Checkpoint should not have progressed to processing
				checkpoint, _ := checkpointRepo.GetByPath(context.Background(), 100, "/test/movie.mp4")
				if checkpoint.Status == scanner.CheckpointProcessing {
					t.Error("Checkpoint should not be marked as processing when update fails due to job deletion")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			checkpointRepo.WithCheckpoints(tt.checkpoint)
			scanStateRepo := mocks.NewScanStateRepository(t)

			if tt.setupRepos != nil {
				tt.setupRepos(checkpointRepo, scanStateRepo)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					Checkpoint: checkpointRepo,
					ScanState:  scanStateRepo,
				},
				config: DefaultScanConfig(),
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			foundFilesMu := &sync.Mutex{}
			foundFiles := make(map[string]bool)
			existingMediaCache := &sync.Map{}

			// Note: Full testing of processCheckpointWorker requires coordinator setup.
			// This test focuses on the early abort path which doesn't require processFileWithCheckpoint.
			uc.processCheckpointWorker(
				context.Background(),
				tt.lib,
				tt.checkpoint,
				tt.maxRetries,
				foundFilesMu,
				foundFiles,
				existingMediaCache,
			)

			if tt.checkResult != nil {
				tt.checkResult(t, checkpointRepo, foundFiles)
			}
		})
	}
}
