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
	repo      transcodeDomain.Repository
	queue     *transcode.Queue
	outputDir string
}

// NewTranscodeHandler creates a new transcode handler.
func NewTranscodeHandler(repo transcodeDomain.Repository, queue *transcode.Queue, outputDir string) *TranscodeHandler {
	return &TranscodeHandler{
		repo:      repo,
		queue:     queue,
		outputDir: outputDir,
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
	mediaIDStr := c.Param("media_id")
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

	// Create the job
	job, err := transcode.CreateJob(c.Request.Context(), h.repo, transcode.CreateJobRequest{
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

	// Enqueue the job for processing
	if h.queue != nil {
		if err := h.queue.EnqueueJob(job); err != nil {
			// Job created but failed to enqueue - it will be picked up by the poller
			// Log the error but don't fail the request
		}
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
	mediaIDStr := c.Param("media_id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	quality := c.Param("quality")

	job, err := transcode.GetJobForMedia(c.Request.Context(), h.repo, mediaID, quality)
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

// ListTranscodeJobs lists all transcode jobs for a media item.
//
// @Summary List transcode jobs for media
// @Description Gets all transcode jobs for a specific media item
// @Tags transcode
// @Produce json
// @Param media_id path int true "Media ID"
// @Success 200 {array} TranscodeJobResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/transcode [get]
func (h *TranscodeHandler) ListTranscodeJobs(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	mediaID, err := parseID(mediaIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid media ID"})
		return
	}

	// Get jobs for all qualities for this media
	var allJobs []*transcodeDomain.TranscodeJob
	for _, quality := range transcodeDomain.GetAllQualities() {
		job, err := transcode.GetJobForMedia(c.Request.Context(), h.repo, mediaID, quality)
		if err == nil && job != nil {
			allJobs = append(allJobs, job)
		}
	}

	responses := make([]TranscodeJobResponse, len(allJobs))
	for i, job := range allJobs {
		responses[i] = toTranscodeJobResponse(job)
	}

	c.JSON(http.StatusOK, responses)
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

// ServeManifest serves the DASH manifest file for a transcoded media item.
//
// @Summary Serve DASH manifest
// @Description Serves the DASH manifest (.mpd) file for adaptive streaming
// @Tags transcode
// @Produce application/dash+xml
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {file} file "DASH manifest file"
// @Failure 404 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/dash/{quality}/manifest.mpd [get]
func (h *TranscodeHandler) ServeManifest(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	quality := c.Param("quality")

	// Build manifest path
	manifestPath := filepath.Join(h.outputDir, mediaIDStr, quality, "manifest.mpd")

	// Serve the file
	c.Header("Content-Type", "application/dash+xml")
	c.Header("Access-Control-Allow-Origin", "*") // CORS for DASH
	c.File(manifestPath)
}

// ServeDASHSegment serves DASH segment files (init and media segments).
//
// @Summary Serve DASH segment
// @Description Serves DASH segment files (.m4s) for adaptive streaming
// @Tags transcode
// @Produce application/octet-stream
// @Param media_id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Param filename path string true "Segment filename"
// @Success 200 {file} file "DASH segment file"
// @Failure 404 {object} handlers.ErrorResponse
// @Router /api/media/{media_id}/dash/{quality}/{filename} [get]
func (h *TranscodeHandler) ServeDASHSegment(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	quality := c.Param("quality")
	filename := c.Param("filename")

	// Build segment path
	segmentPath := filepath.Join(h.outputDir, mediaIDStr, quality, filename)

	// Serve the file
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Access-Control-Allow-Origin", "*") // CORS for DASH
	c.File(segmentPath)
}

// toTranscodeJobResponse converts a domain transcode job to an API response.
func toTranscodeJobResponse(job *transcodeDomain.TranscodeJob) TranscodeJobResponse {
	response := TranscodeJobResponse{
		ID:        job.ID,
		MediaID:   job.MediaID,
		Quality:   job.Quality,
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
