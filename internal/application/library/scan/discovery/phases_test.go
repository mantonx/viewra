package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateWalker(t *testing.T) {
	tests := []struct {
		name                string
		config              *scan.Config
		systemProfile       *system.Profile
		expectedParallel    bool
		expectedProgressLog bool
	}{
		{
			name: "creates walker with system profile recommendations",
			config: &scan.Config{
				ParallelWalkers:  0,
				ProgressInterval: 0,
			},
			systemProfile: &system.Profile{
				Storage: system.StorageProfile{
					Type:     "network",
					IsRemote: true,
				},
				CPU: system.CPUProfile{
					NumCPU: 8,
				},
			},
			expectedParallel:    true,
			expectedProgressLog: false,
		},
		{
			name: "creates walker with config parallel walkers",
			config: &scan.Config{
				ParallelWalkers:  4,
				ProgressInterval: 0,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: false,
		},
		{
			name: "creates walker with progress logging",
			config: &scan.Config{
				ParallelWalkers:  0,
				ProgressInterval: 100,
			},
			systemProfile:       nil,
			expectedParallel:    false,
			expectedProgressLog: true,
		},
		{
			name: "creates walker with both parallel and progress",
			config: &scan.Config{
				ParallelWalkers:  2,
				ProgressInterval: 50,
			},
			systemProfile:       nil,
			expectedParallel:    true,
			expectedProgressLog: true,
		},
		{
			name: "creates sequential walker by default",
			config: &scan.Config{
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
			deps := &Deps{
				Config:        tt.config,
				SystemProfile: tt.systemProfile,
				Logger:        testLogger(),
			}

			walker := CreateWalker(deps)

			if walker == nil {
				t.Fatal("Expected non-nil walker")
			}
		})
	}
}

func TestPhaseCountFiles(t *testing.T) {
	tests := []struct {
		name          string
		setupDir      func(*testing.T) string
		expectedCount int64
	}{
		{
			name: "counts media files successfully",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createFileInDir(t, dir, "movie1.mp4")
				createFileInDir(t, dir, "movie2.mkv")
				createFileInDir(t, dir, "readme.txt") // Not a media file
				// Create a subdirectory with more files
				subdir := dir + "/subdir"
				if err := os.Mkdir(subdir, 0755); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				createFileInDir(t, subdir, "episode.mp4")
				return dir
			},
			expectedCount: 3,
		},
		{
			name: "counts zero files in empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectedCount: 0,
		},
		{
			name: "handles count error gracefully",
			setupDir: func(t *testing.T) string {
				return "/nonexistent/directory/12345"
			},
			expectedCount: 0,
		},
		{
			name: "filters non-media files correctly",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createFileInDir(t, dir, "video.mp4")
				createFileInDir(t, dir, "video.avi") // Not in media list
				createFileInDir(t, dir, "document.pdf")
				createFileInDir(t, dir, "image.jpg")
				return dir
			},
			expectedCount: 1, // Only .mp4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirPath := tt.setupDir(t)

			scanJobRepo := mocks.NewScanJobRepository(t)
			lib := &library.Library{
				ID:   1,
				Path: dirPath,
			}

			job := &scanner.ScanJob{
				ID:             100,
				LibraryID:      1,
				EstimatedTotal: 0,
			}
			scanJobRepo.WithJobs(job)

			scanRepos := &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			}

			deps := &Deps{
				ScanRepos: scanRepos,
				Logger:    testLogger(),
				IsMediaFile: func(ext string) bool {
					return ext == ".mp4" || ext == ".mkv"
				},
			}

			walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))
			dctx := &Context{
				JobID:      100,
				Lib:        lib,
				CurrentJob: job,
				Walker:     walker,
			}

			count := PhaseCountFiles(context.Background(), dctx, deps)

			if count != tt.expectedCount {
				t.Errorf("expected count of %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

func TestPhaseCountFiles_WithUpdateProgressError(t *testing.T) {
	// Test the error path when UpdateProgress fails
	dir := t.TempDir()
	createFileInDir(t, dir, "movie.mp4")

	scanJobRepo := mocks.NewScanJobRepository(t)
	scanJobRepo.UpdateProgressErr = errors.New("database error")

	lib := &library.Library{
		ID:   1,
		Path: dir,
	}

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob: scanJobRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
		IsMediaFile: func(ext string) bool {
			return ext == ".mp4"
		},
	}

	walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))
	dctx := &Context{
		JobID:      100,
		Lib:        lib,
		CurrentJob: job,
		Walker:     walker,
	}

	// Should not panic even when UpdateProgress fails
	count := PhaseCountFiles(context.Background(), dctx, deps)

	// Count should still succeed
	if count != 1 {
		t.Errorf("expected count of 1, got %d", count)
	}
}

func TestPhaseCountFiles_WithContext(t *testing.T) {
	// Test that respects context
	dir := t.TempDir()
	createFileInDir(t, dir, "movie.mp4")

	scanJobRepo := mocks.NewScanJobRepository(t)
	lib := &library.Library{
		ID:   1,
		Path: dir,
	}

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob: scanJobRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
		IsMediaFile: func(ext string) bool {
			return ext == ".mp4"
		},
	}

	walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))
	dctx := &Context{
		JobID:      100,
		Lib:        lib,
		CurrentJob: job,
		Walker:     walker,
	}

	// Use regular context (not cancelled)
	ctx := context.Background()
	count := PhaseCountFiles(ctx, dctx, deps)

	if count != 1 {
		t.Errorf("expected count of 1, got %d", count)
	}
}

func TestPhaseCountFiles_CallbackFiltering(t *testing.T) {
	// Explicit test to ensure callback properly filters directories and non-media files
	dir := t.TempDir()

	// Create nested directory structure
	subdir1 := dir + "/movies"
	subdir2 := dir + "/shows"
	if err := os.Mkdir(subdir1, 0755); err != nil {
		t.Fatalf("failed to create movies dir: %v", err)
	}
	if err := os.Mkdir(subdir2, 0755); err != nil {
		t.Fatalf("failed to create shows dir: %v", err)
	}

	// Create media files
	createFileInDir(t, dir, "root.mp4")
	createFileInDir(t, subdir1, "action.mkv")
	createFileInDir(t, subdir2, "comedy.mp4")

	// Create non-media files that should be filtered out
	createFileInDir(t, dir, "readme.txt")
	createFileInDir(t, subdir1, "poster.jpg")
	createFileInDir(t, subdir2, "subtitles.srt")

	scanJobRepo := mocks.NewScanJobRepository(t)
	lib := &library.Library{
		ID:   1,
		Path: dir,
	}

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob: scanJobRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
		IsMediaFile: func(ext string) bool {
			// Only count .mp4 and .mkv
			return ext == ".mp4" || ext == ".mkv"
		},
	}

	walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))
	dctx := &Context{
		JobID:      100,
		Lib:        lib,
		CurrentJob: job,
		Walker:     walker,
	}

	count := PhaseCountFiles(context.Background(), dctx, deps)

	// Should count exactly 3 media files (root.mp4, action.mkv, comedy.mp4)
	// and exclude 2 directories and 3 non-media files
	if count != 3 {
		t.Errorf("expected count of 3, got %d", count)
	}
}

func TestLogDiscoveryStats(t *testing.T) {
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
				DirsSkipped:      2,
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
				FilesSkipped:     5,
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
			// Should not panic for any input
			LogDiscoveryStats(testLogger(), 100, tt.stats)
		})
	}
}

func TestPhaseHandleDeleted(t *testing.T) {
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
				_, err := repos.ScanState.GetByPath(context.Background(), 1, "/media/deleted1.mp4")
				if err == nil {
					t.Error("expected deleted1.mp4 to be removed")
				}

				_, err = repos.ScanState.GetByPath(context.Background(), 1, "/media/deleted2.mp4")
				if err == nil {
					t.Error("expected deleted2.mp4 to be removed")
				}

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
				_, err := repos.ScanState.GetByPath(context.Background(), 1, "/media/file.mp4")
				if err != nil {
					t.Errorf("expected file to remain, got error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos := tt.setupRepos(t)

			deps := &Deps{
				ScanRepos: repos,
				Logger:    testLogger(),
			}

			dctx := &Context{
				JobID: 100,
				Lib:   &library.Library{ID: 1, Path: "/media"},
			}

			PhaseHandleDeleted(context.Background(), dctx, deps, tt.diff)

			tt.validateRepo(t, repos)
		})
	}
}

func TestPhaseHandleDeleted_WithDeleteError(t *testing.T) {
	// Test the error path when DeleteByPaths fails
	scanStateRepo := mocks.NewScanStateRepository(t)
	scanStateRepo.DeleteByPathsErr = errors.New("database error")

	states := []*scanner.ScanState{
		{LibraryID: 1, FilePath: "/media/deleted1.mp4", FileSize: 1000},
		{LibraryID: 1, FilePath: "/media/deleted2.mp4", FileSize: 2000},
	}
	scanStateRepo.WithStates(states...)

	scanRepos := &scan.ScanRepositories{
		ScanState: scanStateRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
	}

	lib := &library.Library{ID: 1, Path: "/media"}
	dctx := &Context{
		JobID: 100,
		Lib:   lib,
	}

	diff := &scanner.ScanDiff{
		DeletedFiles: []string{"/media/deleted1.mp4", "/media/deleted2.mp4"},
	}

	// Should not panic even when DeleteByPaths fails
	PhaseHandleDeleted(context.Background(), dctx, deps, diff)

	// Files should still exist since deletion failed
	_, err := scanRepos.ScanState.GetByPath(context.Background(), 1, "/media/deleted1.mp4")
	if err != nil {
		t.Error("expected deleted1.mp4 to still exist after deletion error")
	}
}

func TestPhaseWalkDirectory(t *testing.T) {
	tests := []struct {
		name            string
		setupDir        func(*testing.T) string
		expectedFiles   int
		expectError     bool
		progressCalls   int
	}{
		{
			name: "walks empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectedFiles: 0,
			expectError:   false,
		},
		{
			name: "walks directory with media files",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				createFileInDir(t, dir, "movie.mp4")
				createFileInDir(t, dir, "show.mkv")
				createFileInDir(t, dir, "readme.txt") // Should be filtered out
				return dir
			},
			expectedFiles: 2,
			expectError:   false,
		},
		{
			name: "handles nonexistent path gracefully",
			setupDir: func(t *testing.T) string {
				return "/nonexistent/path/12345"
			},
			expectedFiles: 0,
			expectError:   false, // Walker returns empty result, not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirPath := tt.setupDir(t)

			deps := &Deps{
				Config: &scan.Config{
					DiscoveryBufferSize: 100,
					DiscoveryLogEvery:   10,
				},
				IsMediaFile: func(ext string) bool {
					return ext == ".mp4" || ext == ".mkv"
				},
				Logger: testLogger(),
			}

			lib := &library.Library{ID: 1, Path: dirPath}
			walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))

			dctx := &Context{
				JobID:      100,
				Lib:        lib,
				CurrentJob: &scanner.ScanJob{ID: 100},
				Walker:     walker,
			}

			var progressCallCount int
			progressCallback := func(count int64) {
				progressCallCount++
			}

			files, err := PhaseWalkDirectory(context.Background(), dctx, deps, progressCallback)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if len(files) != tt.expectedFiles {
				t.Errorf("expected %d files, got %d", tt.expectedFiles, len(files))
			}
		})
	}
}

func TestPhaseWalkDirectory_WithProgressCallback(t *testing.T) {
	dir := t.TempDir()
	// Create multiple files to trigger progress callback
	for i := 0; i < 15; i++ {
		createFileInDir(t, dir, "movie"+string(rune('0'+i))+".mp4")
	}

	deps := &Deps{
		Config: &scan.Config{
			DiscoveryBufferSize: 100,
			DiscoveryLogEvery:   5, // Callback every 5 files
		},
		IsMediaFile: func(ext string) bool {
			return ext == ".mp4"
		},
		Logger: testLogger(),
	}

	lib := &library.Library{ID: 1, Path: dir}
	walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))

	dctx := &Context{
		JobID:      100,
		Lib:        lib,
		CurrentJob: &scanner.ScanJob{ID: 100},
		Walker:     walker,
	}

	var progressCallCount int
	progressCallback := func(count int64) {
		progressCallCount++
	}

	files, err := PhaseWalkDirectory(context.Background(), dctx, deps, progressCallback)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 15 {
		t.Errorf("expected 15 files, got %d", len(files))
	}

	// Should have been called at 5, 10, 15 files = 3 times during walk + 1 final
	if progressCallCount < 3 {
		t.Errorf("expected at least 3 progress callbacks, got %d", progressCallCount)
	}
}

func TestPhaseWalkDirectory_NilProgressCallback(t *testing.T) {
	// Test with nil progress callback to ensure no panics
	dir := t.TempDir()
	createFileInDir(t, dir, "movie1.mp4")
	createFileInDir(t, dir, "movie2.mp4")

	deps := &Deps{
		Config: &scan.Config{
			DiscoveryBufferSize: 100,
			DiscoveryLogEvery:   1,
		},
		IsMediaFile: func(ext string) bool {
			return ext == ".mp4"
		},
		Logger: testLogger(),
	}

	lib := &library.Library{ID: 1, Path: dir}
	walker := filesystem.NewWalker(filesystem.WithLogger(testLogger()))

	dctx := &Context{
		JobID:      100,
		Lib:        lib,
		CurrentJob: &scanner.ScanJob{ID: 100},
		Walker:     walker,
	}

	// Call with nil callback - should not panic
	files, err := PhaseWalkDirectory(context.Background(), dctx, deps, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestPhaseDetermineChanges(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	laterTime := baseTime.Add(time.Hour)

	tests := []struct {
		name            string
		discoveredFiles []scanner.FileInfo
		setupRepo       func(*mocks.ScanStateRepository)
		expectNil       bool
	}{
		{
			name:            "returns diff when new files detected",
			discoveredFiles: []scanner.FileInfo{{Path: "/media/movie.mp4", Size: 1000, ModTime: baseTime}},
			setupRepo: func(m *mocks.ScanStateRepository) {
				// No previous state - all files are new
			},
			expectNil: false,
		},
		{
			name:            "returns nil when no changes (unchanged files)",
			discoveredFiles: []scanner.FileInfo{{Path: "/media/existing.mp4", Size: 1000, ModTime: baseTime}},
			setupRepo: func(m *mocks.ScanStateRepository) {
				// Previous state matches current - no changes
				m.WithStates(&scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/existing.mp4",
					FileSize:  1000,
					FileMTime: baseTime,
				})
			},
			expectNil: true,
		},
		{
			name:            "returns diff when files modified",
			discoveredFiles: []scanner.FileInfo{{Path: "/media/movie.mp4", Size: 2000, ModTime: laterTime}},
			setupRepo: func(m *mocks.ScanStateRepository) {
				// Previous state has different size/mtime
				m.WithStates(&scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/movie.mp4",
					FileSize:  1000,
					FileMTime: baseTime,
				})
			},
			expectNil: false,
		},
		{
			name:            "falls back to full scan on repo error",
			discoveredFiles: []scanner.FileInfo{{Path: "/media/movie.mp4", Size: 1000, ModTime: baseTime}},
			setupRepo: func(m *mocks.ScanStateRepository) {
				m.GetLibraryStateErr = errors.New("database error")
			},
			expectNil: false, // Falls back to treating all files as new
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanStateRepo := mocks.NewScanStateRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(scanStateRepo)
			}

			incrScanner := NewIncrementalScanner(scanStateRepo, testLogger())

			deps := &Deps{
				IncrScanner: incrScanner,
				Logger:      testLogger(),
			}

			lib := &library.Library{ID: 1, Path: "/media"}
			dctx := &Context{
				JobID:      100,
				Lib:        lib,
				CurrentJob: &scanner.ScanJob{ID: 100},
			}

			result := PhaseDetermineChanges(context.Background(), dctx, deps, tt.discoveredFiles)

			if tt.expectNil && result != nil {
				t.Error("expected nil diff but got non-nil")
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil diff but got nil")
			}
		})
	}
}

func createFileInDir(t *testing.T, dir, name string) {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}
