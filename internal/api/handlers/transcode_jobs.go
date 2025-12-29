package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/transcode"
	transcodeDomain "github.com/mantonx/viewra/internal/domain/transcode"
)

// CreateTranscodeJob creates and enqueues a new transcode job.
//
// @Summary Start transcoding
// @Description Creates a new transcode job for the specified media and quality level
// @Tags transcode
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param request body CreateTranscodeJobRequest true "Transcode request"
// @Success 201 {object} TranscodeJobResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse "Job already exists"
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{id}/transcode [post]
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
// @Param id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {object} TranscodeJobResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{id}/transcode/{quality} [get]
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

// CancelTranscodeJob cancels an actively processing transcode job.
//
// @Summary Cancel transcode job
// @Description Cancels an actively transcoding job (called when user pauses/stops video)
// @Tags transcode
// @Param id path int true "Media ID"
// @Param quality path string true "Quality level (360p, 720p, 1080p, 4k)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /api/media/{id}/transcode/{quality}/cancel [post]
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
