package scanner

import "time"

// CheckpointStatus represents the processing state of a file in a scan
type CheckpointStatus string

const (
	CheckpointPending    CheckpointStatus = "pending"
	CheckpointProcessing CheckpointStatus = "processing"
	CheckpointCompleted  CheckpointStatus = "completed"
	CheckpointFailed     CheckpointStatus = "failed"
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
	ProcessedFiles   int64 // Completed + Failed
	CompletedFiles   int64
	FailedFiles      int64
	ErrorsByCategory map[ErrorCategory]int64
}

// GetProcessedFiles returns the total number of processed files (completed + failed)
func (s *CheckpointStats) GetProcessedFiles() int64 {
	return s.CompletedFiles + s.FailedFiles
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
