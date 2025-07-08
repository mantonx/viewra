// Package api provides HTTP API handlers for the playback module.
package api

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	playbacktypes "github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
)

// Handler handles HTTP requests for the playback module
type Handler struct {
	playbackService services.PlaybackService
	mediaService    services.MediaService
	progressHandler *core.ProgressiveHandler
	sessionManager  *core.SessionManager
	logger          hclog.Logger
}

// NewHandler creates a new API handler
func NewHandler(playbackService services.PlaybackService, mediaService services.MediaService, progressHandler *core.ProgressiveHandler, sessionManager *core.SessionManager, logger hclog.Logger) *Handler {
	return &Handler{
		playbackService: playbackService,
		mediaService:    mediaService,
		progressHandler: progressHandler,
		sessionManager:  sessionManager,
		logger:          logger,
	}
}

// DecidePlayback handles POST /api/v1/playback/decide
// Determines the best playback method for a media file
func (h *Handler) DecidePlayback(c *gin.Context) {
	var req struct {
		MediaPath     string               `json:"media_path" binding:"required"`
		DeviceProfile *types.DeviceProfile `json:"device_profile" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	decision, err := h.playbackService.DecidePlayback(req.MediaPath, req.DeviceProfile)
	if err != nil {
		logger.Error("Failed to make playback decision", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, decision)
}

// GetMediaInfo handles GET /api/v1/playback/media-info
// Returns detailed information about a media file
func (h *Handler) GetMediaInfo(c *gin.Context) {
	mediaPath := c.Query("path")
	if mediaPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Media path is required"})
		return
	}

	info, err := h.playbackService.GetMediaInfo(mediaPath)
	if err != nil {
		logger.Error("Failed to get media info", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// StartSession handles POST /api/v1/playback/sessions
// Starts a new playback session with optional device analytics
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
// Updates an existing playback session including analytics data
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
// Ends a playback session
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
// Gets information about a specific session
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
// Returns all active playback sessions
func (h *Handler) GetActiveSessions(c *gin.Context) {
	sessions := h.sessionManager.GetActiveSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// StreamDirect handles GET /api/v1/playback/stream/direct
// Streams a file directly with range support
func (h *Handler) StreamDirect(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path is required"})
		return
	}

	// Security: ensure path is safe
	// In production, you'd validate against allowed paths
	cleanPath := filepath.Clean(filePath)

	// Use progressive handler to serve the file
	h.progressHandler.ServeFile(c, cleanPath)
}

// PrepareStream handles POST /api/v1/playback/prepare
// Prepares a stream URL based on playback decision
func (h *Handler) PrepareStream(c *gin.Context) {
	var req struct {
		Decision *types.PlaybackDecision `json:"decision" binding:"required"`
		BaseURL  string                  `json:"base_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// PrepareStreamURL is not part of the interface, so we'll construct it here
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

// Heartbeat handles POST /api/v1/playback/sessions/:sessionId/heartbeat
// Updates session activity to keep it alive
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

// GetSupportedFormats handles GET /api/v1/playback/supported-formats
// Returns formats supported by a device profile
func (h *Handler) GetSupportedFormats(c *gin.Context) {
	var deviceProfile types.DeviceProfile
	if err := c.ShouldBindJSON(&deviceProfile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device profile"})
		return
	}

	formats := h.playbackService.GetSupportedFormats(&deviceProfile)
	c.JSON(http.StatusOK, gin.H{
		"formats": formats,
		"count":   len(formats),
	})
}

// AnalyticsSession handles POST /api/analytics/session
// Records playback session analytics and events
func (h *Handler) AnalyticsSession(c *gin.Context) {
	var req struct {
		SessionInfo map[string]interface{} `json:"session_info"`
		Analytics   map[string]interface{} `json:"analytics"`
		Events      []map[string]interface{} `json:"events"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid analytics request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Log the analytics data for now
	h.logger.Info("Playback analytics received",
		"sessionId", req.SessionInfo["sessionId"],
		"mediaId", req.SessionInfo["mediaId"],
		"watchTime", req.Analytics["watchTime"],
		"bufferingEvents", req.Analytics["bufferingTime"],
		"errorCount", req.Analytics["errorCount"],
		"eventCount", len(req.Events),
	)

	// TODO: Store analytics in database for future analysis
	// For now, just acknowledge receipt
	c.JSON(http.StatusOK, gin.H{
		"status": "received",
		"message": "Analytics data recorded successfully",
	})
}

// GetPlaybackCompatibility handles GET /api/playback/compatibility
// Returns playback compatibility for a batch of media files
func (h *Handler) GetPlaybackCompatibility(c *gin.Context) {
	var req struct {
		MediaFileIds    []string              `json:"media_file_ids"`
		DeviceProfile   *types.DeviceProfile  `json:"device_profile"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Use a default device profile if none provided
	deviceProfile := req.DeviceProfile
	if deviceProfile == nil {
		deviceProfile = &types.DeviceProfile{
			SupportedVideoCodecs: []string{"h264"},
			SupportedAudioCodecs: []string{"aac", "mp3"},
			SupportedContainers:  []string{"mp4", "webm"},
		}
	}

	results := make(map[string]interface{})
	
	for _, mediaFileId := range req.MediaFileIds {
		// Get media file from media service
		ctx := c.Request.Context()
		mediaFile, err := h.mediaService.GetFile(ctx, mediaFileId)
		if err != nil {
			results[mediaFileId] = gin.H{
				"error": "Media file not found",
			}
			continue
		}
		
		// Get playback decision
		decision, err := h.playbackService.DecidePlayback(mediaFile.Path, deviceProfile)
		if err != nil {
			results[mediaFileId] = gin.H{
				"error": "Failed to determine compatibility",
			}
			continue
		}
		
		// Use the method from the decision
		canDirectPlay := decision.Method == types.PlaybackMethodDirect
		
		results[mediaFileId] = gin.H{
			"method": string(decision.Method),  // Convert to string for JSON
			"reason": decision.Reason,
			"can_direct_play": canDirectPlay,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"compatibility": results,
	})
}
