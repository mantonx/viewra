package handlers

import (
	"fmt"
	"log/slog"
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
		Codec:         req.Codec,
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
	MediaID        *int64  `json:"media_id"`
	Quality        *string `json:"quality"`
	Failed         bool    `json:"failed"`
	Orphans        bool    `json:"orphans"`
	OlderThanHours *int    `json:"older_than_hours"`
	DryRun         bool    `json:"dry_run"`
}

// CleanupResponse represents cleanup operation results
type CleanupResponse struct {
	DeletedCount     int      `json:"deleted_count"`
	DeletedSizeBytes int64    `json:"deleted_size_bytes"`
	DeletedSizeHuman string   `json:"deleted_size_human"`
	FailedCount      int      `json:"failed_count"`
	Errors           []string `json:"errors,omitempty"`
	DryRun           bool     `json:"dry_run"`
}

// DiskUsageResponse represents disk usage statistics
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
		OutputDir:       usage.OutputDir,
		TotalSizeBytes:  usage.TotalSizeBytes,
		TotalSizeHuman:  format.Bytes(usage.TotalSizeBytes),
		FileCount:       usage.FileCount,
		TotalJobs:       usage.TotalJobs,
		CompletedCount:  usage.CompletedCount,
		FailedCount:     usage.FailedCount,
		QueuedCount:     usage.QueuedCount,
		ProcessingCount: usage.ProcessingCount,
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

// MasterPlaylistResponse is returned when master playlist generation fails but we have metadata
type MasterPlaylistResponse struct {
	MediaID            int64    `json:"media_id"`
	AvailableQualities []string `json:"available_qualities"`
	Error              string   `json:"error,omitempty"`
}

// ServeMasterPlaylist serves an HLS master playlist with all available quality variants.
// This enables adaptive bitrate streaming where the player can switch between quality levels.
//
// @Summary Serve HLS master playlist
// @Description Serves an HLS master playlist (.m3u8) that lists all available quality levels for adaptive streaming.
// @Description The player uses this to select and switch between quality levels based on network conditions.
// @Tags transcode
// @Produce application/vnd.apple.mpegurl,application/json
// @Param media_id path int true "Media ID"
// @Param start query number false "Start position in seconds for seeking"
// @Success 200 {file} file "HLS master playlist with all quality variants"
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/hls/master.m3u8 [get]
func (h *TranscodeHandler) ServeMasterPlaylist(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	// Parse optional start position query parameter for seeking
	startPosition := ""
	if startStr := c.Query("start"); startStr != "" {
		startPosition = startStr
	}

	// Get media info to determine source resolution and properties
	mediaItem, err := h.mediaRepo.GetByID(c.Request.Context(), mediaID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Media not found"})
		return
	}

	// Build list of quality profiles appropriate for this media
	// Filter based on source resolution (don't upscale)
	sourceHeight := mediaItem.Height
	sourceWidth := mediaItem.Width

	// Log source resolution for debugging
	slog.Debug("building master playlist",
		"media_id", mediaID,
		"source_width", sourceWidth,
		"source_height", sourceHeight,
	)

	// Default ABR ladder - a subset of profiles that work well together
	// These are selected to provide good coverage without too many variants
	abrLadder := []struct {
		quality   string
		bandwidth int
		width     int
		height    int
		codecs    string
	}{
		// Start with lower qualities for poor connections
		{"360p", 800_000, 640, 360, "avc1.4d401e,mp4a.40.2"},
		{"480p", 1_800_000, 854, 480, "avc1.4d401e,mp4a.40.2"},
		{"720p", 4_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
		{"1080p", 8_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"4k", 25_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	}

	// Build master playlist
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:4\n"
	playlist += "#EXT-X-INDEPENDENT-SEGMENTS\n\n"

	// Filter qualities based on source resolution
	for _, variant := range abrLadder {
		// Skip qualities higher than source ONLY if we know the source resolution
		// If source resolution is unknown (0), include all qualities up to 1080p as a safe default
		if sourceHeight > 0 && sourceWidth > 0 {
			// Skip qualities higher than source
			if variant.height > sourceHeight {
				continue
			}
			// Also check width
			if variant.width > sourceWidth {
				continue
			}
		} else {
			// Source resolution unknown - include up to 1080p as safe default
			// This prevents offering 4K when we don't know if the source supports it
			if variant.height > 1080 {
				slog.Debug("skipping quality (unknown source resolution)",
					"quality", variant.quality,
					"variant_height", variant.height,
				)
				continue
			}
		}

		// Add variant stream
		playlist += fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",NAME=\"%s\"\n",
			variant.bandwidth,
			variant.width,
			variant.height,
			variant.codecs,
			variant.quality,
		)

		// Variant stream URL
		variantURL := fmt.Sprintf("%s/playlist.m3u8", variant.quality)
		if startPosition != "" {
			variantURL += "?start=" + startPosition
		}
		playlist += variantURL + "\n\n"
	}

	// Set headers for HLS
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}
