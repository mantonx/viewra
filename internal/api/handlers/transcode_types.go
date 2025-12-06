package handlers

import (
	transcodeDomain "github.com/mantonx/viewra/internal/domain/transcode"
)

// CreateTranscodeJobRequest represents a request to start transcoding.
type CreateTranscodeJobRequest struct {
	Quality       string `json:"quality" binding:"required"`
	Codec         string `json:"codec,omitempty"`          // Optional: h264, h265, vp9, av1 (defaults to h264)
	StartPosition int    `json:"start_position,omitempty"` // Optional: start position in seconds for seek-based transcoding
}

// TranscodeJobResponse represents a transcode job response.
type TranscodeJobResponse struct {
	ID          int64  `json:"id"`
	MediaID     int64  `json:"media_id"`
	Quality     string `json:"quality"`
	Codec       string `json:"codec"` // Video codec: h264, h265, vp9, av1
	Type        string `json:"type"`  // Job type: remux, remux_audio, or transcode
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	Error       string `json:"error,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// CleanupRequest represents a transcode cleanup request.
type CleanupRequest struct {
	MediaID        *int64  `json:"media_id"`
	Quality        *string `json:"quality"`
	Failed         bool    `json:"failed"`
	Orphans        bool    `json:"orphans"`
	OlderThanHours *int    `json:"older_than_hours"`
	DryRun         bool    `json:"dry_run"`
}

// CleanupResponse represents cleanup operation results.
type CleanupResponse struct {
	DeletedCount     int      `json:"deleted_count"`
	DeletedSizeBytes int64    `json:"deleted_size_bytes"`
	DeletedSizeHuman string   `json:"deleted_size_human"`
	FailedCount      int      `json:"failed_count"`
	Errors           []string `json:"errors,omitempty"`
	DryRun           bool     `json:"dry_run"`
}

// DiskUsageResponse represents disk usage statistics.
type DiskUsageResponse struct {
	OutputDir       string `json:"output_dir"`
	TotalSizeBytes  int64  `json:"total_size_bytes"`
	TotalSizeHuman  string `json:"total_size_human"`
	FileCount       int    `json:"file_count"`
	TotalJobs       int    `json:"total_jobs"`
	CompletedCount  int    `json:"completed_count"`
	FailedCount     int    `json:"failed_count"`
	QueuedCount     int    `json:"queued_count"`
	ProcessingCount int    `json:"processing_count"`
}

// toTranscodeJobResponse converts a domain transcode job to an API response.
func toTranscodeJobResponse(job *transcodeDomain.TranscodeJob) TranscodeJobResponse {
	response := TranscodeJobResponse{
		ID:        job.ID,
		MediaID:   job.MediaID,
		Quality:   job.Quality,
		Codec:     job.Codec,
		Type:      job.Type,
		Status:    job.Status,
		Progress:  job.Progress,
		CreatedAt: formatTime(job.CreatedAt),
	}

	if job.Error != "" {
		response.Error = job.Error
	}

	if !job.StartedAt.IsZero() {
		response.StartedAt = formatTime(job.StartedAt)
	}

	if !job.CompletedAt.IsZero() {
		response.CompletedAt = formatTime(job.CompletedAt)
	}

	return response
}
