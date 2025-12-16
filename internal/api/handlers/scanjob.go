package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/application/scanjob"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// ScanLibraryExecutor defines the interface for resuming scans.
type ScanLibraryExecutor interface {
	ResumeScan(ctx context.Context, jobID int64) error
}

// ScanStatusProvider defines the interface for getting enriched scan status.
type ScanStatusProvider interface {
	GetScanStatus(ctx context.Context, libraryID int64) (*status.Result, error)
}

// ScanJobHandler handles scan job-related HTTP requests.
type ScanJobHandler struct {
	service        *scanjob.Service
	scanLibrary    ScanLibraryExecutor
	statusProvider ScanStatusProvider
	eventBus       *events.Bus
	logger         *slog.Logger
}

// NewScanJobHandler creates a new scan job handler.
func NewScanJobHandler(
	service *scanjob.Service,
	scanLibrary ScanLibraryExecutor,
	statusProvider ScanStatusProvider,
	eventBus *events.Bus,
	logger *slog.Logger,
) *ScanJobHandler {
	return &ScanJobHandler{
		service:        service,
		scanLibrary:    scanLibrary,
		statusProvider: statusProvider,
		eventBus:       eventBus,
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

	// Get enriched scan status from application layer
	status, err := h.statusProvider.GetScanStatus(c.Request.Context(), libraryID)
	if err != nil {
		if err == scanner.ErrNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "No scan jobs found for this library"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get scan status"})
		return
	}

	// Convert to HTTP response
	response := ScanStatusResponse{
		JobID:          status.JobID,
		Status:         string(status.Status),
		Progress:       status.Progress,
		FilesFound:     status.FilesFound,
		FilesProcessed: status.FilesProcessed,
		BytesProcessed: status.BytesProcessed,
		ErrorCount:     status.ErrorCount,
		WarningCount:   status.WarningCount,
		ErrorMessage:   status.ErrorMessage,
		StartedAt:      status.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		Phase:          string(status.Phase),
		EstimatedTotal: status.EstimatedTotal,
		DiscoveryDone:  status.DiscoveryDone,
		ETASeconds:     status.ETASeconds,

		// Discovery health metrics
		DiscoveryErrors:   status.DiscoveryErrors,
		DiscoveryWarnings: status.DiscoveryWarnings,
		DirsScanned:       status.DirsScanned,
		DirsSkipped:       status.DirsSkipped,
		FilesSkipped:      status.FilesSkipped,
	}

	if status.CompletedAt != nil {
		completedAtStr := status.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
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
	jobs, err := h.service.ListByLibrary(c.Request.Context(), libraryID, int32(limit))
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

	// Send initial state immediately
	initialStatus, err := h.statusProvider.GetScanStatus(c.Request.Context(), libraryID)
	if err != nil {
		if err == scanner.ErrNotFound {
			fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"No scan found for this library\"}\n\n")
			c.Writer.(http.Flusher).Flush()
			return
		}
		fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"Failed to get scan status\"}\n\n")
		c.Writer.(http.Flusher).Flush()
		return
	}

	initialData := h.buildScanProgressData(initialStatus)
	jsonData, _ := json.Marshal(initialData)
	fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
	c.Writer.(http.Flusher).Flush()

	// If scan is already completed/failed, close the stream
	if initialStatus.Status == scanner.ScanStatusCompleted || initialStatus.Status == scanner.ScanStatusFailed {
		return
	}

	// Subscribe to scan events
	sub := h.eventBus.Subscribe(
		events.WithBufferSize(100),
		events.WithEventPrefix("scan."),
	)
	defer h.eventBus.Unsubscribe(sub)

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			h.logger.Debug("SSE client disconnected", "library_id", libraryID)
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}

			// Filter events by library ID
			// Handle both int64 (direct) and float64 (from JSON unmarshal) types
			var eventLibraryID int64
			switch v := event.Data["library_id"].(type) {
			case int64:
				eventLibraryID = v
			case float64:
				eventLibraryID = int64(v)
			case int:
				eventLibraryID = int64(v)
			default:
				// Skip events without valid library_id
				continue
			}
			if eventLibraryID != libraryID {
				continue
			}

			// Send the event
			jsonData, err := json.Marshal(event.Data)
			if err != nil {
				h.logger.Error("Failed to marshal scan event", "error", err)
				continue
			}

			// Map event type to SSE event name
			eventName := "update"
			if event.Type == events.EventScanCompleted || event.Type == events.EventScanFailed {
				eventName = "complete"
			}

			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventName, jsonData)
			c.Writer.(http.Flusher).Flush()

			// Close stream on completion
			if event.Type == events.EventScanCompleted || event.Type == events.EventScanFailed {
				return
			}
		}
	}
}

// buildScanProgressData converts status.Result to a map for JSON serialization.
func (h *ScanJobHandler) buildScanProgressData(s *status.Result) map[string]any {
	data := map[string]any{
		"job_id":            s.JobID,
		"status":            string(s.Status),
		"progress":          s.Progress,
		"phase":             string(s.Phase),
		"files_found":       s.FilesFound,
		"files_processed":   s.FilesProcessed,
		"error_count":       s.ErrorCount,
		"warning_count":     s.WarningCount,
		"estimated_total":   s.EstimatedTotal,
		"discovery_done":    s.DiscoveryDone,
		"discovery_errors":  s.DiscoveryErrors,
		"discovery_warnings": s.DiscoveryWarnings,
		"dirs_scanned":      s.DirsScanned,
		"dirs_skipped":      s.DirsSkipped,
		"files_skipped":     s.FilesSkipped,
	}

	if s.ETASeconds != nil {
		data["eta_seconds"] = *s.ETASeconds
	}

	if s.ErrorMessage != "" {
		data["error_message"] = s.ErrorMessage
	}

	return data
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
	failed, err := h.service.ListFailed(c.Request.Context(), jobID, 1000)
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
	issues, err := h.service.GetLibraryIssues(c.Request.Context(), libraryID)
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
	count, err := h.service.ResetFailed(c.Request.Context(), jobID)
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
	job, err := h.service.GetByID(c.Request.Context(), jobID)
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
	if err := h.service.UpdateStatus(c.Request.Context(), jobID, scanner.ScanStatusPaused); err != nil {
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
