package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/domain/media"
	transcodeDomain "github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
	"github.com/mantonx/viewra/internal/pkg/format"
)

// TranscodeHandler handles transcode-related HTTP requests.
type TranscodeHandler struct {
	createJobUseCase     *transcode.CreateJobUseCase
	getStatusUseCase     *transcode.GetJobStatusUseCase
	serveManifestUseCase *transcode.ServeManifestUseCase
	queue                *transcode.Queue
	cleanupService       *transcode.CleanupService
	sessionManager       *transcoding.SessionManager
	mediaRepo            media.Repository
	outputDir            string
}

// NewTranscodeHandler creates a new transcode handler.
func NewTranscodeHandler(
	createJobUseCase *transcode.CreateJobUseCase,
	getStatusUseCase *transcode.GetJobStatusUseCase,
	serveManifestUseCase *transcode.ServeManifestUseCase,
	queue *transcode.Queue,
	cleanupService *transcode.CleanupService,
	sessionManager *transcoding.SessionManager,
	mediaRepo media.Repository,
	outputDir string,
) *TranscodeHandler {
	return &TranscodeHandler{
		createJobUseCase:     createJobUseCase,
		getStatusUseCase:     getStatusUseCase,
		serveManifestUseCase: serveManifestUseCase,
		queue:                queue,
		cleanupService:       cleanupService,
		sessionManager:       sessionManager,
		mediaRepo:            mediaRepo,
		outputDir:            outputDir,
	}
}

// CreateTranscodeJobRequest represents a request to start transcoding.
type CreateTranscodeJobRequest struct {
	Quality       string `json:"quality" binding:"required"`
	StartPosition int    `json:"start_position,omitempty"` // Optional: start position in seconds for seek-based transcoding
}

// TranscodeJobResponse represents a transcode job response.
type TranscodeJobResponse struct {
	ID          int64  `json:"id"`
	MediaID     int64  `json:"media_id"`
	Quality     string `json:"quality"`
	Type        string `json:"type"` // Job type: remux, remux_audio, or transcode
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	Error       string `json:"error,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// CreateTranscodeJob creates and enqueues a new transcode job.
//
// @Summary Start transcoding
// @Description Creates a new transcode job for the specified media and quality level
// @Tags transcode
// @Accept json
// @Produce json
// @Param media_id path int true "Media ID"
// @Param request body CreateTranscodeJobRequest true "Transcode request"
// @Success 201 {object} TranscodeJobResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse "Job already exists"
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/transcode [post]
func (h *TranscodeHandler) CreateTranscodeJob(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	var req CreateTranscodeJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Use the create job use case
	job, err := h.createJobUseCase.Execute(c.Request.Context(), transcode.CreateJobRequest{
		MediaID:       mediaID,
		Quality:       req.Quality,
		StartPosition: req.StartPosition,
	})
	if err != nil {
		switch err {
		case transcodeDomain.ErrInvalidQuality:
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case transcodeDomain.ErrJobAlreadyExists:
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create transcode job"})
		}
		return
	}

	c.JSON(http.StatusCreated, toTranscodeJobResponse(job))
}

// GetTranscodeStatus gets the status of a transcode job.
//
// @Summary Get transcode status
// @Description Gets the status of a transcode job for specific media and quality
// @Tags transcode
// @Produce json
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {object} TranscodeJobResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/transcode/{quality} [get]
func (h *TranscodeHandler) GetTranscodeStatus(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	quality := c.Param("quality")

	// Use the get status use case
	job, err := h.getStatusUseCase.Execute(c.Request.Context(), transcode.GetJobStatusRequest{
		MediaID: mediaID,
		Quality: quality,
	})
	if err != nil {
		if err == transcodeDomain.ErrJobNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Transcode job not found"})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get transcode status"})
		}
		return
	}

	c.JSON(http.StatusOK, toTranscodeJobResponse(job))
}

// GetQueueStats gets statistics about the transcode queue.
//
// @Summary Get transcode queue statistics
// @Description Gets current statistics about the transcode job queue
// @Tags transcode
// @Produce json
// @Success 200 {object} transcode.QueueStats
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/transcode/stats [get]
func (h *TranscodeHandler) GetQueueStats(c *gin.Context) {
	if h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "Transcode queue not available"})
		return
	}

	stats, err := h.queue.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get queue stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ServePlaylist serves the HLS playlist file for a media item with on-demand segment generation.
//
// @Summary Serve HLS playlist (instant manifest generation)
// @Description Serves the HLS playlist (.m3u8) file for adaptive streaming. Generates complete manifest instantly
// @Description from segment 0. Segments are created on-demand as the player requests them. Compatible videos redirect to direct stream.
// @Tags transcode
// @Produce application/vnd.apple.mpegurl,application/json
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {file} file "HLS playlist file - segments generated on-demand"
// @Success 302 "Redirect to direct stream (for compatible files)"
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/hls/{quality}/playlist.m3u8 [get]
func (h *TranscodeHandler) ServePlaylist(c *gin.Context) {
	mediaIDStr := c.Param("id")
	quality := c.Param("quality")

	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	// Parse optional start position query parameter for seeking
	startPosition := 0.0
	if startStr := c.Query("start"); startStr != "" {
		if start, err := parseFloat(startStr); err == nil {
			startPosition = start
		}
	}

	// Use the serve manifest use case
	response, err := h.serveManifestUseCase.Execute(c.Request.Context(), transcode.ServeManifestRequest{
		MediaID:       mediaID,
		Quality:       quality,
		OutputDir:     h.outputDir,
		StartPosition: startPosition,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Handle response based on strategy
	switch response.Strategy {
	case transcode.StrategyServe:
		// Manifest generated - serve it directly
		// Segments will be generated on-demand as player requests them
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Access-Control-Allow-Origin", "*") // CORS for HLS
		c.File(response.ManifestPath)

	case transcode.StrategyDirectPlay:
		// Video is compatible - redirect to direct stream
		// Frontend expects 302 redirect for direct play
		c.Redirect(http.StatusFound, response.DirectPlayURL)

	default:
		// Should never reach here with new segment-based system
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Unknown streaming strategy"})
	}
}

// ServeHLSSegment serves HLS segment files from progressive transcode sessions.
//
// @Summary Serve HLS segment
// @Description Serves HLS segment files (.ts) from progressive transcoding sessions
// @Tags transcode
// @Produce video/mp2t
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Param filename path string true "Segment filename (e.g., seg_000123.ts)"
// @Success 200 {file} file "HLS segment file"
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/hls/{quality}/{filename} [get]
func (h *TranscodeHandler) ServeHLSSegment(c *gin.Context) {
	mediaIDStr := c.Param("id")
	quality := c.Param("quality")
	filename := c.Param("filename")

	// Parse media ID
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	// Parse segment number from filename
	segmentNum := transcoding.ParseSegmentNumber(filename)
	if segmentNum < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid segment filename"})
		return
	}

	// Get active transcode session
	session, err := h.sessionManager.GetSession(mediaID, quality)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "No active transcode session"})
		return
	}

	// Wait for segment to be generated (30 second timeout)
	segmentPath, err := session.WaitForSegment(segmentNum, 30*time.Second)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, ErrorResponse{
			Error: "Segment not available - transcoding may be slow or failed",
		})
		return
	}

	// Update session last accessed time
	session.UpdateLastAccessed()

	// Serve the segment
	c.Header("Content-Type", "video/mp2t")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(segmentPath)
}

// toTranscodeJobResponse converts a domain transcode job to an API response.
func toTranscodeJobResponse(job *transcodeDomain.TranscodeJob) TranscodeJobResponse {
	response := TranscodeJobResponse{
		ID:        job.ID,
		MediaID:   job.MediaID,
		Quality:   job.Quality,
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

// CancelTranscodeJob cancels an actively processing transcode job.
//
// @Summary Cancel transcode job
// @Description Cancels an actively transcoding job (called when user pauses/stops video)
// @Tags transcode
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/transcode/{quality}/cancel [post]
func (h *TranscodeHandler) CancelTranscodeJob(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	quality := c.Param("quality")

	// Cancel the job via the queue
	if err := h.queue.CancelJob(c.Request.Context(), mediaID, quality); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transcode job cancelled successfully"})
}

// CleanupRequest represents a transcode cleanup request
type CleanupRequest struct {
	MediaID     *int64  `json:"media_id"`
	Quality     *string `json:"quality"`
	Failed      bool    `json:"failed"`
	Orphans     bool    `json:"orphans"`
	OlderThanHours *int `json:"older_than_hours"`
	DryRun      bool    `json:"dry_run"`
}

// CleanupResponse represents cleanup operation results
type CleanupResponse struct {
	DeletedCount     int                `json:"deleted_count"`
	DeletedSizeBytes int64              `json:"deleted_size_bytes"`
	DeletedSizeHuman string             `json:"deleted_size_human"`
	FailedCount      int                `json:"failed_count"`
	Errors           []string           `json:"errors,omitempty"`
	DryRun           bool               `json:"dry_run"`
}

// DiskUsageResponse represents disk usage statistics
type DiskUsageResponse struct {
	OutputDir        string `json:"output_dir"`
	TotalSizeBytes   int64  `json:"total_size_bytes"`
	TotalSizeHuman   string `json:"total_size_human"`
	FileCount        int    `json:"file_count"`
	TotalJobs        int    `json:"total_jobs"`
	CompletedCount   int    `json:"completed_count"`
	FailedCount      int    `json:"failed_count"`
	QueuedCount      int    `json:"queued_count"`
	ProcessingCount  int    `json:"processing_count"`
}

// GetDiskUsage returns transcode disk usage statistics.
//
// @Summary Get disk usage
// @Description Returns current disk usage statistics for transcode files
// @Tags transcode
// @Produce json
// @Success 200 {object} DiskUsageResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/transcode/disk-usage [get]
func (h *TranscodeHandler) GetDiskUsage(c *gin.Context) {
	usage, err := h.cleanupService.GetDiskUsage(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, DiskUsageResponse{
		OutputDir:        usage.OutputDir,
		TotalSizeBytes:   usage.TotalSizeBytes,
		TotalSizeHuman:   format.Bytes(usage.TotalSizeBytes),
		FileCount:        usage.FileCount,
		TotalJobs:        usage.TotalJobs,
		CompletedCount:   usage.CompletedCount,
		FailedCount:      usage.FailedCount,
		QueuedCount:      usage.QueuedCount,
		ProcessingCount:  usage.ProcessingCount,
	})
}

// CleanupTranscodes performs cleanup of transcode files.
//
// @Summary Cleanup transcodes
// @Description Cleans up transcode files based on specified criteria
// @Tags transcode
// @Accept json
// @Produce json
// @Param request body CleanupRequest true "Cleanup criteria"
// @Success 200 {object} CleanupResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/transcode/cleanup [post]
func (h *TranscodeHandler) CleanupTranscodes(c *gin.Context) {
	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	var result *transcode.CleanupResult
	var err error

	if req.Orphans {
		result, err = h.cleanupService.CleanOrphans(c.Request.Context(), req.DryRun)
	} else if req.Failed {
		olderThan := 24 * time.Hour
		if req.OlderThanHours != nil {
			olderThan = time.Duration(*req.OlderThanHours) * time.Hour
		}
		result, err = h.cleanupService.CleanFailed(c.Request.Context(), olderThan, req.DryRun)
	} else if req.MediaID != nil {
		result, err = h.cleanupService.CleanByMediaID(c.Request.Context(), *req.MediaID, req.DryRun)
	} else if req.OlderThanHours != nil {
		olderThan := time.Duration(*req.OlderThanHours) * time.Hour
		result, err = h.cleanupService.CleanOld(c.Request.Context(), olderThan, req.DryRun)
	} else {
		// Custom filter
		filter := transcode.CleanupFilter{
			IncludeCompleted: true,
			IncludeFailed:    req.Failed,
			DryRun:           req.DryRun,
		}
		if req.Quality != nil {
			filter.Quality = req.Quality
		}
		result, err = h.cleanupService.Clean(c.Request.Context(), filter)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Convert errors to strings
	errorStrings := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errorStrings[i] = e.Error()
	}

	c.JSON(http.StatusOK, CleanupResponse{
		DeletedCount:     result.DeletedCount,
		DeletedSizeBytes: result.DeletedSizeBytes,
		DeletedSizeHuman: format.Bytes(result.DeletedSizeBytes),
		FailedCount:      result.FailedCount,
		Errors:           errorStrings,
		DryRun:           req.DryRun,
	})
}

