package scanner

import "time"

// CheckpointStatus represents the processing state of a file in a scan
type CheckpointStatus string

const (
	CheckpointPending    CheckpointStatus = "pending"
	CheckpointProcessing CheckpointStatus = "processing"
	CheckpointCompleted  CheckpointStatus = "completed"
	CheckpointFailed     CheckpointStatus = "failed"
	CheckpointWarning    CheckpointStatus = "warning" // Processed with non-fatal warnings
)

// ErrorCategory represents the type of error that occurred during file processing
type ErrorCategory string

const (
	ErrorCategoryParsing    ErrorCategory = "parsing"
	ErrorCategoryFFmpeg     ErrorCategory = "ffmpeg"
	ErrorCategoryDatabase   ErrorCategory = "database"
	ErrorCategoryFilesystem ErrorCategory = "filesystem"
	ErrorCategoryMetadata   ErrorCategory = "metadata"
)

// ScanCheckpoint tracks the processing state of an individual file within a scan job
type ScanCheckpoint struct {
	ID            int64
	ScanJobID     int64
	FilePath      string
	Status        CheckpointStatus
	FileSize      int64
	FileHash      string
	ErrorMessage  string
	ErrorCategory ErrorCategory
	RetryCount    int
	ProcessedAt   *time.Time
	CreatedAt     time.Time
}

// CheckpointStats provides aggregate statistics about checkpoint progress
type CheckpointStats struct {
	TotalFiles       int64
	PendingFiles     int64
	ProcessedFiles   int64 // Completed + Failed + Warning
	CompletedFiles   int64
	FailedFiles      int64
	WarningFiles     int64 // Files processed with warnings
	ErrorsByCategory map[ErrorCategory]int64
	FirstProcessedAt *time.Time // When processing started (for ETA calculation)
}

// GetProcessedFiles returns the total number of processed files (completed + failed + warning)
func (s *CheckpointStats) GetProcessedFiles() int64 {
	return s.CompletedFiles + s.FailedFiles + s.WarningFiles
}

// GetSuccessRate returns the percentage of successfully processed files
func (s *CheckpointStats) GetSuccessRate() float64 {
	if s.TotalFiles == 0 {
		return 0
	}
	return float64(s.CompletedFiles) / float64(s.TotalFiles) * 100
}

// GetProgress returns the processing progress percentage
func (s *CheckpointStats) GetProgress() float64 {
	if s.TotalFiles == 0 {
		return 0
	}
	return float64(s.GetProcessedFiles()) / float64(s.TotalFiles) * 100
}

// EstimateRemainingSeconds calculates the estimated time remaining for the scan.
// Returns nil if ETA cannot be calculated (no processing started or no files remaining).
// Uses FirstProcessedAt for accurate timing - job.StartedAt includes discovery/hashing overhead.
func (s *CheckpointStats) EstimateRemainingSeconds() *int64 {
	if s.ProcessedFiles == 0 || s.TotalFiles == 0 || s.FirstProcessedAt == nil {
		return nil
	}

	remaining := s.TotalFiles - s.ProcessedFiles
	if remaining <= 0 {
		return nil
	}

	elapsed := time.Since(*s.FirstProcessedAt).Seconds()
	if elapsed <= 0 {
		return nil
	}

	rate := float64(s.ProcessedFiles) / elapsed
	if rate <= 0 {
		return nil
	}

	eta := int64(float64(remaining) / rate)
	return &eta
}
