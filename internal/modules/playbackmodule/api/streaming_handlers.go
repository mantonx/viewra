// Package api - Media streaming handlers
package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/types"
)

// StreamSession handles GET /api/v1/playback/stream/:sessionId
// It streams content based on an active playback session.
// This is the recommended streaming method as it tracks session activity.
//
// Path parameters:
//   - sessionId: The active session ID
//
// Response: Media file stream with range support
func (h *Handler) StreamSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Get media file info
	mediaFile, err := h.mediaService.GetFile(c.Request.Context(), session.MediaFileID)
	if err != nil {
		h.logger.Error("Failed to get media file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get media file"})
		return
	}

	// Update session activity
	updates := map[string]interface{}{
		"last_activity": time.Now(),
	}
	h.sessionManager.UpdateSession(sessionID, updates)

	// Stream based on playback method
	switch types.PlaybackMethod(session.Method) {
	case types.PlaybackMethodDirect:
		// Direct play - serve the original file
		h.progressHandler.ServeFile(c, mediaFile.Path)
	case types.PlaybackMethodRemux:
		// TODO: Implement remuxing
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Remux streaming not yet implemented"})
	case types.PlaybackMethodTranscode:
		// TODO: Implement transcoding
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Transcode streaming not yet implemented"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid playback method"})
	}
}

// StreamFile handles GET /api/v1/playback/stream/file/:fileId
// It streams a media file by ID without requiring a session.
// This is useful for simple playback scenarios.
//
// Path parameters:
//   - fileId: The media file ID
//
// Response: Media file stream with range support
func (h *Handler) StreamFile(c *gin.Context) {
	fileID := c.Param("fileId")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID is required"})
		return
	}

	// Get media file
	mediaFile, err := h.mediaService.GetFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
		return
	}

	// For now, just do direct play
	// TODO: Make playback decision based on device capabilities
	h.progressHandler.ServeFile(c, mediaFile.Path)
}

// StreamDirect handles GET /api/v1/playback/stream/direct
// It streams a file directly by path (legacy method).
// This endpoint is kept for backward compatibility.
//
// Query parameters:
//   - path: The file path to stream
//
// Response: Media file stream with range support
func (h *Handler) StreamDirect(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path is required"})
		return
	}

	// Security: ensure path is safe
	// In production, validate against allowed paths
	cleanPath := filepath.Clean(filePath)

	// Use progressive handler to serve the file
	h.progressHandler.ServeFile(c, cleanPath)
}

// PrepareStream handles POST /api/v1/playback/stream/prepare
// It prepares a streaming URL based on a playback decision.
// This is useful for clients that need to construct URLs in advance.
//
// Request body:
//   {
//     "decision": { ... playback decision ... },
//     "base_url": "https://example.com"
//   }
//
// Response:
//   {
//     "stream_url": "https://example.com/api/v1/playback/stream/...",
//     "method": "direct"
//   }
func (h *Handler) PrepareStream(c *gin.Context) {
	var req struct {
		Decision *types.PlaybackDecision `json:"decision" binding:"required"`
		BaseURL  string                  `json:"base_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Construct stream URL based on playback method
	var streamURL string
	switch req.Decision.Method {
	case types.PlaybackMethodTranscode:
		// For transcoding, we need to start a transcode session
		if req.Decision.TranscodeParams != nil {
			// The transcoding service will handle this
			streamURL = fmt.Sprintf("%s/api/v1/playback/stream/transcode", req.BaseURL)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Transcode params required for transcoding"})
			return
		}
	case types.PlaybackMethodRemux:
		// For remuxing, we also use the transcode endpoint but with remux-only params
		streamURL = fmt.Sprintf("%s/api/v1/playback/stream/transcode", req.BaseURL)
	case types.PlaybackMethodDirect:
		if req.Decision.DirectPlayURL != "" {
			streamURL = req.Decision.DirectPlayURL
		} else {
			// Default to direct stream
			streamURL = fmt.Sprintf("%s/api/v1/playback/stream/direct", req.BaseURL)
		}
	default:
		// Default to direct stream
		streamURL = fmt.Sprintf("%s/api/v1/playback/stream/direct", req.BaseURL)
	}

	c.JSON(http.StatusOK, gin.H{
		"stream_url": streamURL,
		"method":     string(req.Decision.Method),
	})
}