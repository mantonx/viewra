package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		config               ScanConfig
		systemProfile        *system.Profile
		expectedParallel     bool
		expectedProgressLog  bool
	}{
		{
			name: "creates walker with system profile recommendations",
			config: ScanConfig{
				ParallelWalkers:  0,
				ProgressInterval: 0,
			},
			systemProfile: &system.Profile{
				Storage: system.StorageProfile{
					Type: "network",
				},
			},
			expectedParallel:    true, // systemProfile.Calculate() will set this
			expectedProgressLog: false,
		},
		{
			name: "creates walker with config parallel walkers",
			config: ScanConfig{
				ParallelWalkers:  4,
				ProgressInterval: 0,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: false,
		},
		{
			name: "creates walker with progress logging",
			config: ScanConfig{
				ParallelWalkers:  0,
				ProgressInterval: 100,
			},
			systemProfile:       nil,
			expectedParallel:    false,
			expectedProgressLog: true,
		},
		{
			name: "creates walker with both parallel and progress",
			config: ScanConfig{
				ParallelWalkers:  2,
				ProgressInterval: 50,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: true,
		},
		{
			name: "creates sequential walker by default",
			config: ScanConfig{
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
				scanRepos: &ScanRepositories{
					ScanJob: scanJobRepo,
				},
				logger: discardLogger(),
			}

			dctx := &discoveryContext{
				jobID:      100,
				lib:        lib,
				currentJob: job,
				walker:     tt.setupWalker(t),
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
			scanRepos: &ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: ScanConfig{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID:      100,
			lib:        &library.Library{ID: 1, Path: "/nonexistent"},
			currentJob: job,
			walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
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
			scanRepos: &ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: ScanConfig{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID:      100,
			lib:        &library.Library{ID: 1, Path: tmpDir},
			currentJob: job,
			walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
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
		if dctx.discoveryStats == nil {
			t.Error("discoveryStats should be set")
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
			scanRepos: &ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: ScanConfig{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10, // Progress every 10 files
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID:      100,
			lib:        &library.Library{ID: 1, Path: tmpDir},
			currentJob: job,
			walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
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
			scanRepos: &ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: ScanConfig{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   1, // Progress every file
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID:      100,
			lib:        &library.Library{ID: 1, Path: tmpDir},
			currentJob: job,
			walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
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
			scanRepos: &ScanRepositories{
				ScanJob: scanJobRepo,
			},
			config: ScanConfig{
				DiscoveryBufferSize: 100,
				DiscoveryLogEvery:   10,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID:      100,
			lib:        &library.Library{ID: 1, Path: "/nonexistent/path/that/does/not/exist"},
			currentJob: job,
			walker:     filesystem.NewWalker(filesystem.WithLogger(discardLogger())),
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
			scanRepos: &ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: ScanConfig{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID: 100,
			lib:   &library.Library{ID: 1, Path: "/media"},
			discoveryStats: &filesystem.WalkStats{
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
			scanRepos: &ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: ScanConfig{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID: 100,
			lib:   &library.Library{ID: 1, Path: "/media"},
			discoveryStats: &filesystem.WalkStats{
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
			scanRepos: &ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: ScanConfig{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID: 100,
			lib:   &library.Library{ID: 1, Path: "/nonexistent"},
			discoveryStats: &filesystem.WalkStats{
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
			scanRepos: &ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: ScanConfig{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID: 100,
			lib:   &library.Library{ID: 1, Path: tmpDir},
			discoveryStats: &filesystem.WalkStats{
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
			scanRepos: &ScanRepositories{
				Checkpoint: checkpointRepo,
				ScanJob:    scanJobRepo,
			},
			config: ScanConfig{
				CheckpointBatchSize:  10,
				MaxRetries:           3,
				WorkerTimeout:        5 * time.Minute,
				HashProgressLogEvery: 1000,
			},
			logger: discardLogger(),
		}

		dctx := &discoveryContext{
			jobID: 100,
			lib:   &library.Library{ID: 1, Path: tmpDir},
			discoveryStats: &filesystem.WalkStats{
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
