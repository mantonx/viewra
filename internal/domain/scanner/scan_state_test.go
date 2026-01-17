package scanner

import (
	"testing"
	"time"
)

func TestScanStateCreation(t *testing.T) {
	now := time.Now()
	mediaID := int64(123)

	state := ScanState{
		ID:              1,
		LibraryID:       100,
		FilePath:        "/path/to/movie.mkv",
		FileSize:        1024 * 1024 * 500,
		FileMTime:       now,
		FileHash:        "abc123",
		MediaID:         &mediaID,
		LastScannedAt:   now,
		ScanJobID:       200,
		CreatedAt:       now,
		HasWarning:      false,
		WarningMessage:  "",
		WarningCategory: "",
		HasError:        false,
		ErrorMessage:    "",
		ErrorCategory:   "",
	}

	if state.ID != 1 {
		t.Errorf("Expected ID=1, got %d", state.ID)
	}
	if state.LibraryID != 100 {
		t.Errorf("Expected LibraryID=100, got %d", state.LibraryID)
	}
	if state.FilePath != "/path/to/movie.mkv" {
		t.Errorf("Expected FilePath=/path/to/movie.mkv, got %s", state.FilePath)
	}
	if state.MediaID == nil || *state.MediaID != 123 {
		t.Errorf("Expected MediaID=123, got %v", state.MediaID)
	}
}

func TestScanStateWithWarning(t *testing.T) {
	state := ScanState{
		ID:              1,
		LibraryID:       100,
		FilePath:        "/path/to/movie.mkv",
		HasWarning:      true,
		WarningMessage:  "Unable to extract complete metadata",
		WarningCategory: "metadata",
	}

	if !state.HasWarning {
		t.Error("Expected HasWarning=true")
	}
	if state.WarningMessage != "Unable to extract complete metadata" {
		t.Errorf("Expected warning message, got %s", state.WarningMessage)
	}
	if state.WarningCategory != "metadata" {
		t.Errorf("Expected WarningCategory=metadata, got %s", state.WarningCategory)
	}
}

func TestScanStateWithError(t *testing.T) {
	state := ScanState{
		ID:            1,
		LibraryID:     100,
		FilePath:      "/path/to/movie.mkv",
		HasError:      true,
		ErrorMessage:  "FFmpeg failed to process file",
		ErrorCategory: "ffmpeg",
	}

	if !state.HasError {
		t.Error("Expected HasError=true")
	}
	if state.ErrorMessage != "FFmpeg failed to process file" {
		t.Errorf("Expected error message, got %s", state.ErrorMessage)
	}
	if state.ErrorCategory != "ffmpeg" {
		t.Errorf("Expected ErrorCategory=ffmpeg, got %s", state.ErrorCategory)
	}
}

func TestScanDiffNeedsProcessing(t *testing.T) {
	tests := []struct {
		name     string
		diff     ScanDiff
		expected bool
	}{
		{
			name:     "empty diff",
			diff:     ScanDiff{},
			expected: false,
		},
		{
			name: "only unchanged files",
			diff: ScanDiff{
				UnchangedFiles: []string{"/file1", "/file2"},
			},
			expected: false,
		},
		{
			name: "new files",
			diff: ScanDiff{
				NewFiles: []FileInfo{{Path: "/new/file.mkv"}},
			},
			expected: true,
		},
		{
			name: "modified files",
			diff: ScanDiff{
				ModifiedFiles: []FileInfo{{Path: "/modified/file.mkv"}},
			},
			expected: true,
		},
		{
			name: "deleted files",
			diff: ScanDiff{
				DeletedFiles: []string{"/deleted/file.mkv"},
			},
			expected: true,
		},
		{
			name: "all changes",
			diff: ScanDiff{
				NewFiles:       []FileInfo{{Path: "/new.mkv"}},
				ModifiedFiles:  []FileInfo{{Path: "/mod.mkv"}},
				DeletedFiles:   []string{"/del.mkv"},
				UnchangedFiles: []string{"/same.mkv"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diff.NeedsProcessing()
			if result != tt.expected {
				t.Errorf("NeedsProcessing() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScanDiffSummary(t *testing.T) {
	diff := ScanDiff{
		NewFiles:       []FileInfo{{Path: "/new1.mkv"}, {Path: "/new2.mkv"}},
		ModifiedFiles:  []FileInfo{{Path: "/mod.mkv"}},
		DeletedFiles:   []string{"/del1.mkv", "/del2.mkv", "/del3.mkv"},
		UnchangedFiles: []string{"/same1.mkv", "/same2.mkv", "/same3.mkv", "/same4.mkv"},
	}

	expected := "new=2, modified=1, deleted=3, unchanged=4"
	result := diff.Summary()
	if result != expected {
		t.Errorf("Summary() = %q, want %q", result, expected)
	}
}

func TestScanDiffTotalChanges(t *testing.T) {
	tests := []struct {
		name     string
		diff     ScanDiff
		expected int
	}{
		{
			name:     "empty diff",
			diff:     ScanDiff{},
			expected: 0,
		},
		{
			name: "only unchanged",
			diff: ScanDiff{
				UnchangedFiles: []string{"/a", "/b", "/c"},
			},
			expected: 0,
		},
		{
			name: "new files only",
			diff: ScanDiff{
				NewFiles: []FileInfo{{Path: "/a"}, {Path: "/b"}},
			},
			expected: 2,
		},
		{
			name: "all types",
			diff: ScanDiff{
				NewFiles:       []FileInfo{{Path: "/new"}},
				ModifiedFiles:  []FileInfo{{Path: "/mod1"}, {Path: "/mod2"}},
				DeletedFiles:   []string{"/del1", "/del2", "/del3"},
				UnchangedFiles: []string{"/same"},
			},
			expected: 6, // 1 + 2 + 3, unchanged not counted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diff.TotalChanges()
			if result != tt.expected {
				t.Errorf("TotalChanges() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestLibraryIssueCountsCreation(t *testing.T) {
	counts := LibraryIssueCounts{
		ErrorCount:   10,
		WarningCount: 25,
	}

	if counts.ErrorCount != 10 {
		t.Errorf("Expected ErrorCount=10, got %d", counts.ErrorCount)
	}
	if counts.WarningCount != 25 {
		t.Errorf("Expected WarningCount=25, got %d", counts.WarningCount)
	}
}

func TestScanStateNilMediaID(t *testing.T) {
	state := ScanState{
		ID:        1,
		LibraryID: 100,
		FilePath:  "/path/to/movie.mkv",
		MediaID:   nil,
	}

	if state.MediaID != nil {
		t.Error("Expected MediaID to be nil")
	}
}
