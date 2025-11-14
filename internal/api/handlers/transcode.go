package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/application/transcode"
	transcodeDomain "github.com/viewra/viewra/internal/domain/transcode"
)

// TranscodeHandler handles transcode-related HTTP requests.
type TranscodeHandler struct {
	createJobUseCase   *transcode.CreateJobUseCase
	getStatusUseCase   *transcode.GetJobStatusUseCase
	serveManifestUseCase *transcode.ServeManifestUseCase
	queue              *transcode.Queue
	outputDir          string
}

// NewTranscodeHandler creates a new transcode handler.
func NewTranscodeHandler(
	createJobUseCase *transcode.CreateJobUseCase,
	getStatusUseCase *transcode.GetJobStatusUseCase,
	serveManifestUseCase *transcode.ServeManifestUseCase,
	queue *transcode.Queue,
	outputDir string,
) *TranscodeHandler {
	return &TranscodeHandler{
		createJobUseCase:   createJobUseCase,
		getStatusUseCase:   getStatusUseCase,
		serveManifestUseCase: serveManifestUseCase,
		queue:              queue,
		outputDir:          outputDir,
	}
}

// CreateTranscodeJobRequest represents a request to start transcoding.
type CreateTranscodeJobRequest struct {
	Quality string `json:"quality" binding:"required"`
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
		MediaID: mediaID,
		Quality: req.Quality,
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

// OnDemandResponse represents the response for on-demand transcoding requests.
type OnDemandResponse struct {
	Strategy      string `json:"strategy"`                 // direct_play, remux, remux_audio, or transcode
	URL           string `json:"url,omitempty"`            // For direct_play
	JobID         int64  `json:"job_id,omitempty"`         // For processing strategies
	Status        string `json:"status,omitempty"`         // Job status
	Progress      int    `json:"progress,omitempty"`       // Job progress (0-100)
	EstimatedTime string `json:"estimated_time,omitempty"` // Estimated completion time
}

// ServePlaylist serves the HLS playlist file for a transcoded media item with on-demand transcoding support.
//
// @Summary Serve HLS playlist (with on-demand transcoding)
// @Description Serves the HLS playlist (.m3u8) file for adaptive streaming. If playlist doesn't exist, analyzes video
// @Description and either redirects to direct stream or creates a transcode job.
// @Tags transcode
// @Produce application/vnd.apple.mpegurl,application/json
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {file} file "HLS playlist file (if exists)"
// @Success 302 "Redirect to direct stream (for compatible files)"
// @Success 202 {object} OnDemandResponse "Job created (for files needing processing)"
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

	// Use the serve manifest use case
	response, err := h.serveManifestUseCase.Execute(c.Request.Context(), transcode.ServeManifestRequest{
		MediaID:   mediaID,
		Quality:   quality,
		OutputDir: h.outputDir,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Handle response based on strategy
	switch response.Strategy {
	case transcode.StrategyServe:
		// Playlist exists - serve it directly
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Access-Control-Allow-Origin", "*") // CORS for HLS
		c.File(response.ManifestPath)

	case transcode.StrategyDirectPlay:
		// Video is compatible - redirect to direct stream
		// Frontend expects 302 redirect for direct play
		c.Redirect(http.StatusFound, response.DirectPlayURL)

	case transcode.StrategyTranscode:
		// Transcode needed - return job information
		c.JSON(http.StatusAccepted, OnDemandResponse{
			Strategy:      response.Job.Type,
			JobID:         response.Job.ID,
			Status:        response.Job.Status,
			Progress:      response.Job.Progress,
			EstimatedTime: response.EstimatedTime,
		})
	}
}

// ServeHLSSegment serves HLS segment files (MPEG-TS segments).
//
// @Summary Serve HLS segment
// @Description Serves HLS segment files (.ts) for adaptive streaming
// @Tags transcode
// @Produce video/mp2t
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Param filename path string true "Segment filename"
// @Success 200 {file} file "HLS segment file"
// @Failure 404 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/hls/{quality}/{filename} [get]
func (h *TranscodeHandler) ServeHLSSegment(c *gin.Context) {
	mediaIDStr := c.Param("id")
	quality := c.Param("quality")
	filename := c.Param("filename")

	// Record access to prevent idle timeout
	mediaID, err := parseID(mediaIDStr)
	if err == nil {
		h.queue.RecordAccess(mediaID, quality)
	}

	// Build segment path with hls/ subdirectory
	segmentPath := filepath.Join(h.outputDir, "hls", mediaIDStr, quality, filename)

	// Serve the file
	c.Header("Content-Type", "video/mp2t")
	c.Header("Access-Control-Allow-Origin", "*") // CORS for HLS
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
