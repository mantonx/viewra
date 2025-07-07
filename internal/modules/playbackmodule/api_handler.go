package playbackmodule

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// APIHandler handles HTTP requests for the playback module
type APIHandler struct {
	manager *Manager
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(manager *Manager) *APIHandler {
	return &APIHandler{
		manager: manager,
	}
}

// HandlePlaybackDecision determines whether to direct play or transcode
func (h *APIHandler) HandlePlaybackDecision(c *gin.Context) {
	var request struct {
		MediaPath     string        `json:"media_path"`
		FileID        string        `json:"file_id"`       // Support frontend format
		MediaFileID   string        `json:"media_file_id"` // Alternative field name
		DeviceProfile DeviceProfile `json:"device_profile" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		api.RespondWithValidationError(c, "Invalid request format", err.Error())
		return
	}

	// Handle file ID to path resolution
	if request.MediaPath == "" && (request.FileID != "" || request.MediaFileID != "") {
		// Get media file path from ID
		mediaFileID := request.FileID
		if mediaFileID == "" {
			mediaFileID = request.MediaFileID
		}

		mediaPath, err := h.manager.GetMediaFilePath(mediaFileID)
		if err != nil {
			appErr := types.NewAppError(
				types.ErrorCodeMediaNotFound,
				"Failed to resolve media file",
				http.StatusNotFound,
			).WithContext("media_file_id", mediaFileID).
				WithUserMessage("The requested media file could not be found")
			api.RespondWithError(c, appErr)
			return
		}
		request.MediaPath = mediaPath
	}

	if request.MediaPath == "" {
		api.RespondWithValidationError(c, "media_path or file_id is required")
		return
	}

	decision, err := h.manager.DecidePlayback(request.MediaPath, &request.DeviceProfile)
	if err != nil {
		// Check error type
		if errors.Is(err, os.ErrNotExist) {
			api.RespondWithNotFound(c, "media file", request.MediaPath)
			return
		}

		appErr := types.NewInternalError("Failed to make playback decision", err).
			WithContext("media_path", request.MediaPath)
		api.RespondWithError(c, appErr)
		return
	}

	c.JSON(http.StatusOK, decision)
}

// HandleStartTranscode initiates a new transcoding session
func (h *APIHandler) HandleStartTranscode(c *gin.Context) {
	logger.Info("handleStartTranscode called")

	// Read the raw body to check what type of request this is
	bodyBytes, err := c.GetRawData()
	if err != nil {
		logger.Error("failed to read request body", "error", err)
		api.RespondWithValidationError(c, "Failed to read request body")
		return
	}

	logger.Info("raw request body", "body", string(bodyBytes))

	// Try to parse as media file request first
	var mediaRequest struct {
		MediaFileID   string         `json:"media_file_id"`
		MediaID       string         `json:"media_id"` // Support both field names
		Container     string         `json:"container"`
		SeekPosition  float64        `json:"seek_position,omitempty"`
		EnableABR     bool           `json:"enable_abr,omitempty"`
		DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`
		Settings      *struct {      // Support nested settings
			Container  string `json:"container"`
			VideoCodec string `json:"video_codec"`
			AudioCodec string `json:"audio_codec"`
			Quality    int    `json:"quality"`
			EnableABR  bool   `json:"enable_abr"`
		} `json:"settings,omitempty"`
	}

	parseErr := json.Unmarshal(bodyBytes, &mediaRequest)
	
	// Support both media_id and media_file_id
	if mediaRequest.MediaID != "" && mediaRequest.MediaFileID == "" {
		mediaRequest.MediaFileID = mediaRequest.MediaID
	}
	
	// Extract settings if provided in nested format
	if mediaRequest.Settings != nil {
		if mediaRequest.Container == "" {
			mediaRequest.Container = mediaRequest.Settings.Container
		}
		if !mediaRequest.EnableABR {
			mediaRequest.EnableABR = mediaRequest.Settings.EnableABR
		}
	}
	
	logger.Info("media request parse result", "error", parseErr, "media_file_id", mediaRequest.MediaFileID, "media_id", mediaRequest.MediaID, "container", mediaRequest.Container)

	if parseErr == nil && mediaRequest.MediaFileID != "" {
		// Handle media file based request
		// Create a request struct that matches what handleMediaFileTranscode expects
		mediaFileRequest := struct {
			MediaFileID   string         `json:"media_file_id"`
			Container     string         `json:"container"`
			SeekPosition  float64        `json:"seek_position,omitempty"`
			EnableABR     bool           `json:"enable_abr,omitempty"`
			DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`
		}{
			MediaFileID:   mediaRequest.MediaFileID,
			Container:     mediaRequest.Container,
			SeekPosition:  mediaRequest.SeekPosition,
			EnableABR:     mediaRequest.EnableABR,
			DeviceProfile: mediaRequest.DeviceProfile,
		}
		session, err := h.handleMediaFileTranscode(c, mediaFileRequest)
		if err != nil {
			// Error already handled by handleMediaFileTranscode
			return
		}

		// Build successful response
		h.respondWithSession(c, session)
		return
	}

	// Fall back to direct transcode request
	var directRequest struct {
		InputPath     string         `json:"input_path"`
		Container     string         `json:"container"`
		VideoCodec    string         `json:"video_codec"`
		AudioCodec    string         `json:"audio_codec"`
		Quality       int            `json:"quality"`
		SpeedPriority string         `json:"speed_priority"`
		Seek          float64        `json:"seek"`
		EnableABR     bool           `json:"enable_abr"`
		DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &directRequest); err != nil {
		logger.Error("failed to parse direct transcode request", "error", err)
		api.RespondWithValidationError(c, "Invalid request format", err.Error())
		return
	}

	session, err := h.handleDirectTranscode(c, directRequest)
	if err != nil {
		// Error already handled by handleDirectTranscode
		return
	}

	h.respondWithSession(c, session)
}

// handleMediaFileTranscode processes media file based transcoding requests
func (h *APIHandler) handleMediaFileTranscode(c *gin.Context, request struct {
	MediaFileID   string         `json:"media_file_id"`
	Container     string         `json:"container"`
	SeekPosition  float64        `json:"seek_position,omitempty"`
	EnableABR     bool           `json:"enable_abr,omitempty"`
	DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`
}) (*database.TranscodeSession, error) {
	logger.Info("handling media file based request",
		"media_file_id", request.MediaFileID,
		"container", request.Container,
		"seek_position", request.SeekPosition,
		"enable_abr", request.EnableABR)

	// Use device profile for intelligent transcoding decisions
	deviceProfile := request.DeviceProfile
	if deviceProfile == nil {
		logger.Warn("no device profile provided, using default profile")
		deviceProfile = h.getDefaultDeviceProfile()
	}

	session, err := h.manager.StartTranscodeFromMediaFile(
		request.MediaFileID,
		request.Container,
		request.SeekPosition,
		request.EnableABR,
		deviceProfile,
	)

	if err != nil {
		logger.Error("failed to start transcode from media file", "error", err)

		// Determine error type
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found"):
			api.RespondWithNotFound(c, "media file", request.MediaFileID)
		case strings.Contains(errMsg, "no provider"):
			appErr := types.NewAppError(
				types.ErrorCodeTranscodingUnavailable,
				"No transcoding provider available",
				http.StatusServiceUnavailable,
			).WithUserMessage("Video transcoding is temporarily unavailable. Please try again later.").
				WithRetryAfter(30 * time.Second)
			api.RespondWithError(c, appErr)
		case strings.Contains(errMsg, "session limit"):
			appErr := types.NewAppError(
				types.ErrorCodeSessionLimitReached,
				"Maximum concurrent sessions reached",
				http.StatusServiceUnavailable,
			).WithUserMessage("Too many active video streams. Please try again in a few minutes.").
				WithRetryAfter(1 * time.Minute)
			api.RespondWithError(c, appErr)
		default:
			appErr := types.NewTranscodingError(
				types.ErrorCodeTranscodingFailed,
				"Failed to start transcoding session",
				err,
			).WithContext("media_file_id", request.MediaFileID)
			api.RespondWithError(c, appErr)
		}

		return nil, err
	}

	logger.Info("transcode session created successfully", "session_id", session.ID)
	return session, nil
}

// handleDirectTranscode processes direct transcoding requests
func (h *APIHandler) handleDirectTranscode(c *gin.Context, request struct {
	InputPath     string         `json:"input_path"`
	Container     string         `json:"container"`
	VideoCodec    string         `json:"video_codec"`
	AudioCodec    string         `json:"audio_codec"`
	Quality       int            `json:"quality"`
	SpeedPriority string         `json:"speed_priority"`
	Seek          float64        `json:"seek"`
	EnableABR     bool           `json:"enable_abr"`
	DeviceProfile *DeviceProfile `json:"device_profile,omitempty"`
}) (*database.TranscodeSession, error) {
	logger.Info("handling direct transcode request", "input_path", request.InputPath)

	// Use device profile for intelligent transcoding decisions
	deviceProfile := request.DeviceProfile
	if deviceProfile == nil {
		logger.Warn("no device profile provided for direct request, using default profile")
		deviceProfile = h.getDefaultDeviceProfile()
	}

	// Use playback planner to make intelligent decisions
	decision, err := h.manager.DecidePlayback(request.InputPath, deviceProfile)
	if err != nil {
		logger.Error("failed to make playback decision", "error", err)

		if errors.Is(err, os.ErrNotExist) {
			api.RespondWithNotFound(c, "media file", request.InputPath)
		} else {
			appErr := types.NewInternalError("Failed to analyze media file", err).
				WithContext("input_path", request.InputPath)
			api.RespondWithError(c, appErr)
		}
		return nil, err
	}

	// Check if transcoding is needed
	if !decision.ShouldTranscode {
		logger.Info("direct play recommended", "reason", decision.Reason)
		c.JSON(http.StatusOK, gin.H{
			"direct_play": true,
			"reason":      decision.Reason,
			"stream_url":  decision.StreamURL,
		})
		return nil, nil
	}

	// Use intelligent transcoding parameters from decision
	transcodeReq := decision.TranscodeParams
	if transcodeReq == nil {
		appErr := types.NewInternalError("No transcoding parameters in decision", nil)
		api.RespondWithError(c, appErr)
		return nil, errors.New("no transcoding parameters")
	}

	// Apply user-specified overrides where provided
	if request.Container != "" {
		transcodeReq.Container = request.Container
	}
	if request.Seek > 0 {
		transcodeReq.Seek = time.Duration(request.Seek * float64(time.Second))
	}
	transcodeReq.EnableABR = request.EnableABR

	logger.Info("using intelligent transcode request",
		"input_path", transcodeReq.InputPath,
		"container", transcodeReq.Container,
		"video_codec", transcodeReq.VideoCodec,
		"audio_codec", transcodeReq.AudioCodec,
		"quality", transcodeReq.Quality,
		"decision_reason", decision.Reason)

	session, err := h.manager.StartTranscode(transcodeReq)
	if err != nil {
		logger.Error("failed to start transcode", "error", err)

		// Parse FFmpeg-specific errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "ffmpeg") {
			if strings.Contains(errMsg, "not found") {
				appErr := types.NewAppError(
					types.ErrorCodeFFmpegNotFound,
					"FFmpeg not available",
					http.StatusServiceUnavailable,
				).WithUserMessage("Video processing is unavailable. Please contact support.")
				api.RespondWithError(c, appErr)
				return nil, err
			} else if strings.Contains(errMsg, "killed") || strings.Contains(errMsg, "signal") {
				appErr := types.NewTranscodingError(
					types.ErrorCodeFFmpegKilled,
					"FFmpeg process terminated",
					err,
				).WithRetryAfter(10 * time.Second)
				api.RespondWithError(c, appErr)
				return nil, err
			} else {
				appErr := types.NewTranscodingError(
					types.ErrorCodeFFmpegFailed,
					"FFmpeg encoding failed",
					err,
				).WithContext("input_path", request.InputPath)
				api.RespondWithError(c, appErr)
				return nil, err
			}
		} else {
			appErr := types.NewTranscodingError(
				types.ErrorCodeTranscodingFailed,
				"Failed to start transcoding",
				err,
			)
			api.RespondWithError(c, appErr)
			return nil, err
		}
	}

	logger.Info("transcode session created successfully", "session_id", session.ID)
	return session, nil
}

// respondWithSession builds and sends the session response
func (h *APIHandler) respondWithSession(c *gin.Context, session *database.TranscodeSession) {
	if session == nil {
		return // Already handled (e.g., direct play)
	}

	response := gin.H{
		"id":       session.ID,
		"status":   session.Status,
		"provider": session.Provider,
	}

	// Add content hash and URLs if available
	if session.ContentHash != "" {
		response["content_hash"] = session.ContentHash
		response["content_url"] = fmt.Sprintf("/api/v1/content/%s/", session.ContentHash)
		response["manifest_url"] = fmt.Sprintf("/api/v1/content/%s/manifest.mpd", session.ContentHash)
	} else {
		// For new sessions, content hash will be generated during transcoding
		logger.Warn("session doesn't have content hash yet, using session-based URLs", "session_id", session.ID)
		response["manifest_url"] = fmt.Sprintf("/api/v1/sessions/%s/manifest.mpd", session.ID)
		response["content_url"] = fmt.Sprintf("/api/v1/sessions/%s/", session.ID)
		response["uses_session_urls"] = true
	}

	c.JSON(http.StatusOK, response)
}

// HandleSeekAhead handles seek-ahead transcoding requests
func (h *APIHandler) HandleSeekAhead(c *gin.Context) {
	logger.Info("HandleSeekAhead called")

	// Read the raw body to debug what's being sent
	bodyBytes, err := c.GetRawData()
	if err != nil {
		logger.Error("failed to read request body", "error", err)
		api.RespondWithValidationError(c, "Failed to read request body")
		return
	}

	logger.Info("seek-ahead raw request body", "body", string(bodyBytes))

	var request struct {
		SessionID    string  `json:"session_id" binding:"required"`
		SeekPosition float64 `json:"seek_position" binding:"required"`
	}

	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		logger.Error("failed to parse JSON request", "error", err)
		api.RespondWithValidationError(c, "Invalid request format", err.Error())
		return
	}

	logger.Info("seek-ahead request parsed", "session_id", request.SessionID, "seek_position", request.SeekPosition)

	// Get the original session
	originalSession, err := h.manager.GetSession(request.SessionID)
	if err != nil {
		api.RespondWithNotFound(c, "session", request.SessionID)
		return
	}

	// Get the request from the original session
	originalRequest, err := originalSession.GetRequest()
	if err != nil || originalRequest == nil {
		appErr := types.NewAppError(
			types.ErrorCodeSessionInvalid,
			"Original session has no request data",
			http.StatusInternalServerError,
		).WithContext("session_id", request.SessionID)
		api.RespondWithError(c, appErr)
		return
	}

	// Create a new request with the seek offset
	seekRequest := *originalRequest
	seekRequest.Seek = time.Duration(request.SeekPosition) * time.Second
	seekRequest.SessionID = "" // Let the system generate a new session ID

	// Start new session with seek offset
	newSession, err := h.manager.StartTranscode(&seekRequest)
	if err != nil {
		appErr := types.NewTranscodingError(
			types.ErrorCodeTranscodingFailed,
			"Failed to start seek-ahead session",
			err,
		).WithContext("original_session", request.SessionID).
			WithContext("seek_position", request.SeekPosition)
		api.RespondWithError(c, appErr)
		return
	}

	h.respondWithSession(c, newSession)
}

// HandleGetSession retrieves transcoding session information
func (h *APIHandler) HandleGetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		api.RespondWithNotFound(c, "session", sessionID)
		return
	}

	// Convert database session to response format
	response := gin.H{
		"id":           session.ID,
		"status":       session.Status,
		"provider":     session.Provider,
		"content_hash": session.ContentHash,
		"directory":    session.DirectoryPath,
		"start_time":   session.StartTime,
		"end_time":     session.EndTime,
	}

	// Parse progress if available
	if session.Progress != "" {
		var progress plugins.TranscodingProgress
		if err := json.Unmarshal([]byte(session.Progress), &progress); err == nil {
			response["progress"] = progress.PercentComplete
			response["Progress"] = session.Progress // Keep raw progress data
		}
	}

	// Add manifest and content URLs
	if session.ContentHash != "" {
		response["content_url"] = fmt.Sprintf("/api/v1/content/%s/", session.ContentHash)
		response["manifest_url"] = fmt.Sprintf("/api/v1/content/%s/manifest.mpd", session.ContentHash)
	} else {
		// Use session-based URLs as fallback
		response["manifest_url"] = fmt.Sprintf("/api/v1/sessions/%s/manifest.mpd", session.ID)
		response["content_url"] = fmt.Sprintf("/api/v1/sessions/%s/", session.ID)
		response["uses_session_urls"] = true
	}

	c.JSON(http.StatusOK, response)
}

// HandleStopTranscode terminates a transcoding session
func (h *APIHandler) HandleStopTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")

	if err := h.manager.StopSession(sessionID); err != nil {
		logger.Error("failed to stop transcode", "error", err, "session_id", sessionID)

		if strings.Contains(err.Error(), "not found") {
			api.RespondWithNotFound(c, "session", sessionID)
		} else {
			appErr := types.NewInternalError("Failed to stop session", err).
				WithContext("session_id", sessionID)
			api.RespondWithError(c, appErr)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session stopped"})
}

// HandleListSessions returns all active transcoding sessions
func (h *APIHandler) HandleListSessions(c *gin.Context) {
	sessions, err := h.manager.ListSessions()
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		api.RespondWithInternalError(c, "Failed to list sessions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// HandleStopAllSessions stops all active transcoding sessions
func (h *APIHandler) HandleStopAllSessions(c *gin.Context) {
	sessions, err := h.manager.ListSessions()
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		api.RespondWithInternalError(c, "Failed to list sessions", err)
		return
	}

	var stoppedCount int
	var errors []string

	for _, session := range sessions {
		if session.Status == "running" || session.Status == "queued" {
			if err := h.manager.StopSession(session.ID); err != nil {
				errors = append(errors, fmt.Sprintf("session %s: %v", session.ID, err))
				logger.Error("failed to stop session", "session_id", session.ID, "error", err)
			} else {
				stoppedCount++
			}
		}
	}

	response := gin.H{
		"stopped_count":  stoppedCount,
		"total_sessions": len(sessions),
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}

// HandleCleanupStaleSessions manually triggers cleanup of stale sessions
func (h *APIHandler) HandleCleanupStaleSessions(c *gin.Context) {
	// Parse optional max_age parameter (default 2 hours)
	maxAgeHours := 2
	if maxAge := c.Query("max_age_hours"); maxAge != "" {
		if parsed, err := strconv.Atoi(maxAge); err == nil && parsed > 0 {
			maxAgeHours = parsed
		}
	}

	maxAge := time.Duration(maxAgeHours) * time.Hour

	// Call the cleanup method directly
	count, err := h.manager.GetSessionStore().CleanupStaleSessions(maxAge)
	if err != nil {
		logger.Error("failed to cleanup stale sessions", "error", err)
		api.RespondWithInternalError(c, "Failed to cleanup sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cleaned_count": count,
		"max_age_hours": maxAgeHours,
		"message":       fmt.Sprintf("Marked %d stale sessions as failed", count),
	})
}

// HandleListOrphanedSessions lists potential orphaned sessions
func (h *APIHandler) HandleListOrphanedSessions(c *gin.Context) {
	// Get all sessions
	sessions, err := h.manager.GetSessionStore().GetActiveSessions()
	if err != nil {
		logger.Error("failed to get active sessions", "error", err)
		api.RespondWithInternalError(c, "Failed to get active sessions", err)
		return
	}

	// Check for orphaned sessions (running/queued for too long)
	var orphaned []gin.H
	threshold := 30 * time.Minute

	for _, session := range sessions {
		if time.Since(session.UpdatedAt) > threshold {
			orphaned = append(orphaned, gin.H{
				"id":          session.ID,
				"status":      session.Status,
				"provider":    session.Provider,
				"last_update": session.UpdatedAt,
				"age":         time.Since(session.UpdatedAt).String(),
				"directory":   session.DirectoryPath,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"orphaned_sessions": orphaned,
		"count":             len(orphaned),
		"threshold":         threshold.String(),
	})
}

// HandleGetStats returns transcoding statistics
func (h *APIHandler) HandleGetStats(c *gin.Context) {
	stats, err := h.manager.GetStats()
	if err != nil {
		logger.Error("failed to get stats", "error", err)
		api.RespondWithInternalError(c, "Failed to get statistics", err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// HandleErrorRecoveryStats returns error recovery and circuit breaker statistics
func (h *APIHandler) HandleErrorRecoveryStats(c *gin.Context) {
	stats := h.manager.GetErrorRecoveryStats()
	c.JSON(http.StatusOK, stats)
}

// HandleValidateMedia validates a media file
func (h *APIHandler) HandleValidateMedia(c *gin.Context) {
	var request struct {
		MediaPath string `json:"media_path" binding:"required"`
		Quick     bool   `json:"quick,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		api.RespondWithValidationError(c, "Invalid request format", err.Error())
		return
	}

	validation, err := h.manager.ValidateMediaFile(c.Request.Context(), request.MediaPath, request.Quick)

	if err != nil {
		logger.Error("Media validation failed", "error", err, "path", request.MediaPath)

		if errors.Is(err, os.ErrNotExist) {
			api.RespondWithNotFound(c, "media file", request.MediaPath)
		} else if strings.Contains(err.Error(), "unsupported") {
			appErr := types.NewAppError(
				types.ErrorCodeMediaUnsupported,
				"Media format not supported",
				http.StatusBadRequest,
			).WithContext("path", request.MediaPath).
				WithUserMessage("This media format is not supported for playback")
			api.RespondWithError(c, appErr)
		} else {
			api.RespondWithInternalError(c, "Media validation failed", err)
		}
		return
	}

	c.JSON(http.StatusOK, validation)
}

// HandleHealthCheck returns module health status
func (h *APIHandler) HandleHealthCheck(c *gin.Context) {
	// Get provider count
	providerCount := 0
	var providerNames []string

	if h.manager.transcodingService != nil {
		providers := h.manager.transcodingService.GetProviders()
		providerCount = len(providers)
		for _, info := range providers {
			providerNames = append(providerNames, fmt.Sprintf("%s (%s)", info.Name, info.ID))
		}
	}

	// Determine readiness
	ready := h.manager.IsEnabled() && h.manager.initialized && providerCount > 0
	status := "healthy"
	if !ready {
		status = "degraded"
	}

	health := gin.H{
		"status":      status,
		"ready":       ready,
		"enabled":     h.manager.IsEnabled(),
		"initialized": h.manager.initialized,
		"providers": gin.H{
			"count": providerCount,
			"names": providerNames,
		},
		"message": func() string {
			if !h.manager.IsEnabled() {
				return "Playback module is disabled"
			}
			if !h.manager.initialized {
				return "Playback module is not initialized"
			}
			if providerCount == 0 {
				return "No transcoding providers available (plugin discovery in progress)"
			}
			return "Ready for transcoding"
		}(),
	}

	c.JSON(http.StatusOK, health)
}

// HandleRefreshPlugins refreshes the list of available transcoding plugins
func (h *APIHandler) HandleRefreshPlugins(c *gin.Context) {
	if err := h.manager.RefreshTranscodingPlugins(); err != nil {
		appErr := types.NewPluginError(
			"transcoding",
			types.ErrorCodePluginFailed,
			"Failed to refresh plugins",
			err,
		)
		api.RespondWithError(c, appErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "plugins refreshed successfully"})
}

// HandleManualCleanup triggers manual cleanup
func (h *APIHandler) HandleManualCleanup(c *gin.Context) {
	// Check for custom retention hours parameter
	retentionHours := 24 // default
	if hours := c.Query("retention_hours"); hours != "" {
		if parsed, err := strconv.Atoi(hours); err == nil && parsed > 0 {
			retentionHours = parsed
		}
	}

	// Run cleanup with custom retention
	policy := core.RetentionPolicy{
		RetentionHours:     retentionHours,
		ExtendedHours:      retentionHours * 2,
		MaxTotalSizeGB:     50,
		LargeFileThreshold: 500 * 1024 * 1024,
	}

	count, err := h.manager.GetSessionStore().CleanupExpiredSessions(policy)
	if err != nil {
		api.RespondWithInternalError(c, "Failed to cleanup sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "manual cleanup triggered",
		"cleaned_sessions": count,
		"retention_hours":  retentionHours,
		"note":             "cleanup is now handled by transcoding module",
	})
}

// HandleCleanupStats returns cleanup statistics
func (h *APIHandler) HandleCleanupStats(c *gin.Context) {
	// Cleanup stats are now available from transcoding module
	c.JSON(http.StatusOK, gin.H{
		"message":  "cleanup statistics are now available from the transcoding module API",
		"endpoint": "/api/v1/transcoding/cleanup/stats",
	})
}

// HandleGetFFmpegLogs serves FFmpeg stdout/stderr logs for debugging
func (h *APIHandler) HandleGetFFmpegLogs(c *gin.Context) {
	sessionID := c.Param("sessionId")
	logType := c.Query("type") // "stdout" or "stderr"

	if logType == "" {
		logType = "stderr" // Default to stderr
	}

	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		logger.Error("session not found for logs", "session_id", sessionID, "error", err)
		api.RespondWithNotFound(c, "session", sessionID)
		return
	}

	var logFilename string
	switch logType {
	case "stdout":
		logFilename = "ffmpeg-stdout.log"
	case "stderr":
		logFilename = "ffmpeg-stderr.log"
	default:
		api.RespondWithValidationError(c, "Invalid log type, use 'stdout' or 'stderr'")
		return
	}

	logPath := filepath.Join(h.getSessionDirectory(sessionID, session), logFilename)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		api.RespondWithNotFound(c, "log file", logPath)
		return
	}

	// Read log file
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		logger.Error("failed to read log file", "path", logPath, "error", err)
		api.RespondWithInternalError(c, "Failed to read log file", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"log_type":   logType,
		"content":    string(logContent),
	})
}

// Helper method for getting session directory path
func (h *APIHandler) getSessionDirectory(sessionID string, session *database.TranscodeSession) string {
	cfg := config.Get()

	// If session has directory path, use it
	if session != nil && session.DirectoryPath != "" {
		logger.Info("using session.DirectoryPath", "session_id", sessionID, "directory_path", session.DirectoryPath)
		return session.DirectoryPath
	}

	// Otherwise construct it based on naming convention
	var dirName string
	if session != nil && session.Request != "" {
		request, err := session.GetRequest()
		if err == nil && request != nil {
			switch request.Container {
			case "dash":
				dirName = fmt.Sprintf("dash_%s_%s", session.Provider, sessionID)
			case "hls":
				dirName = fmt.Sprintf("hls_%s_%s", session.Provider, sessionID)
			default:
				dirName = fmt.Sprintf("mp4_%s_%s", session.Provider, sessionID)
			}
		} else {
			dirName = fmt.Sprintf("transcode_%s_%s", session.Provider, sessionID)
		}
	} else {
		// No session, just use ID
		dirName = fmt.Sprintf("transcode_%s", sessionID)
	}

	return filepath.Join(cfg.Transcoding.DataDir, dirName)
}

// getDefaultDeviceProfile returns a default device profile for compatibility
func (h *APIHandler) getDefaultDeviceProfile() *DeviceProfile {
	return &DeviceProfile{
		UserAgent:       "unknown",
		SupportedCodecs: []string{"h264", "aac"},
		MaxResolution:   "1080p",
		MaxBitrate:      6000,
		SupportsHEVC:    false,
		SupportsAV1:     false,
		SupportsHDR:     false,
	}
}
