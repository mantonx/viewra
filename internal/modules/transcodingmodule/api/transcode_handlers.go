package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	plugins "github.com/mantonx/viewra/sdk"
)

// StartTranscode handles POST /api/v1/transcoding/transcode
//
// Request body:
//
//	{
//	  "mediaId": "string",      // Required: ID of the media to transcode
//	  "container": "string",    // Required: Target container format (mp4, mkv)
//	  "inputPath": "string",    // Required: Path to source media file
//	  "outputPath": "string",   // Optional: Desired output location
//	  "encodingOptions": {...}  // Optional: Encoding parameters
//	}
//
// Response:
//
//	{
//	  "sessionId": "string",    // Unique session identifier
//	  "status": "string",       // Current status (queued, running, etc.)
//	  "provider": "string"      // Provider handling the transcode
//	}
func (h *APIHandler) StartTranscode(c *gin.Context) {
	var req plugins.TranscodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if req.MediaID == "" || req.InputPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "mediaId and inputPath are required",
		})
		return
	}

	// Set defaults if not provided
	if req.Container == "" {
		req.Container = "mp4"
	}

	logger.Info("Starting transcode request",
		"mediaId", req.MediaID,
		"container", req.Container,
		"input", req.InputPath)

	// Start transcoding
	handle, err := h.service.StartTranscode(c.Request.Context(), req)
	if err != nil {
		logger.Error("Failed to start transcode: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transcoding",
			"details": err.Error(),
		})
		return
	}

	// Return handle information
	c.JSON(http.StatusAccepted, gin.H{
		"sessionId": handle.SessionID,
		"status":    string(handle.Status),
		"provider":  handle.Provider,
		"message":   "Transcoding started successfully",
	})
}