// Package api - Session management handlers
package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	playbacktypes "github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	"github.com/mantonx/viewra/internal/services"
	plugins "github.com/mantonx/viewra/sdk"
)

// StartSession handles POST /api/v1/playback/sessions
// It starts a new playback session with optional device analytics.
//
// Request body:
//   {
//     "media_file_id": "123",
//     "user_id": "user1",
//     "device_id": "device1",
//     "method": "direct",
//     "analytics": {
//       "ip_address": "192.168.1.1",
//       "user_agent": "Mozilla/5.0...",
//       "device_name": "Chrome on Windows",
//       "device_type": "desktop"
//     }
//   }
//
// Response: PlaybackSession object with session ID and stream URL
func (h *Handler) StartSession(c *gin.Context) {
	var req playbacktypes.SessionStartRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Extract client IP address if not provided
	if req.Analytics != nil && req.Analytics.IPAddress == "" {
		req.Analytics.IPAddress = c.ClientIP()
	}

	// Start session
	var session *core.PlaybackSession
	var err error

	if req.Analytics != nil {
		// Use sessionManager directly for analytics-enabled sessions
		session, err = h.sessionManager.CreateSessionWithAnalytics(
			req.MediaFileID,
			req.UserID,
			req.DeviceID,
			req.Method,
			req.Analytics,
		)
	} else {
		session, err = h.sessionManager.CreateSession(
			req.MediaFileID,
			req.UserID,
			req.DeviceID,
			req.Method,
		)
	}
	if err != nil {
		logger.Error("Failed to start session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// UpdateSession handles PUT /api/v1/playback/sessions/:sessionId
// It updates an existing playback session including position and analytics data.
//
// Path parameters:
//   - sessionId: The session ID to update
//
// Request body:
//   {
//     "position": 120,
//     "state": "playing",
//     "quality_played": "1080p",
//     "bandwidth": 5000000,
//     "debug_info": { ... }
//   }
//
// Response: Success message
func (h *Handler) UpdateSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	var req playbacktypes.SessionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Build updates map from typed request
	updates := make(map[string]interface{})
	if req.Position != nil {
		updates["position"] = *req.Position
	}
	if req.State != nil {
		updates["state"] = *req.State
	}
	if req.QualityPlayed != nil {
		updates["quality_played"] = *req.QualityPlayed
	}
	if req.Bandwidth != nil {
		updates["bandwidth"] = *req.Bandwidth
	}
	if req.DebugInfo != nil {
		updates["debug_info"] = req.DebugInfo
	}

	err := h.sessionManager.UpdateSession(sessionID, updates)
	if err != nil {
		logger.Error("Failed to update session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session updated"})
}

// EndSession handles DELETE /api/v1/playback/sessions/:sessionId
// It ends a playback session and cleans up any associated resources.
//
// Path parameters:
//   - sessionId: The session ID to end
//
// Response: Success message
func (h *Handler) EndSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	err := h.sessionManager.EndSession(sessionID)
	if err != nil {
		logger.Error("Failed to end session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session ended"})
}

// GetSession handles GET /api/v1/playback/sessions/:sessionId
// It retrieves information about a specific playback session.
//
// Path parameters:
//   - sessionId: The session ID to retrieve
//
// Response: PlaybackSession object or 404 if not found
func (h *Handler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetActiveSessions handles GET /api/v1/playback/sessions
// It returns all currently active playback sessions.
//
// Response:
//   {
//     "sessions": [...],
//     "count": 5
//   }
func (h *Handler) GetActiveSessions(c *gin.Context) {
	sessions := h.sessionManager.GetActiveSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// Heartbeat handles POST /api/v1/playback/sessions/:sessionId/heartbeat
// It updates session activity to keep it alive and reports current playback position.
//
// Path parameters:
//   - sessionId: The session ID to keep alive
//
// Request body:
//   {
//     "position": 240,
//     "state": "playing"
//   }
//
// Response: Success message
func (h *Handler) Heartbeat(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	var req struct {
		Position int64  `json:"position"`
		State    string `json:"state"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{
		"position": req.Position,
		"state":    req.State,
	}

	err := h.sessionManager.UpdateSession(sessionID, updates)
	if err != nil {
		logger.Error("Failed to update heartbeat", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Heartbeat received"})
}

// StartTranscodeSession handles POST /api/v1/playback/transcode
// It starts a transcoding session for a media file.
// This endpoint proxies to the transcoding service via the service registry.
//
// Request body:
//   {
//     "mediaId": "...",
//     "container": "mp4",
//     "inputPath": "...",
//     "encodingOptions": {
//       "videoCodec": "h264",
//       "audioCodec": "aac",
//       "quality": 23
//     }
//   }
//
// Response:
//   {
//     "sessionId": "...",
//     "status": "...",
//     "contentHash": "...",
//     "contentUrl": "..."
//   }
func (h *Handler) StartTranscodeSession(c *gin.Context) {
	var req struct {
		MediaID         string `json:"mediaId" binding:"required"`
		Container       string `json:"container"`
		InputPath       string `json:"inputPath"`
		EncodingOptions struct {
			VideoCodec string `json:"videoCodec"`
			AudioCodec string `json:"audioCodec"`
			Quality    int    `json:"quality"`
		} `json:"encodingOptions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get transcoding service from registry
	transcodingService, err := services.Get("transcoding")
	if err != nil {
		h.logger.Error("Transcoding service not available", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoding service not available"})
		return
	}

	transcoder := transcodingService.(services.TranscodingService)

	// If inputPath is not provided, look it up from the media file
	if req.InputPath == "" {
		mediaFile, err := h.mediaService.GetFile(c.Request.Context(), req.MediaID)
		if err != nil {
			h.logger.Error("Failed to get media file", "error", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Media file not found"})
			return
		}
		req.InputPath = mediaFile.Path
	}

	// Create transcode request
	transcodeReq := &plugins.TranscodeRequest{
		InputPath:   req.InputPath,
		MediaID:     req.MediaID,
		Container:   req.Container,
		VideoCodec:  req.EncodingOptions.VideoCodec,
		AudioCodec:  req.EncodingOptions.AudioCodec,
		Quality:     req.EncodingOptions.Quality,
	}

	// Default values if not provided
	if transcodeReq.Container == "" {
		transcodeReq.Container = "mp4"
	}
	if transcodeReq.VideoCodec == "" {
		transcodeReq.VideoCodec = "h264"
	}
	if transcodeReq.AudioCodec == "" {
		transcodeReq.AudioCodec = "aac"
	}
	if transcodeReq.Quality == 0 {
		transcodeReq.Quality = 23
	}

	// Start transcoding via the service
	session, err := transcoder.StartTranscode(c.Request.Context(), transcodeReq)
	if err != nil {
		h.logger.Error("Failed to start transcode", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transcoding"})
		return
	}

	// Return response matching frontend expectations
	c.JSON(http.StatusCreated, gin.H{
		"sessionId":   session.ID,
		"status":      session.Status,
		"contentHash": session.ContentHash,
		"contentUrl":  fmt.Sprintf("/api/v1/transcoding/content/%s", session.ContentHash),
	})
}

// StopTranscodeSession handles DELETE /api/v1/playback/transcode/:sessionId
// It stops a transcoding session.
//
// Path parameters:
//   - sessionId: The transcoding session ID
//
// Response: 204 No Content on success
func (h *Handler) StopTranscodeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	// Get transcoding service from registry
	transcodingService, err := services.Get("transcoding")
	if err != nil {
		h.logger.Error("Transcoding service not available", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Transcoding service not available"})
		return
	}

	transcoder := transcodingService.(services.TranscodingService)

	// Stop the session
	if err := transcoder.StopSession(sessionID); err != nil {
		h.logger.Error("Failed to stop transcode session", "error", err, "sessionId", sessionID)
		// Don't return error to client - session might already be stopped
	}

	c.Status(http.StatusNoContent)
}