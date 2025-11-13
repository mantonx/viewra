package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/domain/scanner"
)

// ScanJobRepository defines the interface for scan job data access
type ScanJobRepository interface {
	GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error)
	ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error)
}

// ScanJobHandler handles scan job-related HTTP requests
type ScanJobHandler struct {
	scanJobRepo ScanJobRepository
}

// NewScanJobHandler creates a new scan job handler
func NewScanJobHandler(scanJobRepo ScanJobRepository) *ScanJobHandler {
	return &ScanJobHandler{
		scanJobRepo: scanJobRepo,
	}
}

// ScanStatusResponse represents the current status of a library scan
type ScanStatusResponse struct {
	Status         string  `json:"status"`                    // pending, running, paused, completed, failed
	Progress       float64 `json:"progress"`                  // 0-100
	FilesFound     int64   `json:"files_found"`               // Total files discovered
	FilesProcessed int64   `json:"files_processed"`           // Files processed so far
	BytesProcessed int64   `json:"bytes_processed"`           // Bytes processed
	ErrorCount     int64   `json:"error_count"`               // Number of errors encountered
	ErrorMessage   string  `json:"error_message,omitempty"`   // Error message if failed
	StartedAt      string  `json:"started_at"`                // ISO 8601 timestamp
	CompletedAt    *string `json:"completed_at,omitempty"`    // ISO 8601 timestamp
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

	// Convert to response
	response := ScanStatusResponse{
		Status:         string(job.Status),
		Progress:       job.Progress,
		FilesFound:     job.FilesFound,
		FilesProcessed: job.FilesProcessed,
		BytesProcessed: job.BytesProcessed,
		ErrorCount:     job.ErrorCount,
		ErrorMessage:   job.ErrorMessage,
		StartedAt:      job.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
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
				`{"status":"%s","progress":%.2f,"files_found":%d,"files_processed":%d,"bytes_processed":%d,"error_count":%d}`,
				job.Status,
				job.Progress,
				job.FilesFound,
				job.FilesProcessed,
				job.BytesProcessed,
				job.ErrorCount,
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
