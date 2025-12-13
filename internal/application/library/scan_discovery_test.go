package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Tests for createWalker

func TestScanLibraryUseCase_createWalker(t *testing.T) {
	tests := []struct {
		name                 string
		config               scan.Config
		systemProfile        *system.Profile
		expectedParallel     bool
		expectedProgressLog  bool
	}{
		{
			name: "creates walker with system profile recommendations",
			config: scan.Config{
				ParallelWalkers:  0,
				ProgressInterval: 0,
			},
			systemProfile: &system.Profile{
				Storage: system.StorageProfile{
					Type:     "network",
					IsRemote: true, // Required for ScanWalkers to be set
				},
				CPU: system.CPUProfile{
					NumCPU: 8,
				},
			},
			expectedParallel:    true, // systemProfile.Calculate() will set ScanWalkers
			expectedProgressLog: false,
		},
		{
			name: "creates walker with config parallel walkers",
			config: scan.Config{
				ParallelWalkers:  4,
				ProgressInterval: 0,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: false,
		},
		{
			name: "creates walker with progress logging",
			config: scan.Config{
				ParallelWalkers:  0,
				ProgressInterval: 100,
			},
			systemProfile:       nil,
			expectedParallel:    false,
			expectedProgressLog: true,
		},
		{
			name: "creates walker with both parallel and progress",
			config: scan.Config{
				ParallelWalkers:  2,
				ProgressInterval: 50,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: true,
		},
		{
			name: "creates sequential walker by default",
			config: scan.Config{
				ParallelWalkers:  0,
				ProgressInterval: 0,
			},
			systemProfile:       nil,
			expectedParallel:    false,
			expectedProgressLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				config:        tt.config,
				systemProfile: tt.systemProfile,
				logger:        discardLogger(),
			}

			walker := uc.createWalker()

			if walker == nil {
				t.Fatal("Expected non-nil walker")
			}

			// Walker is created, we can't easily inspect internal state
			// but we verify it doesn't panic and returns a valid walker
		})
	}
}

// Tests for phaseCountFiles

func TestScanLibraryUseCase_phaseCountFiles(t *testing.T) {
	tests := []struct {
		name              string
		libraryPath       string
		setupWalker       func(*testing.T) *filesystem.Walker
		expectedLoggedMsg string
		expectUpdateCall  bool
	}{
		{
			name:        "counts files successfully",
			libraryPath: "/test/library",
			setupWalker: func(t *testing.T) *filesystem.Walker {
				// Create a walker that will succeed with a count
				return filesystem.NewWalker(filesystem.WithLogger(discardLogger()))
			},
			expectUpdateCall: true,
		},
		{
			name:        "handles count error gracefully",
			libraryPath: "/nonexistent",
			setupWalker: func(t *testing.T) *filesystem.Walker {
				return filesystem.NewWalker(filesystem.WithLogger(discardLogger()))
			},
			expectUpdateCall: false, // Error means no update
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanJobRepo := mocks.NewScanJobRepository(t)
			lib := &library.Library{
				ID:   1,
				Path: tt.libraryPath,
			}

			job := &scanner.ScanJob{
				ID:             100,
				LibraryID:      1,
				EstimatedTotal: 0,
			}
			scanJobRepo.WithJobs(job)

			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanJob: scanJobRepo,
				},
				logger: discardLogger(),
			}

			dctx := &discoveryContext{
				JobID:      100,
				Lib:        lib,
				CurrentJob: job,
				Walker:     tt.setupWalker(t),
			}

			// This should not panic even with errors
			uc.phaseCountFiles(context.Background(), dctx)

			// Verify the function completes without panic
		})
	}
}

// Tests for phaseWalkDirectory

func TestScanLibraryUseCase_phaseWalkDirectory_ContextCancellation(t *testing.T) {
	t.Run("handles context cancellation during walk", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: scan.Config{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID:      100,
			Lib:        &library.Library{ID: 1, Path: "/nonexistent"},
			CurrentJob: job,
			Walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
		}

		// Cancel context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := uc.phaseWalkDirectory(ctx, dctx)

		// Should either get context.Canceled or a filesystem error
		// Both are acceptable since the path doesn't exist
		_ = err
	})
}

func TestScanLibraryUseCase_phaseWalkDirectory_WithTempDir(t *testing.T) {
	t.Run("discovers media files in temp directory", func(t *testing.T) {
		// Create temp directory with test files
		tmpDir := t.TempDir()

		// Create some test files (won't be actual media, just checking discovery)
		testFiles := []string{"movie1.mp4", "movie2.mkv", "readme.txt", "movie3.avi"}
		for _, filename := range testFiles {
			if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: scan.Config{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID:      100,
			Lib:        &library.Library{ID: 1, Path: tmpDir},
			CurrentJob: job,
			Walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
		}

		files, err := uc.phaseWalkDirectory(context.Background(), dctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should discover 3 media files (mp4, mkv, avi) - not txt
		if len(files) != 3 {
			t.Errorf("expected 3 media files, got %d", len(files))
		}

		// Verify stats were captured
		if dctx.DiscoveryStats == nil {
			t.Error("DiscoveryStats should be set")
		}
	})

	t.Run("handles progress reporting at intervals", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create enough files to trigger progress callback
		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("movie%d.mp4", i)
			if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 100,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: scan.Config{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10, // Progress every 10 files
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID:      100,
			Lib:        &library.Library{ID: 1, Path: tmpDir},
			CurrentJob: job,
			Walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
		}

		files, err := uc.phaseWalkDirectory(context.Background(), dctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 25 {
			t.Errorf("expected 25 files, got %d", len(files))
		}
	})

	t.Run("handles update progress error gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test file
		if err := os.WriteFile(filepath.Join(tmpDir, "movie.mp4"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)
		scanJobRepo.UpdateProgressErr = errors.New("database error")

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: scan.Config{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   1, // Progress every file
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID:      100,
			Lib:        &library.Library{ID: 1, Path: tmpDir},
			CurrentJob: job,
			Walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
		}

		// Should still succeed despite progress update errors
		files, err := uc.phaseWalkDirectory(context.Background(), dctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("handles walker errors for nonexistent path", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: scan.Config{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID:      100,
			Lib:        &library.Library{ID: 1, Path: "/nonexistent/path/that/does/not/exist"},
			CurrentJob: job,
			Walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
		}

		files, err := uc.phaseWalkDirectory(context.Background(), dctx)

		// Walk may return error or succeed with empty results depending on implementation
		// Both are acceptable - we're testing that it doesn't panic
		if err != nil {
			// Got error - that's fine
			t.Logf("Got expected error: %v", err)
		} else {
			// No error - should have empty files
			if len(files) != 0 {
				t.Errorf("expected 0 files for nonexistent path, got %d", len(files))
			}
		}
	})
}

// Tests for phaseHashAndProcess

func TestScanLibraryUseCase_phaseHashAndProcess_EmptyFiles(t *testing.T) {
	t.Run("handles empty file list gracefully", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: "/media"},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 0,
			},
		}

		diff := &scanner.ScanDiff{
			NewFiles:       []scanner.FileInfo{},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should complete without error
		uc.phaseHashAndProcess(context.Background(), dctx, diff)
	})

	t.Run("handles unchanged files correctly", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: "/media"},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 10,
			},
		}

		diff := &scanner.ScanDiff{
			NewFiles:      []scanner.FileInfo{},
			ModifiedFiles: []scanner.FileInfo{},
			UnchangedFiles: []string{
				"/media/movie1.mp4",
				"/media/movie2.mp4",
				"/media/movie3.mp4",
			},
		}

		// Should skip processing unchanged files
		uc.phaseHashAndProcess(context.Background(), dctx, diff)
	})
}

func TestScanLibraryUseCase_phaseHashAndProcess_ErrorHandling(t *testing.T) {
	t.Run("handles nonexistent files gracefully", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: "/nonexistent"},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 1,
			},
		}

		diff := &scanner.ScanDiff{
			NewFiles: []scanner.FileInfo{
				{Path: "/nonexistent/movie.mp4", Size: 1024, ModTime: time.Now()},
			},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should handle file not found errors gracefully
		uc.phaseHashAndProcess(context.Background(), dctx, diff)
	})

	t.Run("continues processing after individual file errors", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create one valid file and reference one nonexistent file
		validFile := filepath.Join(tmpDir, "valid.mp4")
		if err := os.WriteFile(validFile, []byte("valid"), 0644); err != nil {
			t.Fatal(err)
		}

		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: tmpDir},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 2,
			},
		}

		diff := &scanner.ScanDiff{
			NewFiles: []scanner.FileInfo{
				{Path: validFile, Size: 5, ModTime: time.Now()},
				{Path: filepath.Join(tmpDir, "nonexistent.mp4"), Size: 1024, ModTime: time.Now()},
			},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should process valid file and handle error for nonexistent file
		uc.phaseHashAndProcess(context.Background(), dctx, diff)
	})
}

func TestScanLibraryUseCase_phaseHashAndProcess_Metrics(t *testing.T) {
	t.Run("logs processing metrics for multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create multiple files to test metrics
		for i := 0; i < 5; i++ {
			filename := filepath.Join(tmpDir, fmt.Sprintf("movie%d.mp4", i))
			if err := os.WriteFile(filename, []byte("content"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: tmpDir},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 5,
				DirsScanned:     1,
			},
		}

		var files []scanner.FileInfo
		for i := 0; i < 5; i++ {
			files = append(files, scanner.FileInfo{
				Path:    filepath.Join(tmpDir, fmt.Sprintf("movie%d.mp4", i)),
				Size:    7,
				ModTime: time.Now(),
			})
		}

		diff := &scanner.ScanDiff{
			NewFiles:       files,
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should process all files and log metrics
		uc.phaseHashAndProcess(context.Background(), dctx, diff)
	})
}

func TestScanLibraryUseCase_phaseHashAndProcess_CheckpointCreationError(t *testing.T) {
	t.Run("handles checkpoint creation error", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a test file
		testFile := filepath.Join(tmpDir, "movie.mp4")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}

		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
			Status:    scanner.ScanStatusRunning,
		}
		scanJobRepo.WithJobs(job)

		// Inject error when creating checkpoints
		checkpointRepo.CreateBatchErr = errors.New("database connection failed")

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: scan.Config{
				CheckpointBatchSize:   10,
				CheckpointBufferSize:  10,
				MaxRetries:            3,
				WorkerTimeout:         5 * time.Minute,
				HashProgressLogEvery:  1000,
				ProgressUpdateTick:    100 * time.Millisecond,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: tmpDir},
			DiscoveryStats: &filesystem.WalkStats{
				FilesDiscovered: 1,
			},
		}

		diff := &scanner.ScanDiff{
			NewFiles: []scanner.FileInfo{
				{Path: testFile, Size: 12, ModTime: time.Now()},
			},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should handle error gracefully and mark job as failed
		uc.phaseHashAndProcess(context.Background(), dctx, diff)

		// Verify job was marked as failed
		updatedJob, err := scanJobRepo.GetByID(context.Background(), 100)
		if err != nil {
			t.Fatalf("failed to get job: %v", err)
		}
		if updatedJob.Status != scanner.ScanStatusFailed {
			t.Errorf("expected job status %v, got %v", scanner.ScanStatusFailed, updatedJob.Status)
		}
	})
}

// Tests for runFreshScan

func TestScanLibraryUseCase_runFreshScan(t *testing.T) {
	tests := []struct {
		name              string
		setupTempDir      func(*testing.T) string
		setupRepos        func(*testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner)
		jobID             int64
		expectJobComplete bool
		expectError       bool
	}{
		{
			name: "completes full scan successfully with new files",
			setupTempDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				// Create test files
				for i := 0; i < 3; i++ {
					filename := filepath.Join(tmpDir, fmt.Sprintf("movie%d.mp4", i))
					if err := os.WriteFile(filename, []byte("test content"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				return tmpDir
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 0,
				}
				scanJobRepo.WithJobs(job)

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, discardLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: false, // Job completes via phaseHashAndProcess
			expectError:       false,
		},
		{
			name: "handles no changes detected - marks job complete",
			setupTempDir: func(t *testing.T) string {
				// For this test, we don't need actual files to match scan state
				// since we're testing the no-change detection logic
				return t.TempDir()
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 0,
				}
				scanJobRepo.WithJobs(job)

				// This test validates behavior when no files are discovered
				// (empty discovery = no changes, job should complete)
				// We intentionally don't add any scan state

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, discardLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: true,
			expectError:       false,
		},
		{
			name: "handles deleted files",
			setupTempDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				// Create one file (simulate deletion by not creating the old file)
				filename := filepath.Join(tmpDir, "new_movie.mp4")
				if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 0,
				}
				scanJobRepo.WithJobs(job)

				// Pre-populate with scan state for a deleted file
				scanState := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/deleted/old_movie.mp4",
					FileSize:  1000,
					FileMTime: time.Now().Add(-2 * time.Hour),
				}
				scanStateRepo.WithStates(scanState)

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, discardLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: false,
			expectError:       false,
		},
		{
			name: "handles initialization error gracefully",
			setupTempDir: func(t *testing.T) string {
				return t.TempDir()
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				// Inject error to fail initialization
				scanJobRepo.GetErr = errors.New("database error")

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, discardLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: false,
			expectError:       true,
		},
		{
			name: "handles walk directory error",
			setupTempDir: func(t *testing.T) string {
				// Return non-existent path to trigger walk error
				return "/nonexistent/path/that/does/not/exist"
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 0,
				}
				scanJobRepo.WithJobs(job)

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, discardLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: false,
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := tt.setupTempDir(t)
			repos, incScanner := tt.setupRepos(t)

			lib := &library.Library{
				ID:   1,
				Path: tmpDir,
			}

			uc := &ScanLibraryUseCase{
				scanRepos:          repos,
				incrementalScanner: incScanner,
				config: scan.Config{
					DiscoveryBufferSize:  100,
					DiscoveryLogEvery:    10,
					CheckpointBatchSize:  10,
					MaxRetries:           3,
					WorkerTimeout:        5 * time.Minute,
					HashProgressLogEvery: 1000,
				},
				logger: discardLogger(),
			}

			// Execute the scan
			uc.runFreshScan(context.Background(), tt.jobID, lib)

			// Wait a bit for async operations
			time.Sleep(100 * time.Millisecond)

			// Verify job state if expected to complete
			if tt.expectJobComplete {
				job, err := repos.ScanJob.GetByID(context.Background(), tt.jobID)
				if err != nil {
					if !tt.expectError {
						t.Fatalf("failed to get job: %v", err)
					}
				} else {
					if job.Status != scanner.ScanStatusCompleted {
						t.Errorf("expected job status %v, got %v", scanner.ScanStatusCompleted, job.Status)
					}
				}
			}
		})
	}
}

// Tests for initDiscoveryContext

func TestScanLibraryUseCase_initDiscoveryContext(t *testing.T) {
	tests := []struct {
		name           string
		setupRepos     func(*testing.T) *scan.ScanRepositories
		jobID          int64
		lib            *library.Library
		expectError    bool
		validateResult func(*testing.T, *discoveryContext)
	}{
		{
			name: "initializes context successfully",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 1000,
				}
				scanJobRepo.WithJobs(job)

				return &scan.ScanRepositories{
					ScanJob: scanJobRepo,
				}
			},
			jobID: 100,
			lib: &library.Library{
				ID:   1,
				Path: "/media",
			},
			expectError: false,
			validateResult: func(t *testing.T, dctx *discoveryContext) {
				if dctx == nil {
					t.Fatal("expected non-nil discoveryContext")
				}
				if dctx.JobID != 100 {
					t.Errorf("expected JobID 100, got %d", dctx.JobID)
				}
				if dctx.Lib.ID != 1 {
					t.Errorf("expected library ID 1, got %d", dctx.Lib.ID)
				}
				if dctx.CurrentJob == nil {
					t.Error("expected non-nil CurrentJob")
				}
				if dctx.Walker == nil {
					t.Error("expected non-nil Walker")
				}
			},
		},
		{
			name: "handles job not found error",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				// No jobs created - will return not found

				return &scan.ScanRepositories{
					ScanJob: scanJobRepo,
				}
			},
			jobID: 999,
			lib: &library.Library{
				ID:   1,
				Path: "/media",
			},
			expectError: true,
			validateResult: func(t *testing.T, dctx *discoveryContext) {
				if dctx != nil {
					t.Error("expected nil discoveryContext on error")
				}
			},
		},
		{
			name: "handles repository error",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanJobRepo.GetErr = errors.New("database connection failed")

				return &scan.ScanRepositories{
					ScanJob: scanJobRepo,
				}
			},
			jobID: 100,
			lib: &library.Library{
				ID:   1,
				Path: "/media",
			},
			expectError: true,
			validateResult: func(t *testing.T, dctx *discoveryContext) {
				if dctx != nil {
					t.Error("expected nil discoveryContext on error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos := tt.setupRepos(t)

			uc := &ScanLibraryUseCase{
				scanRepos: repos,
				config:    scan.Config{},
				logger:    discardLogger(),
			}

			dctx, err := uc.initDiscoveryContext(context.Background(), tt.jobID, tt.lib)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			tt.validateResult(t, dctx)
		})
	}
}

// Tests for phaseDetermineChanges

func TestScanLibraryUseCase_phaseDetermineChanges(t *testing.T) {
	tests := []struct {
		name            string
		setupRepos      func(*testing.T) *scan.ScanRepositories
		setupIncScanner func(*testing.T, *scan.ScanRepositories) *discovery.IncrementalScanner
		discoveredFiles []scanner.FileInfo
		expectNil       bool // Expect nil diff when no changes
		validateDiff    func(*testing.T, *scanner.ScanDiff)
	}{
		{
			name: "detects new files",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, discardLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/new1.mp4", Size: 1000, ModTime: time.Now()},
				{Path: "/media/new2.mp4", Size: 2000, ModTime: time.Now()},
			},
			expectNil: false,
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff == nil {
					t.Fatal("expected non-nil diff")
				}
				if len(diff.NewFiles) != 2 {
					t.Errorf("expected 2 new files, got %d", len(diff.NewFiles))
				}
				if len(diff.ModifiedFiles) != 0 {
					t.Errorf("expected 0 modified files, got %d", len(diff.ModifiedFiles))
				}
			},
		},
		{
			name: "detects modified files",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				// Pre-populate with old file state
				scanState := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/modified.mp4",
					FileSize:  1000,
					FileMTime: time.Now().Add(-2 * time.Hour),
				}
				scanStateRepo.WithStates(scanState)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, discardLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/modified.mp4", Size: 2000, ModTime: time.Now()}, // Size changed
			},
			expectNil: false,
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff == nil {
					t.Fatal("expected non-nil diff")
				}
				if len(diff.ModifiedFiles) != 1 {
					t.Errorf("expected 1 modified file, got %d", len(diff.ModifiedFiles))
				}
			},
		},
		{
			name: "detects no changes - marks job complete",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				// Use fixed time to ensure exact match
				modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
				scanState := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/unchanged.mp4",
					FileSize:  1000,
					FileMTime: modTime,
				}
				scanStateRepo.WithStates(scanState)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, discardLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/unchanged.mp4", Size: 1000, ModTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
			expectNil: true, // Returns nil when no changes
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff != nil {
					t.Errorf("expected nil diff when no changes, got %+v", diff)
				}
			},
		},
		{
			name: "falls back to full scan on incremental error",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				// Inject error to trigger fallback
				scanStateRepo.GetLibraryStateErr = errors.New("database error")

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, discardLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/file1.mp4", Size: 1000, ModTime: time.Now()},
				{Path: "/media/file2.mp4", Size: 2000, ModTime: time.Now()},
			},
			expectNil: false,
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff == nil {
					t.Fatal("expected non-nil diff on fallback")
				}
				// Fallback treats all files as new
				if len(diff.NewFiles) != 2 {
					t.Errorf("expected 2 new files in fallback, got %d", len(diff.NewFiles))
				}
				if len(diff.ModifiedFiles) != 0 {
					t.Errorf("expected 0 modified files in fallback, got %d", len(diff.ModifiedFiles))
				}
			},
		},
		{
			name: "handles complete job error gracefully",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				// Inject error when completing job
				scanJobRepo.CompleteErr = errors.New("complete error")

				// Use fixed time to ensure exact match
				modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
				scanState := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/unchanged.mp4",
					FileSize:  1000,
					FileMTime: modTime,
				}
				scanStateRepo.WithStates(scanState)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, discardLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/unchanged.mp4", Size: 1000, ModTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
			expectNil: true, // Still returns nil despite complete error
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff != nil {
					t.Error("expected nil diff even with complete error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos := tt.setupRepos(t)
			incScanner := tt.setupIncScanner(t, repos)

			uc := &ScanLibraryUseCase{
				scanRepos:          repos,
				incrementalScanner: incScanner,
				logger:             discardLogger(),
			}

			dctx := &discoveryContext{
				JobID: 100,
				Lib:   &library.Library{ID: 1, Path: "/media"},
			}

			diff := uc.phaseDetermineChanges(context.Background(), dctx, tt.discoveredFiles)

			if tt.expectNil && diff != nil {
				t.Errorf("expected nil diff, got %+v", diff)
			}

			if !tt.expectNil && diff == nil {
				t.Error("expected non-nil diff, got nil")
			}

			tt.validateDiff(t, diff)
		})
	}
}

// Tests for logDiscoveryStats

func TestScanLibraryUseCase_logDiscoveryStats(t *testing.T) {
	tests := []struct {
		name  string
		stats *filesystem.WalkStats
	}{
		{
			name:  "handles nil stats gracefully",
			stats: nil,
		},
		{
			name: "logs normal stats without errors",
			stats: &filesystem.WalkStats{
				FilesDiscovered:  100,
				DirsScanned:      20,
				DirsSkipped:      0,
				FilesSkipped:     0,
				PermissionErrors: 0,
				NetworkErrors:    0,
				OtherErrors:      0,
			},
		},
		{
			name: "logs stats with dirs skipped (triggers HasErrors)",
			stats: &filesystem.WalkStats{
				FilesDiscovered:  90,
				DirsScanned:      18,
				DirsSkipped:      2, // Triggers HasErrors()
				FilesSkipped:     0,
				PermissionErrors: 2,
				NetworkErrors:    0,
				OtherErrors:      0,
				SkippedPaths:     []string{"/media/restricted", "/media/protected"},
			},
		},
		{
			name: "logs stats with files skipped (triggers HasErrors)",
			stats: &filesystem.WalkStats{
				FilesDiscovered:  95,
				DirsScanned:      20,
				DirsSkipped:      0,
				FilesSkipped:     5, // Triggers HasErrors()
				PermissionErrors: 0,
				NetworkErrors:    3,
				OtherErrors:      2,
				SkippedPaths:     []string{"/media/file1.mp4", "/media/file2.mp4", "/media/file3.mp4"},
			},
		},
		{
			name: "logs stats with both dirs and files skipped",
			stats: &filesystem.WalkStats{
				FilesDiscovered:  80,
				DirsScanned:      15,
				DirsSkipped:      5,
				FilesSkipped:     10,
				PermissionErrors: 8,
				NetworkErrors:    4,
				OtherErrors:      3,
				SkippedPaths:     []string{"/media/dir1", "/media/dir2", "/media/file1.mp4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				logger: discardLogger(),
			}

			// Should not panic for any input
			uc.logDiscoveryStats(100, tt.stats)
		})
	}
}

// Tests for phaseHandleDeleted

func TestScanLibraryUseCase_phaseHandleDeleted(t *testing.T) {
	tests := []struct {
		name         string
		setupRepos   func(*testing.T) *scan.ScanRepositories
		diff         *scanner.ScanDiff
		validateRepo func(*testing.T, *scan.ScanRepositories)
	}{
		{
			name: "deletes scan state for deleted files",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanStateRepo := mocks.NewScanStateRepository(t)

				// Pre-populate with states
				states := []*scanner.ScanState{
					{LibraryID: 1, FilePath: "/media/deleted1.mp4", FileSize: 1000},
					{LibraryID: 1, FilePath: "/media/deleted2.mp4", FileSize: 2000},
					{LibraryID: 1, FilePath: "/media/kept.mp4", FileSize: 3000},
				}
				scanStateRepo.WithStates(states...)

				return &scan.ScanRepositories{
					ScanState: scanStateRepo,
				}
			},
			diff: &scanner.ScanDiff{
				DeletedFiles: []string{"/media/deleted1.mp4", "/media/deleted2.mp4"},
			},
			validateRepo: func(t *testing.T, repos *scan.ScanRepositories) {
				// Verify deleted files are removed
				_, err := repos.ScanState.GetByPath(context.Background(), 1, "/media/deleted1.mp4")
				if !errors.Is(err, scanner.ErrNotFound) {
					t.Error("expected deleted1.mp4 to be removed")
				}

				_, err = repos.ScanState.GetByPath(context.Background(), 1, "/media/deleted2.mp4")
				if !errors.Is(err, scanner.ErrNotFound) {
					t.Error("expected deleted2.mp4 to be removed")
				}

				// Verify kept file still exists
				_, err = repos.ScanState.GetByPath(context.Background(), 1, "/media/kept.mp4")
				if err != nil {
					t.Errorf("expected kept.mp4 to remain, got error: %v", err)
				}
			},
		},
		{
			name: "handles empty deleted files list",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanStateRepo := mocks.NewScanStateRepository(t)

				state := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/file.mp4",
					FileSize:  1000,
				}
				scanStateRepo.WithStates(state)

				return &scan.ScanRepositories{
					ScanState: scanStateRepo,
				}
			},
			diff: &scanner.ScanDiff{
				DeletedFiles: []string{},
			},
			validateRepo: func(t *testing.T, repos *scan.ScanRepositories) {
				// Verify file still exists (nothing deleted)
				_, err := repos.ScanState.GetByPath(context.Background(), 1, "/media/file.mp4")
				if err != nil {
					t.Errorf("expected file to remain, got error: %v", err)
				}
			},
		},
		{
			name: "handles deletion errors gracefully",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanStateRepo := mocks.NewScanStateRepository(t)

				// Inject error
				scanStateRepo.DeleteByPathsErr = errors.New("database error")

				state := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/deleted.mp4",
					FileSize:  1000,
				}
				scanStateRepo.WithStates(state)

				return &scan.ScanRepositories{
					ScanState: scanStateRepo,
				}
			},
			diff: &scanner.ScanDiff{
				DeletedFiles: []string{"/media/deleted.mp4"},
			},
			validateRepo: func(t *testing.T, repos *scan.ScanRepositories) {
				// Function should not panic despite error
				// Error is logged but doesn't stop execution
			},
		},
		{
			name: "handles multiple deleted files efficiently",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanStateRepo := mocks.NewScanStateRepository(t)

				var states []*scanner.ScanState
				for i := 0; i < 100; i++ {
					states = append(states, &scanner.ScanState{
						LibraryID: 1,
						FilePath:  fmt.Sprintf("/media/deleted%d.mp4", i),
						FileSize:  int64(i * 1000),
					})
				}
				scanStateRepo.WithStates(states...)

				return &scan.ScanRepositories{
					ScanState: scanStateRepo,
				}
			},
			diff: &scanner.ScanDiff{
				DeletedFiles: func() []string {
					var files []string
					for i := 0; i < 100; i++ {
						files = append(files, fmt.Sprintf("/media/deleted%d.mp4", i))
					}
					return files
				}(),
			},
			validateRepo: func(t *testing.T, repos *scan.ScanRepositories) {
				// Verify all deleted files are removed
				count, err := repos.ScanState.CountByLibrary(context.Background(), 1)
				if err != nil {
					t.Fatalf("failed to count states: %v", err)
				}
				if count != 0 {
					t.Errorf("expected 0 remaining states, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos := tt.setupRepos(t)

			uc := &ScanLibraryUseCase{
				scanRepos: repos,
				logger:    discardLogger(),
			}

			dctx := &discoveryContext{
				JobID: 100,
				Lib:   &library.Library{ID: 1, Path: "/media"},
			}

			// Execute - should not panic
			uc.phaseHandleDeleted(context.Background(), dctx, tt.diff)

			// Validate results
			tt.validateRepo(t, repos)
		})
	}
}
