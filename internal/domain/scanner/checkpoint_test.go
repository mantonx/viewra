package scanner

import (
	"testing"
	"time"
)

func TestCheckpointStatusConstants(t *testing.T) {
	statuses := []struct {
		status   CheckpointStatus
		expected string
	}{
		{CheckpointPending, "pending"},
		{CheckpointProcessing, "processing"},
		{CheckpointCompleted, "completed"},
		{CheckpointFailed, "failed"},
		{CheckpointWarning, "warning"},
	}

	for _, tt := range statuses {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestErrorCategoryConstants(t *testing.T) {
	categories := []struct {
		category ErrorCategory
		expected string
	}{
		{ErrorCategoryParsing, "parsing"},
		{ErrorCategoryFFmpeg, "ffmpeg"},
		{ErrorCategoryDatabase, "database"},
		{ErrorCategoryFilesystem, "filesystem"},
		{ErrorCategoryMetadata, "metadata"},
	}

	for _, tt := range categories {
		t.Run(string(tt.category), func(t *testing.T) {
			if string(tt.category) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.category)
			}
		})
	}
}

func TestScanCheckpointCreation(t *testing.T) {
	now := time.Now()
	checkpoint := ScanCheckpoint{
		ID:            1,
		ScanJobID:     100,
		FilePath:      "/path/to/movie.mkv",
		Status:        CheckpointPending,
		FileSize:      1024 * 1024 * 500,
		FileHash:      "abc123def456",
		ErrorMessage:  "",
		ErrorCategory: "",
		RetryCount:    0,
		ProcessedAt:   nil,
		CreatedAt:     now,
	}

	if checkpoint.ID != 1 {
		t.Errorf("Expected ID=1, got %d", checkpoint.ID)
	}
	if checkpoint.ScanJobID != 100 {
		t.Errorf("Expected ScanJobID=100, got %d", checkpoint.ScanJobID)
	}
	if checkpoint.Status != CheckpointPending {
		t.Errorf("Expected Status=pending, got %s", checkpoint.Status)
	}
	if checkpoint.FileSize != 1024*1024*500 {
		t.Errorf("Expected FileSize=500MB, got %d", checkpoint.FileSize)
	}
}

func TestCheckpointStatsGetProcessedFiles(t *testing.T) {
	tests := []struct {
		name      string
		completed int64
		failed    int64
		warning   int64
		expected  int64
	}{
		{"all zeros", 0, 0, 0, 0},
		{"only completed", 50, 0, 0, 50},
		{"only failed", 0, 10, 0, 10},
		{"only warnings", 0, 0, 5, 5},
		{"mixed", 50, 10, 5, 65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CheckpointStats{
				CompletedFiles: tt.completed,
				FailedFiles:    tt.failed,
				WarningFiles:   tt.warning,
			}
			result := stats.GetProcessedFiles()
			if result != tt.expected {
				t.Errorf("GetProcessedFiles() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCheckpointStatsGetSuccessRate(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		completed int64
		expected  float64
	}{
		{"zero files", 0, 0, 0},
		{"all complete", 100, 100, 100},
		{"half complete", 100, 50, 50},
		{"quarter complete", 100, 25, 25},
		{"none complete", 100, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CheckpointStats{
				TotalFiles:     tt.total,
				CompletedFiles: tt.completed,
			}
			result := stats.GetSuccessRate()
			if result != tt.expected {
				t.Errorf("GetSuccessRate() = %.1f, want %.1f", result, tt.expected)
			}
		})
	}
}

func TestCheckpointStatsGetProgress(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		completed int64
		failed    int64
		warning   int64
		expected  float64
	}{
		{"zero files", 0, 0, 0, 0, 0},
		{"all processed", 100, 90, 5, 5, 100},
		{"half processed", 100, 40, 5, 5, 50},
		{"no progress", 100, 0, 0, 0, 0},
		{"only failures", 100, 0, 50, 0, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CheckpointStats{
				TotalFiles:     tt.total,
				CompletedFiles: tt.completed,
				FailedFiles:    tt.failed,
				WarningFiles:   tt.warning,
			}
			result := stats.GetProgress()
			if result != tt.expected {
				t.Errorf("GetProgress() = %.1f, want %.1f", result, tt.expected)
			}
		})
	}
}

func TestCheckpointStatsEstimateRemainingSeconds(t *testing.T) {
	now := time.Now()
	twoSecondsAgo := now.Add(-2 * time.Second)

	tests := []struct {
		name           string
		processedFiles int64
		actualTotal    int64
		firstProcessed *time.Time
		expectNil      bool
	}{
		{"zero processed", 0, 100, &twoSecondsAgo, true},
		{"zero total", 50, 0, &twoSecondsAgo, true},
		{"nil first processed", 50, 100, nil, true},
		{"all done (no remaining)", 100, 100, &twoSecondsAgo, true},
		{"normal case", 50, 100, &twoSecondsAgo, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CheckpointStats{
				ProcessedFiles:   tt.processedFiles,
				FirstProcessedAt: tt.firstProcessed,
			}
			result := stats.EstimateRemainingSeconds(tt.actualTotal)
			if tt.expectNil && result != nil {
				t.Errorf("EstimateRemainingSeconds() = %v, want nil", *result)
			}
			if !tt.expectNil && result == nil {
				t.Error("EstimateRemainingSeconds() = nil, want non-nil")
			}
		})
	}
}

func TestCheckpointStatsErrorsByCategory(t *testing.T) {
	stats := CheckpointStats{
		TotalFiles:       100,
		CompletedFiles:   80,
		FailedFiles:      20,
		ErrorsByCategory: map[ErrorCategory]int64{
			ErrorCategoryFilesystem: 10,
			ErrorCategoryFFmpeg:     5,
			ErrorCategoryParsing:    5,
		},
	}

	if len(stats.ErrorsByCategory) != 3 {
		t.Errorf("Expected 3 error categories, got %d", len(stats.ErrorsByCategory))
	}

	if stats.ErrorsByCategory[ErrorCategoryFilesystem] != 10 {
		t.Errorf("Expected 10 filesystem errors, got %d", stats.ErrorsByCategory[ErrorCategoryFilesystem])
	}
}
