package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ScanJobRepository defines the interface for scan job data access
type ScanJobRepository interface {
	GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error)
	ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error)
	GetByID(ctx context.Context, jobID int64) (*scanner.ScanJob, error)
	UpdateStatus(ctx context.Context, jobID int64, status scanner.ScanStatus) error
}

// CheckpointRepository defines the interface for checkpoint data access
type CheckpointRepository interface {
	GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error)
	ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error)
	ResetFailed(ctx context.Context, jobID int64) (int64, error)
}

// ScanStateRepository defines the interface for scan state data access
type ScanStateRepository interface {
	GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error)
	CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error)
	CountByLibrary(ctx context.Context, libraryID int64) (int64, error)
}

// ScanLibraryExecutor defines the interface for resuming scans
type ScanLibraryExecutor interface {
	ResumeScan(ctx context.Context, jobID int64) error
}

// ScanJobHandler handles scan job-related HTTP requests
type ScanJobHandler struct {
	scanJobRepo     ScanJobRepository
	checkpointRepo  CheckpointRepository
	scanStateRepo   ScanStateRepository
	scanLibrary     ScanLibraryExecutor
	logger          *slog.Logger
}

// NewScanJobHandler creates a new scan job handler
func NewScanJobHandler(scanJobRepo ScanJobRepository, checkpointRepo CheckpointRepository, scanStateRepo ScanStateRepository, scanLibrary ScanLibraryExecutor, logger *slog.Logger) *ScanJobHandler {
	return &ScanJobHandler{
		scanJobRepo:    scanJobRepo,
		checkpointRepo: checkpointRepo,
		scanStateRepo:  scanStateRepo,
		scanLibrary:    scanLibrary,
		logger:         logger,
	}
}

// ScanStatusResponse represents the current status of a library scan
type ScanStatusResponse struct {
	JobID          int64   `json:"job_id"`                    // Scan job ID
	Status         string  `json:"status"`                    // pending, running, paused, completed, failed
	Progress       float64 `json:"progress"`                  // 0-100
	FilesFound     int64   `json:"files_found"`               // Total files discovered
	FilesProcessed int64   `json:"files_processed"`           // Files processed so far
	BytesProcessed int64   `json:"bytes_processed"`           // Bytes processed
	ErrorCount     int64   `json:"error_count"`               // Number of errors encountered
	WarningCount   int64   `json:"warning_count"`             // Number of warnings encountered
	ErrorsJobID    *int64  `json:"errors_job_id,omitempty"`   // Job ID where errors/warnings are from (if different from JobID)
	ErrorMessage   string  `json:"error_message,omitempty"`   // Error message if failed
	StartedAt      string  `json:"started_at"`                // ISO 8601 timestamp
	CompletedAt    *string `json:"completed_at,omitempty"`    // ISO 8601 timestamp
	Phase          string  `json:"phase,omitempty"`           // Current scan phase (discovering/processing/completed)
	EstimatedTotal int64   `json:"estimated_total,omitempty"` // Estimated total files from previous scan
	DiscoveryDone  bool    `json:"discovery_done"`            // Whether file discovery is complete
	ETASeconds     *int64  `json:"eta_seconds,omitempty"`     // Estimated seconds remaining (nil if unknown)

	// Discovery health metrics (added in v0.18)
	DiscoveryErrors   int64 `json:"discovery_errors,omitempty"`   // Errors during file discovery
	DiscoveryWarnings int64 `json:"discovery_warnings,omitempty"` // Warnings during discovery
	DirsScanned       int64 `json:"dirs_scanned,omitempty"`       // Directories successfully scanned
	DirsSkipped       int64 `json:"dirs_skipped,omitempty"`       // Directories that couldn't be read
	FilesSkipped      int64 `json:"files_skipped,omitempty"`      // Files that couldn't be stat'd
}

// ScanHistoryItem represents a historical scan job
type ScanHistoryItem struct {
	ID             int64   `json:"id"`
	Status         string  `json:"status"`
	Progress       float64 `json:"progress"`
	FilesFound     int64   `json:"files_found"`
	FilesProcessed int64   `json:"files_processed"`
	BytesProcessed int64   `json:"bytes_processed"`
	ErrorCount     int64   `json:"error_count"`
	WarningCount   int64   `json:"warning_count"`
	ErrorMessage   string  `json:"error_message,omitempty"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    *string `json:"completed_at,omitempty"`
	Duration       *int64  `json:"duration_seconds,omitempty"` // Duration in seconds
}

// ScanHistoryResponse represents a list of historical scan jobs
type ScanHistoryResponse struct {
	Jobs []ScanHistoryItem `json:"jobs"`
}

// GetStatus returns the current scan status for a library
// @Summary Get current scan status
// @Description Returns the current status of the most recent scan job for a library
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Success 200 {object} ScanStatusResponse "Scan status"
// @Success 404 {object} ErrorResponse "No scan jobs found for this library"
// @Failure 400 {object} ErrorResponse "Invalid library ID"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/status [get]
func (h *ScanJobHandler) GetStatus(c *gin.Context) {
	// Parse library ID
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}

	// Get latest scan job
	job, err := h.scanJobRepo.GetLatestByLibrary(c.Request.Context(), libraryID)
	if err != nil {
		if err == scanner.ErrNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "No scan jobs found for this library"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get scan status"})
		return
	}

	// Get library-level persistent error/warning counts from scan_state
	// This shows all unresolved issues across the library, not just from the current scan
	var errorCount int64
	var warningCount int64
	if counts, err := h.scanStateRepo.CountLibraryIssues(c.Request.Context(), libraryID); err == nil && counts != nil {
		errorCount = counts.ErrorCount
		warningCount = counts.WarningCount
	} else {
		// Fallback to job-based counts if scan_state query fails
		h.logger.Warn("failed to get library issue counts, using job counts",
			"library_id", libraryID,
			"error", err)
		errorCount = job.ErrorCount
		warningCount = job.WarningCount
	}

	// Get total files processed from scan_state (includes all scans, not just current job)
	// This gives users a sense of progress: files already in the database vs total discovered
	filesProcessed := job.FilesProcessed // Default to job count
	if totalProcessed, err := h.scanStateRepo.CountByLibrary(c.Request.Context(), libraryID); err == nil {
		filesProcessed = totalProcessed
	} else {
		// Fallback to job count if scan_state query fails
		h.logger.Warn("failed to get total processed count, using job count",
			"library_id", libraryID,
			"error", err)
	}

	// Calculate progress percentage based on total files in scan_state vs discovered
	// This gives users accurate progress: how much of the library is already processed
	progress := job.Progress // Default to job progress
	if job.FilesFound > 0 {
		progress = float64(filesProcessed) / float64(job.FilesFound) * 100.0
	}

	// Calculate ETA for running scans using checkpoint stats (not scan_state total!)
	// This gives accurate ETA based on files actually being processed by THIS scan
	var etaSeconds *int64
	if job.Status == scanner.ScanStatusRunning {
		// Get checkpoint statistics for the current scan job
		if stats, err := h.checkpointRepo.GetStats(c.Request.Context(), job.ID); err == nil && stats != nil {
			// Use checkpoint stats for accurate ETA calculation
			currentProcessed := stats.ProcessedFiles // Completed + Failed + Warning
			totalCheckpoints := stats.TotalFiles     // All checkpoints for this scan

			if currentProcessed > 0 && totalCheckpoints > 0 {
				// Calculate time elapsed
				elapsed := time.Since(job.StartedAt).Seconds()

				// Calculate processing rate (files per second) based on actual checkpoint processing
				rate := float64(currentProcessed) / elapsed

				// Calculate remaining checkpoints
				remaining := totalCheckpoints - currentProcessed

				// Calculate ETA (only if we have a meaningful rate)
				if rate > 0 && remaining > 0 {
					eta := int64(float64(remaining) / rate)
					etaSeconds = &eta
				}
			}
		}
	}

	// Convert to response
	response := ScanStatusResponse{
		JobID:          job.ID,
		Status:         string(job.Status),
		Progress:       progress,
		FilesFound:     job.FilesFound,
		FilesProcessed: filesProcessed,
		BytesProcessed: job.BytesProcessed,
		ErrorCount:     errorCount,
		WarningCount:   warningCount,
		ErrorMessage:   job.ErrorMessage,
		StartedAt:      job.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		Phase:          string(job.Phase),
		EstimatedTotal: job.EstimatedTotal,
		DiscoveryDone:  job.DiscoveryDone,
		ETASeconds:     etaSeconds,

		// Discovery health metrics
		DiscoveryErrors:   job.DiscoveryErrors,
		DiscoveryWarnings: job.DiscoveryWarnings,
		DirsScanned:       job.DirsScanned,
		DirsSkipped:       job.DirsSkipped,
		FilesSkipped:      job.FilesSkipped,
	}

	if job.CompletedAt != nil {
		completedAtStr := job.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completedAtStr
	}

	c.JSON(http.StatusOK, response)
}

// GetHistory returns the scan history for a library
// @Summary Get scan history
// @Description Returns the last N scan jobs for a library, ordered by creation date (newest first)
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param limit query int false "Number of scan jobs to return (default 10, max 100)" default(10)
// @Success 200 {object} ScanHistoryResponse "Scan history"
// @Failure 400 {object} ErrorResponse "Invalid library ID or limit"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/history [get]
func (h *ScanJobHandler) GetHistory(c *gin.Context) {
	// Parse library ID
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}

	// Parse limit (default 10, max 100)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 || parsedLimit > 100 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid limit (must be between 1 and 100)"})
			return
		}
		limit = parsedLimit
	}

	// Get scan history
	jobs, err := h.scanJobRepo.ListByLibrary(c.Request.Context(), libraryID, int32(limit))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get scan history"})
		return
	}

	// Convert to response
	historyItems := make([]ScanHistoryItem, len(jobs))
	for i, job := range jobs {
		item := ScanHistoryItem{
			ID:             job.ID,
			Status:         string(job.Status),
			Progress:       job.Progress,
			FilesFound:     job.FilesFound,
			FilesProcessed: job.FilesProcessed,
			BytesProcessed: job.BytesProcessed,
			ErrorCount:     job.ErrorCount,
			WarningCount:   job.WarningCount,
			ErrorMessage:   job.ErrorMessage,
			StartedAt:      job.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if job.CompletedAt != nil {
			completedAtStr := job.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			item.CompletedAt = &completedAtStr

			// Calculate duration in seconds
			duration := int64(job.CompletedAt.Sub(job.StartedAt).Seconds())
			item.Duration = &duration
		}

		historyItems[i] = item
	}

	c.JSON(http.StatusOK, ScanHistoryResponse{Jobs: historyItems})
}

// StreamProgress streams real-time scan progress updates using Server-Sent Events (SSE)
// @Summary Stream scan progress
// @Description Streams real-time progress updates for an active scan job using Server-Sent Events
// @Tags scans
// @Accept json
// @Produce text/event-stream
// @Param id path int true "Library ID"
// @Success 200 {string} string "SSE stream of progress updates"
// @Failure 400 {object} ErrorResponse "Invalid library ID"
// @Failure 404 {object} ErrorResponse "No active scan found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/stream [get]
func (h *ScanJobHandler) StreamProgress(c *gin.Context) {
	// Parse library ID
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create a ticker for polling progress updates
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Track last known status to detect completion
	var lastStatus scanner.ScanStatus

	// Create a done channel for cleanup
	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			// Client disconnected
			return
		case <-ticker.C:
			// Get current scan status
			job, err := h.scanJobRepo.GetLatestByLibrary(c.Request.Context(), libraryID)
			if err != nil {
				if err == scanner.ErrNotFound {
					// No scan found - send error and close
					fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"No scan found for this library\"}\n\n")
					c.Writer.(http.Flusher).Flush()
					return
				}
				// Other error - send error and close
				fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"Failed to get scan status\"}\n\n")
				c.Writer.(http.Flusher).Flush()
				return
			}

			// Send progress update
			progressData := fmt.Sprintf(
				`{"status":"%s","progress":%.2f,"files_found":%d,"files_processed":%d,"bytes_processed":%d,"error_count":%d,"warning_count":%d}`,
				job.Status,
				job.Progress,
				job.FilesFound,
				job.FilesProcessed,
				job.BytesProcessed,
				job.ErrorCount,
				job.WarningCount,
			)
			fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", progressData)
			c.Writer.(http.Flusher).Flush()

			// Check if scan completed or failed
			if job.Status == scanner.ScanStatusCompleted || job.Status == scanner.ScanStatusFailed {
				if lastStatus != job.Status {
					// Status just changed - send completion event
					completionData := fmt.Sprintf(
						`{"status":"%s","progress":%.2f,"files_found":%d,"files_processed":%d,"error_message":"%s"}`,
						job.Status,
						job.Progress,
						job.FilesFound,
						job.FilesProcessed,
						job.ErrorMessage,
					)
					fmt.Fprintf(c.Writer, "event: complete\ndata: %s\n\n", completionData)
					c.Writer.(http.Flusher).Flush()
				}
				// Close stream after completion
				return
			}

			lastStatus = job.Status
		}
	}
}

// ScanErrorDetail represents a single file processing error or warning
type ScanErrorDetail struct {
	FilePath      string  `json:"file_path"`
	Status        string  `json:"status"`           // "failed" or "warning"
	ErrorMessage  string  `json:"error_message"`
	ErrorCategory string  `json:"error_category"`
	FileSize      int64   `json:"file_size"`
	ProcessedAt   *string `json:"processed_at,omitempty"`
}

// ScanErrorsResponse represents scan errors grouped by category
type ScanErrorsResponse struct {
	TotalErrors int                            `json:"total_errors"`
	ByCategory  map[string][]ScanErrorDetail   `json:"by_category"`
}

// RetryFailedResponse represents the result of retrying failed files
type RetryFailedResponse struct {
	Message string `json:"message"`
	Count   int64  `json:"count"`
}

// GetScanErrors returns all failed files for a scan job, grouped by error category
// @Summary Get scan errors
// @Description Returns all files that failed during scanning, grouped by error category
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param jobId path int true "Scan Job ID"
// @Success 200 {object} ScanErrorsResponse "Scan errors"
// @Failure 400 {object} ErrorResponse "Invalid library ID or job ID"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/{jobId}/errors [get]
func (h *ScanJobHandler) GetScanErrors(c *gin.Context) {
	// Parse library ID (for consistency, though not strictly needed)
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}
	_ = libraryID // Not used but validates URL structure

	// Parse job ID
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid job ID"})
		return
	}

	// Get failed checkpoints (limit to 1000 to prevent memory issues)
	failed, err := h.checkpointRepo.ListFailed(c.Request.Context(), jobID, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve scan errors"})
		return
	}

	// Group errors by category
	errorsByCategory := make(map[string][]ScanErrorDetail)
	for _, checkpoint := range failed {
		category := string(checkpoint.ErrorCategory)
		if category == "" {
			category = "unknown"
		}

		var processedAtStr *string
		if checkpoint.ProcessedAt != nil {
			str := checkpoint.ProcessedAt.Format("2006-01-02T15:04:05Z07:00")
			processedAtStr = &str
		}

		errorsByCategory[category] = append(
			errorsByCategory[category],
			ScanErrorDetail{
				FilePath:      checkpoint.FilePath,
				Status:        string(checkpoint.Status),
				ErrorMessage:  checkpoint.ErrorMessage,
				ErrorCategory: category,
				FileSize:      checkpoint.FileSize,
				ProcessedAt:   processedAtStr,
			},
		)
	}

	c.JSON(http.StatusOK, ScanErrorsResponse{
		TotalErrors: len(failed),
		ByCategory:  errorsByCategory,
	})
}

// GetLibraryIssues returns all persistent warnings and errors for a library from scan_state
// This endpoint shows current library health, persisting across scans until files are re-scanned successfully
// @Summary Get library warnings and errors
// @Description Returns all files with warnings or errors from scan_state (persistent across scans)
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Success 200 {object} ScanErrorsResponse "Library issues"
// @Failure 400 {object} ErrorResponse "Invalid library ID"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/issues [get]
func (h *ScanJobHandler) GetLibraryIssues(c *gin.Context) {
	// Parse library ID
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}

	// Get all files with warnings or errors from scan_state
	issues, err := h.scanStateRepo.GetLibraryIssues(c.Request.Context(), libraryID)
	if err != nil {
		h.logger.Error("failed to get library issues",
			"library_id", libraryID,
			"error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve library issues"})
		return
	}

	// Group issues by category (use error category if present, otherwise warning category)
	issuesByCategory := make(map[string][]ScanErrorDetail)
	for _, state := range issues {
		category := "unknown"
		message := ""
		status := "warning" // default to warning

		if state.HasError {
			status = "failed"
			category = state.ErrorCategory
			message = state.ErrorMessage
		} else if state.HasWarning {
			status = "warning"
			category = state.WarningCategory
			message = state.WarningMessage
		}

		if category == "" {
			category = "unknown"
		}

		var processedAtStr *string
		if !state.LastScannedAt.IsZero() {
			str := state.LastScannedAt.Format("2006-01-02T15:04:05Z07:00")
			processedAtStr = &str
		}

		issuesByCategory[category] = append(
			issuesByCategory[category],
			ScanErrorDetail{
				FilePath:      state.FilePath,
				Status:        status,
				ErrorMessage:  message,
				ErrorCategory: category,
				FileSize:      state.FileSize,
				ProcessedAt:   processedAtStr,
			},
		)
	}

	c.JSON(http.StatusOK, ScanErrorsResponse{
		TotalErrors: len(issues),
		ByCategory:  issuesByCategory,
	})
}

// RetryFailedFiles resets all failed checkpoints to pending, allowing them to be retried
// @Summary Retry failed files
// @Description Resets all failed file processing attempts to pending status for retry
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param jobId path int true "Scan Job ID"
// @Success 200 {object} RetryFailedResponse "Retry result"
// @Failure 400 {object} ErrorResponse "Invalid library ID or job ID"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/{jobId}/retry-failed [post]
func (h *ScanJobHandler) RetryFailedFiles(c *gin.Context) {
	// Parse library ID (for consistency)
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}
	_ = libraryID // Not used but validates URL structure

	// Parse job ID
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid job ID"})
		return
	}

	// Reset failed checkpoints to pending
	count, err := h.checkpointRepo.ResetFailed(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to reset failed files"})
		return
	}

	c.JSON(http.StatusOK, RetryFailedResponse{
		Message: "Failed files queued for retry",
		Count:   count,
	})
}

// PauseScanResponse represents the result of pausing a scan
type PauseScanResponse struct {
	Message string `json:"message"`
	JobID   int64  `json:"job_id"`
}

// PauseScan pauses a running scan job
// @Summary Pause scan
// @Description Pauses an active scan job, allowing it to be resumed later
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param jobId path int true "Scan Job ID"
// @Success 200 {object} PauseScanResponse "Scan paused successfully"
// @Failure 400 {object} ErrorResponse "Invalid library ID or job ID"
// @Failure 404 {object} ErrorResponse "Scan job not found"
// @Failure 409 {object} ErrorResponse "Scan is not running"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/{jobId}/pause [post]
func (h *ScanJobHandler) PauseScan(c *gin.Context) {
	// Parse library ID (for consistency)
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}
	_ = libraryID // Not used but validates URL structure

	// Parse job ID
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid job ID"})
		return
	}

	// Get the scan job
	job, err := h.scanJobRepo.GetByID(c.Request.Context(), jobID)
	if err != nil {
		if err == scanner.ErrNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Scan job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get scan job"})
		return
	}

	// Check if scan is running
	if job.Status != scanner.ScanStatusRunning {
		c.JSON(http.StatusConflict, ErrorResponse{Error: fmt.Sprintf("Scan is not running (current status: %s)", job.Status)})
		return
	}

	// Update status to paused
	if err := h.scanJobRepo.UpdateStatus(c.Request.Context(), jobID, scanner.ScanStatusPaused); err != nil {
		h.logger.Error("failed to pause scan",
			"job_id", jobID,
			"library_id", libraryID,
			"error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to pause scan"})
		return
	}

	h.logger.Info("scan paused by user",
		"job_id", jobID,
		"library_id", libraryID,
		"files_processed", job.FilesProcessed,
		"files_found", job.FilesFound,
		"progress", job.Progress)

	c.JSON(http.StatusOK, PauseScanResponse{
		Message: "Scan paused successfully",
		JobID:   jobID,
	})
}

// ResumeScanResponse represents the result of resuming a scan
type ResumeScanResponse struct {
	Message string `json:"message"`
	JobID   int64  `json:"job_id"`
}

// ResumeScan resumes a paused scan job
// @Summary Resume scan
// @Description Resumes a paused scan job, continuing from where it left off
// @Tags scans
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param jobId path int true "Scan Job ID"
// @Success 200 {object} ResumeScanResponse "Scan resumed successfully"
// @Failure 400 {object} ErrorResponse "Invalid library ID or job ID"
// @Failure 404 {object} ErrorResponse "Scan job not found"
// @Failure 409 {object} ErrorResponse "Scan is not paused"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/libraries/{id}/scan/{jobId}/resume [post]
func (h *ScanJobHandler) ResumeScan(c *gin.Context) {
	// Parse library ID (for consistency)
	libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid library ID"})
		return
	}
	_ = libraryID // Not used but validates URL structure

	// Parse job ID
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid job ID"})
		return
	}

	// Resume the scan via the use case (which will start the background goroutine)
	if err := h.scanLibrary.ResumeScan(c.Request.Context(), jobID); err != nil {
		h.logger.Error("failed to resume scan",
			"job_id", jobID,
			"library_id", libraryID,
			"error", err)

		// Check error type for appropriate status code
		if err.Error() == "scan job is not paused" || err.Error() == fmt.Sprintf("scan job is not paused (current status: %s)", "") {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to resume scan"})
		return
	}

	h.logger.Info("scan resumed by user",
		"job_id", jobID,
		"library_id", libraryID)

	c.JSON(http.StatusOK, ResumeScanResponse{
		Message: "Scan resumed successfully",
		JobID:   jobID,
	})
}
