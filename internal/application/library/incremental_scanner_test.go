package library

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Tests for NewIncrementalScanner constructor

func TestNewIncrementalScanner(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "creates valid scanner instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewScanStateRepository(t)
			logger := discardLogger()

			scanner := NewIncrementalScanner(repo, logger)

			if scanner == nil {
				t.Fatal("Expected non-nil scanner")
			}
			if scanner.scanStateRepo == nil {
				t.Error("Expected scanStateRepo to be set")
			}
			if scanner.logger == nil {
				t.Error("Expected logger to be set")
			}
		})
	}
}

// Tests for DetermineChanges

func TestIncrementalScanner_DetermineChanges(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	tests := []struct {
		name              string
		libraryID         int64
		currentFiles      []scanner.FileInfo
		previousStates    []*scanner.ScanState
		setupRepo         func(*mocks.ScanStateRepository)
		expectedNewCount  int
		expectedModCount  int
		expectedDelCount  int
		expectedUnchCount int
		expectError       bool
	}{
		{
			name:      "fresh scan with no previous state",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: now},
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: now},
			},
			previousStates:    []*scanner.ScanState{},
			expectedNewCount:  2,
			expectedModCount:  0,
			expectedDelCount:  0,
			expectedUnchCount: 0,
		},
		{
			name:      "incremental scan with all unchanged files",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: oneHourAgo},
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo},
			},
			expectedNewCount:  0,
			expectedModCount:  0,
			expectedDelCount:  0,
			expectedUnchCount: 2,
		},
		{
			name:      "incremental scan with new files",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: oneHourAgo},
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo},
				{Path: "/media/movie3.mp4", Size: 3072, ModTime: now},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo},
			},
			expectedNewCount:  1,
			expectedModCount:  0,
			expectedDelCount:  0,
			expectedUnchCount: 2,
		},
		{
			name:      "incremental scan with modified files (mtime changed)",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: now}, // mtime changed
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: twoDaysAgo},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo},
			},
			expectedNewCount:  0,
			expectedModCount:  1,
			expectedDelCount:  0,
			expectedUnchCount: 1,
		},
		{
			name:      "incremental scan with modified files (size changed)",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 2048, ModTime: oneHourAgo}, // size changed
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo},
			},
			expectedNewCount:  0,
			expectedModCount:  1,
			expectedDelCount:  0,
			expectedUnchCount: 1,
		},
		{
			name:      "incremental scan with deleted files",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo},
			},
			expectedNewCount:  0,
			expectedModCount:  0,
			expectedDelCount:  1,
			expectedUnchCount: 1,
		},
		{
			name:      "complex incremental scan with all change types",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie2.mp4", Size: 2048, ModTime: oneHourAgo}, // unchanged
				{Path: "/media/movie3.mp4", Size: 3072, ModTime: now},        // new
				{Path: "/media/movie4.mp4", Size: 4096, ModTime: now},        // modified (mtime changed)
				{Path: "/media/movie5.mp4", Size: 6000, ModTime: oneHourAgo}, // modified (size changed)
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo}, // deleted
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048, FileMTime: oneHourAgo}, // unchanged
				{LibraryID: 1, FilePath: "/media/movie4.mp4", FileSize: 4096, FileMTime: twoDaysAgo}, // modified
				{LibraryID: 1, FilePath: "/media/movie5.mp4", FileSize: 5120, FileMTime: oneHourAgo}, // modified
			},
			expectedNewCount:  1,
			expectedModCount:  2,
			expectedDelCount:  1,
			expectedUnchCount: 1,
		},
		{
			name:         "handles empty current files list",
			libraryID:    1,
			currentFiles: []scanner.FileInfo{},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: oneHourAgo},
			},
			expectedNewCount:  0,
			expectedModCount:  0,
			expectedDelCount:  1,
			expectedUnchCount: 0,
		},
		{
			name:      "handles repository error",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: now},
			},
			setupRepo: func(repo *mocks.ScanStateRepository) {
				repo.GetLibraryStateErr = errors.New("database connection failed")
			},
			expectError: true,
		},
		{
			name:      "nanosecond precision in mtime comparison",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024, ModTime: now},
				{Path: "/media/movie2.mp4", Size: 1024, ModTime: now.Add(1 * time.Nanosecond)}, // 1ns difference
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024, FileMTime: now},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 1024, FileMTime: now}, // will detect as modified
			},
			expectedNewCount:  0,
			expectedModCount:  1, // movie2.mp4 differs by 1ns
			expectedDelCount:  0,
			expectedUnchCount: 1,
		},
		{
			name:      "files with identical mtime and size are unchanged",
			libraryID: 1,
			currentFiles: []scanner.FileInfo{
				{Path: "/media/movie1.mp4", Size: 1024000, ModTime: oneHourAgo},
				{Path: "/media/movie2.mp4", Size: 2048000, ModTime: oneHourAgo},
			},
			previousStates: []*scanner.ScanState{
				{LibraryID: 1, FilePath: "/media/movie1.mp4", FileSize: 1024000, FileMTime: oneHourAgo, FileHash: "hash1"},
				{LibraryID: 1, FilePath: "/media/movie2.mp4", FileSize: 2048000, FileMTime: oneHourAgo, FileHash: "hash2"},
			},
			expectedNewCount:  0,
			expectedModCount:  0,
			expectedDelCount:  0,
			expectedUnchCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewScanStateRepository(t)
			if len(tt.previousStates) > 0 {
				repo.WithStates(tt.previousStates...)
			}
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			scanner := NewIncrementalScanner(repo, discardLogger())

			diff, err := scanner.DetermineChanges(context.Background(), tt.libraryID, tt.currentFiles)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff == nil {
				t.Fatal("Expected non-nil diff")
			}

			if len(diff.NewFiles) != tt.expectedNewCount {
				t.Errorf("NewFiles count = %d, want %d", len(diff.NewFiles), tt.expectedNewCount)
			}

			if len(diff.ModifiedFiles) != tt.expectedModCount {
				t.Errorf("ModifiedFiles count = %d, want %d", len(diff.ModifiedFiles), tt.expectedModCount)
			}

			if len(diff.DeletedFiles) != tt.expectedDelCount {
				t.Errorf("DeletedFiles count = %d, want %d", len(diff.DeletedFiles), tt.expectedDelCount)
			}

			if len(diff.UnchangedFiles) != tt.expectedUnchCount {
				t.Errorf("UnchangedFiles count = %d, want %d", len(diff.UnchangedFiles), tt.expectedUnchCount)
			}
		})
	}
}

// Tests for isFileModified

func TestIncrementalScanner_isFileModified(t *testing.T) {
	baseTime := time.Now()
	laterTime := baseTime.Add(1 * time.Second)

	tests := []struct {
		name           string
		prevState      *scanner.ScanState
		currentFile    scanner.FileInfo
		expectModified bool
	}{
		{
			name: "identical mtime and size - not modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    1024,
				ModTime: baseTime,
			},
			expectModified: false,
		},
		{
			name: "mtime changed - modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    1024,
				ModTime: laterTime,
			},
			expectModified: true,
		},
		{
			name: "size changed - modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    2048,
				ModTime: baseTime,
			},
			expectModified: true,
		},
		{
			name: "both mtime and size changed - modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    2048,
				ModTime: laterTime,
			},
			expectModified: true,
		},
		{
			name: "nanosecond precision mtime difference - modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    1024,
				ModTime: baseTime.Add(1 * time.Nanosecond),
			},
			expectModified: true,
		},
		{
			name: "hash difference does not affect modification detection",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "oldhash",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    1024,
				ModTime: baseTime,
			},
			expectModified: false, // Hash is not checked during incremental scanning
		},
		{
			name: "empty hash in previous state - not modified if mtime and size match",
			prevState: &scanner.ScanState{
				FilePath:  "/media/movie.mp4",
				FileSize:  1024,
				FileMTime: baseTime,
				FileHash:  "",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/movie.mp4",
				Size:    1024,
				ModTime: baseTime,
			},
			expectModified: false,
		},
		{
			name: "large file size - not modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/large_movie.mp4",
				FileSize:  10737418240, // 10GB
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/large_movie.mp4",
				Size:    10737418240,
				ModTime: baseTime,
			},
			expectModified: false,
		},
		{
			name: "large file size changed by 1 byte - modified",
			prevState: &scanner.ScanState{
				FilePath:  "/media/large_movie.mp4",
				FileSize:  10737418240, // 10GB
				FileMTime: baseTime,
				FileHash:  "hash123",
			},
			currentFile: scanner.FileInfo{
				Path:    "/media/large_movie.mp4",
				Size:    10737418241, // 10GB + 1 byte
				ModTime: baseTime,
			},
			expectModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewIncrementalScanner(
				mocks.NewScanStateRepository(t),
				discardLogger(),
			)

			modified := scanner.isFileModified(tt.prevState, tt.currentFile)

			if modified != tt.expectModified {
				t.Errorf("isFileModified() = %v, want %v", modified, tt.expectModified)
			}
		})
	}
}

// Integration tests verifying the complete flow

func TestIncrementalScanner_Integration(t *testing.T) {
	tests := []struct {
		name             string
		scenario         string
		setupInitialScan func() ([]*scanner.ScanState, []scanner.FileInfo)
		validateDiff     func(*testing.T, *scanner.ScanDiff)
	}{
		{
			name:     "typical incremental scan workflow",
			scenario: "Library has been scanned before, one file added, one modified, one deleted",
			setupInitialScan: func() ([]*scanner.ScanState, []scanner.FileInfo) {
				baseTime := time.Now().Add(-24 * time.Hour)

				// Previous scan state
				previousStates := []*scanner.ScanState{
					{LibraryID: 1, FilePath: "/library/movie1.mp4", FileSize: 1024000, FileMTime: baseTime},
					{LibraryID: 1, FilePath: "/library/movie2.mp4", FileSize: 2048000, FileMTime: baseTime},
					{LibraryID: 1, FilePath: "/library/movie3.mp4", FileSize: 3072000, FileMTime: baseTime},
				}

				// Current filesystem state
				currentFiles := []scanner.FileInfo{
					{Path: "/library/movie1.mp4", Size: 1024000, ModTime: baseTime},   // unchanged
					{Path: "/library/movie2.mp4", Size: 2048000, ModTime: time.Now()}, // modified (mtime)
					{Path: "/library/movie4.mp4", Size: 4096000, ModTime: time.Now()}, // new
					// movie3.mp4 is deleted
				}

				return previousStates, currentFiles
			},
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if len(diff.NewFiles) != 1 {
					t.Errorf("Expected 1 new file, got %d", len(diff.NewFiles))
				}
				if len(diff.ModifiedFiles) != 1 {
					t.Errorf("Expected 1 modified file, got %d", len(diff.ModifiedFiles))
				}
				if len(diff.DeletedFiles) != 1 {
					t.Errorf("Expected 1 deleted file, got %d", len(diff.DeletedFiles))
				}
				if len(diff.UnchangedFiles) != 1 {
					t.Errorf("Expected 1 unchanged file, got %d", len(diff.UnchangedFiles))
				}
				if !diff.NeedsProcessing() {
					t.Error("Expected diff to need processing")
				}
				if diff.TotalChanges() != 3 {
					t.Errorf("Expected 3 total changes, got %d", diff.TotalChanges())
				}
			},
		},
		{
			name:     "first scan of library",
			scenario: "No previous scan state exists",
			setupInitialScan: func() ([]*scanner.ScanState, []scanner.FileInfo) {
				currentFiles := []scanner.FileInfo{
					{Path: "/library/movie1.mp4", Size: 1024000, ModTime: time.Now()},
					{Path: "/library/movie2.mp4", Size: 2048000, ModTime: time.Now()},
					{Path: "/library/movie3.mp4", Size: 3072000, ModTime: time.Now()},
				}
				return []*scanner.ScanState{}, currentFiles
			},
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if len(diff.NewFiles) != 3 {
					t.Errorf("Expected 3 new files, got %d", len(diff.NewFiles))
				}
				if len(diff.ModifiedFiles) != 0 {
					t.Errorf("Expected 0 modified files, got %d", len(diff.ModifiedFiles))
				}
				if len(diff.DeletedFiles) != 0 {
					t.Errorf("Expected 0 deleted files, got %d", len(diff.DeletedFiles))
				}
				if len(diff.UnchangedFiles) != 0 {
					t.Errorf("Expected 0 unchanged files, got %d", len(diff.UnchangedFiles))
				}
			},
		},
		{
			name:     "no changes since last scan",
			scenario: "All files are identical to previous scan",
			setupInitialScan: func() ([]*scanner.ScanState, []scanner.FileInfo) {
				baseTime := time.Now().Add(-1 * time.Hour)

				previousStates := []*scanner.ScanState{
					{LibraryID: 1, FilePath: "/library/movie1.mp4", FileSize: 1024000, FileMTime: baseTime},
					{LibraryID: 1, FilePath: "/library/movie2.mp4", FileSize: 2048000, FileMTime: baseTime},
				}

				currentFiles := []scanner.FileInfo{
					{Path: "/library/movie1.mp4", Size: 1024000, ModTime: baseTime},
					{Path: "/library/movie2.mp4", Size: 2048000, ModTime: baseTime},
				}

				return previousStates, currentFiles
			},
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if len(diff.NewFiles) != 0 {
					t.Errorf("Expected 0 new files, got %d", len(diff.NewFiles))
				}
				if len(diff.ModifiedFiles) != 0 {
					t.Errorf("Expected 0 modified files, got %d", len(diff.ModifiedFiles))
				}
				if len(diff.DeletedFiles) != 0 {
					t.Errorf("Expected 0 deleted files, got %d", len(diff.DeletedFiles))
				}
				if len(diff.UnchangedFiles) != 2 {
					t.Errorf("Expected 2 unchanged files, got %d", len(diff.UnchangedFiles))
				}
				if diff.NeedsProcessing() {
					t.Error("Expected diff to NOT need processing")
				}
				if diff.TotalChanges() != 0 {
					t.Errorf("Expected 0 total changes, got %d", diff.TotalChanges())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewScanStateRepository(t)
			previousStates, currentFiles := tt.setupInitialScan()

			if len(previousStates) > 0 {
				repo.WithStates(previousStates...)
			}

			scanner := NewIncrementalScanner(repo, discardLogger())
			diff, err := scanner.DetermineChanges(context.Background(), 1, currentFiles)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validateDiff(t, diff)
		})
	}
}
