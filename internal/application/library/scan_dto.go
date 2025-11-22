package library

import (
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// StartScanRequest represents the input for starting a library scan
type StartScanRequest struct {
	LibraryID int64 `json:"library_id" validate:"required,gt=0"`
}

// StartScanResponse represents the output after starting a scan
type StartScanResponse struct {
	JobID      int64  `json:"job_id"`
	LibraryID  int64  `json:"library_id"`
	Status     string `json:"status"`
	StartedAt  time.Time `json:"started_at"`
}

// ScanProgressResponse represents the current scan progress
type ScanProgressResponse struct {
	JobID          int64     `json:"job_id"`
	LibraryID      int64     `json:"library_id"`
	Status         string    `json:"status"`
	Progress       float64   `json:"progress"`
	FilesFound     int64     `json:"files_found"`
	FilesProcessed int64     `json:"files_processed"`
	BytesProcessed int64     `json:"bytes_processed"`
	ErrorCount     int64     `json:"error_count"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// ScanHistoryResponse represents a list of scan jobs for a library
type ScanHistoryResponse struct {
	Jobs  []ScanProgressResponse `json:"jobs"`
	Total int                    `json:"total"`
}

// ToStartScanResponse converts a scan job to start scan response DTO
func ToStartScanResponse(job *scanner.ScanJob) StartScanResponse {
	return StartScanResponse{
		JobID:     job.ID,
		LibraryID: job.LibraryID,
		Status:    string(job.Status),
		StartedAt: job.StartedAt,
	}
}

// ToScanProgressResponse converts a scan job to progress response DTO
func ToScanProgressResponse(job *scanner.ScanJob) ScanProgressResponse {
	return ScanProgressResponse{
		JobID:          job.ID,
		LibraryID:      job.LibraryID,
		Status:         string(job.Status),
		Progress:       job.Progress,
		FilesFound:     job.FilesFound,
		FilesProcessed: job.FilesProcessed,
		BytesProcessed: job.BytesProcessed,
		ErrorCount:     job.ErrorCount,
		StartedAt:      job.StartedAt,
		CompletedAt:    job.CompletedAt,
		ErrorMessage:   job.ErrorMessage,
	}
}

// ToScanHistoryResponse converts a list of scan jobs to history response DTO
func ToScanHistoryResponse(jobs []*scanner.ScanJob) ScanHistoryResponse {
	responses := make([]ScanProgressResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = ToScanProgressResponse(job)
	}

	return ScanHistoryResponse{
		Jobs:  responses,
		Total: len(responses),
	}
}
