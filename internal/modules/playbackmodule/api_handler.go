package playbackmodule

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
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
		MediaPath     string        `json:"media_path" binding:"required"`
		DeviceProfile DeviceProfile `json:"device_profile" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	decision, err := h.manager.DecidePlayback(request.MediaPath, &request.DeviceProfile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	
	logger.Info("raw request body", "body", string(bodyBytes))
	
	// Try to parse as media file request first
	var mediaRequest struct {
		MediaFileID   string         `json:"media_file_id"`
		Container     string         `json:"container"`
		SeekPosition  float64        `json:"seek_position,omitempty"`  // Optional seek position in seconds
		EnableABR     bool           `json:"enable_abr,omitempty"`      // Optional ABR flag
		DeviceProfile *DeviceProfile `json:"device_profile,omitempty"` // Optional device profile for intelligent decisions
	}
	
	parseErr := json.Unmarshal(bodyBytes, &mediaRequest)
	logger.Info("media request parse result", "error", parseErr, "media_file_id", mediaRequest.MediaFileID, "container", mediaRequest.Container)
	
	if parseErr == nil && mediaRequest.MediaFileID != "" {
		// Handle media file based request with intelligent decisions
		logger.Info("handling media file based request", "media_file_id", mediaRequest.MediaFileID, "container", mediaRequest.Container, "seek_position", mediaRequest.SeekPosition, "enable_abr", mediaRequest.EnableABR)
		
		// Use device profile for intelligent transcoding decisions
		// If no device profile provided, create a default one for compatibility
		deviceProfile := mediaRequest.DeviceProfile
		if deviceProfile == nil {
			logger.Warn("no device profile provided, using default profile")
			deviceProfile = &DeviceProfile{
				UserAgent:       "unknown",
				SupportedCodecs: []string{"h264", "aac"},
				MaxResolution:   "1080p",
				MaxBitrate:      6000,
				SupportsHEVC:    false,
				SupportsAV1:     false,
				SupportsHDR:     false,
			}
		}
		
		session, err := h.manager.StartTranscodeFromMediaFile(mediaRequest.MediaFileID, mediaRequest.Container, mediaRequest.SeekPosition, mediaRequest.EnableABR, deviceProfile)
		if err != nil {
			logger.Error("failed to start transcode from media file", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transcoding session: " + err.Error()})
			return
		}
		
		logger.Info("transcode session created successfully", "session_id", session.ID)
		
		// Return the session information
		c.JSON(http.StatusOK, gin.H{
			"id":           session.ID,
			"status":       session.Status,
			"manifest_url": fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", session.ID),
			"provider":     session.Provider,
		})
		return
	}
	
	// Fall back to direct transcode request with intelligent decisions
	var directRequest struct {
		InputPath     string         `json:"input_path" binding:"required"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("handling direct transcode request with intelligent decisions", "input_path", directRequest.InputPath)

	// Use device profile for intelligent transcoding decisions
	deviceProfile := directRequest.DeviceProfile
	if deviceProfile == nil {
		logger.Warn("no device profile provided for direct request, using default profile")
		deviceProfile = &DeviceProfile{
			UserAgent:       "unknown",
			SupportedCodecs: []string{"h264", "aac"},
			MaxResolution:   "1080p",
			MaxBitrate:      6000,
			SupportsHEVC:    false,
			SupportsAV1:     false,
			SupportsHDR:     false,
		}
	}

	// Use playback planner to make intelligent decisions
	decision, err := h.manager.DecidePlayback(directRequest.InputPath, deviceProfile)
	if err != nil {
		logger.Error("failed to make playback decision for direct request", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to make playback decision: " + err.Error()})
		return
	}

	// Check if transcoding is needed
	if !decision.ShouldTranscode {
		logger.Info("direct play recommended for direct request", "reason", decision.Reason)
		c.JSON(http.StatusOK, gin.H{
			"direct_play": true,
			"reason":      decision.Reason,
			"stream_url":  decision.StreamURL,
		})
		return
	}

	// Use intelligent transcoding parameters from decision
	request := decision.TranscodeParams
	if request == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no transcoding parameters in decision"})
		return
	}

	// Apply user-specified overrides where provided
	if directRequest.Container != "" {
		request.Container = directRequest.Container
	}
	if directRequest.Seek > 0 {
		request.Seek = time.Duration(directRequest.Seek * float64(time.Second))
	}
	request.EnableABR = directRequest.EnableABR

	logger.Info("using intelligent transcode request for direct path",
		"input_path", request.InputPath,
		"container", request.Container,
		"video_codec", request.VideoCodec,
		"audio_codec", request.AudioCodec,
		"quality", request.Quality,
		"decision_reason", decision.Reason)

	session, err := h.manager.StartTranscode(request)
	if err != nil {
		logger.Error("failed to start transcode", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transcoding session: " + err.Error()})
		return
	}

	logger.Info("transcode session created successfully", "session_id", session.ID)

	// Return the session information
	c.JSON(http.StatusOK, gin.H{
		"id":           session.ID,
		"status":       session.Status,
		"manifest_url": fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", session.ID),
		"provider":     session.Provider,
	})
}

// HandleSeekAhead handles seek-ahead transcoding requests
func (h *APIHandler) HandleSeekAhead(c *gin.Context) {
	logger.Info("HandleSeekAhead called")
	
	// Read the raw body to debug what's being sent
	bodyBytes, err := c.GetRawData()
	if err != nil {
		logger.Error("failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	
	logger.Info("seek-ahead raw request body", "body", string(bodyBytes))
	
	var request struct {
		SessionID    string  `json:"session_id" binding:"required"`
		SeekPosition float64 `json:"seek_position" binding:"required"`
	}

	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		logger.Error("failed to parse JSON request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	logger.Info("seek-ahead request parsed", "session_id", request.SessionID, "seek_position", request.SeekPosition)

	// Get the original session
	originalSession, err := h.manager.GetSession(request.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Get the request from the original session
	originalRequest, err := originalSession.GetRequest()
	if err != nil || originalRequest == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "original session has no request data",
		})
		return
	}

	// Create a new request with the seek offset
	seekRequest := *originalRequest
	seekRequest.Seek = time.Duration(request.SeekPosition) * time.Second
	seekRequest.SessionID = "" // Let the system generate a new session ID

	// Start new session with seek offset
	newSession, err := h.manager.StartTranscode(&seekRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           newSession.ID,
		"status":       newSession.Status,
		"manifest_url": fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", newSession.ID),
		"provider":     newSession.Provider,
	})
}

// HandleGetSession retrieves transcoding session information
func (h *APIHandler) HandleGetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// HandleStopTranscode terminates a transcoding session
func (h *APIHandler) HandleStopTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")

	if err := h.manager.StopSession(sessionID); err != nil {
		logger.Error("failed to stop transcode", "error", err, "session_id", sessionID)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session stopped"})
}

// HandleListSessions returns all active transcoding sessions
func (h *APIHandler) HandleListSessions(c *gin.Context) {
	sessions, err := h.manager.ListSessions()
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// HandleStopAllSessions stops all active transcoding sessions
func (h *APIHandler) HandleStopAllSessions(c *gin.Context) {
	sessions, err := h.manager.ListSessions()
	if err != nil {
		logger.Error("failed to list sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		"stopped_count": stoppedCount,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"cleaned_count": count,
		"max_age_hours": maxAgeHours,
		"message": fmt.Sprintf("Marked %d stale sessions as failed", count),
	})
}

// HandleListOrphanedSessions lists potential orphaned sessions
func (h *APIHandler) HandleListOrphanedSessions(c *gin.Context) {
	// Get all sessions
	sessions, err := h.manager.GetSessionStore().GetActiveSessions()
	if err != nil {
		logger.Error("failed to get active sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Check for orphaned sessions (running/queued for too long)
	var orphaned []gin.H
	threshold := 30 * time.Minute
	
	for _, session := range sessions {
		if time.Since(session.UpdatedAt) > threshold {
			orphaned = append(orphaned, gin.H{
				"id": session.ID,
				"status": session.Status,
				"provider": session.Provider,
				"last_update": session.UpdatedAt,
				"age": time.Since(session.UpdatedAt).String(),
				"directory": session.DirectoryPath,
			})
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"orphaned_sessions": orphaned,
		"count": len(orphaned),
		"threshold": threshold.String(),
	})
}

// HandleGetStats returns transcoding statistics
func (h *APIHandler) HandleGetStats(c *gin.Context) {
	stats, err := h.manager.GetStats()
	if err != nil {
		logger.Error("failed to get stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validation, err := h.manager.ValidateMediaFile(c.Request.Context(), request.MediaPath, request.Quick)

	if err != nil {
		logger.Error("Media validation failed", "error", err, "path", request.MediaPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		providerManager := h.manager.transcodingService.GetProviderManager()
		if providerManager != nil {
			providers := providerManager.GetProviders()
			providerCount = len(providers)
			for id, p := range providers {
				info := p.GetInfo()
				providerNames = append(providerNames, fmt.Sprintf("%s (%s)", info.Name, id))
			}
		}
	}
	
	// Determine readiness
	ready := h.manager.IsEnabled() && h.manager.initialized && providerCount > 0
	status := "healthy"
	if !ready {
		status = "degraded"
	}
	
	health := gin.H{
		"status":    status,
		"ready":     ready,
		"enabled":   h.manager.IsEnabled(),
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "plugins refreshed successfully"})
}

// HandleManualCleanup triggers manual cleanup
func (h *APIHandler) HandleManualCleanup(c *gin.Context) {
	// Get cleanup service from manager
	cleanupService := h.manager.GetCleanupService()
	if cleanupService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cleanup service not available"})
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stats, _ := cleanupService.GetCleanupStats()

	c.JSON(http.StatusOK, gin.H{
		"message": "manual cleanup triggered",
		"cleaned_sessions": count,
		"retention_hours": retentionHours,
		"stats":   stats,
	})
}

// HandleCleanupStats returns cleanup statistics
func (h *APIHandler) HandleCleanupStats(c *gin.Context) {
	cleanupService := h.manager.GetCleanupService()
	if cleanupService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cleanup service not available"})
		return
	}

	stats, err := cleanupService.GetCleanupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Streaming handlers

// HandleStreamTranscode streams transcoded video data
func (h *APIHandler) HandleStreamTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// Get the session information
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// For DASH/HLS sessions, redirect to manifest
	if session.Request != "" {
		request, err := session.GetRequest()
		if err == nil && request != nil && (request.Container == "dash" || request.Container == "hls") {
			manifestURL := fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID)
			if request.Container == "hls" {
				manifestURL = fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", sessionID)
			}
			c.Redirect(http.StatusFound, manifestURL)
			return
		}
	}

	// Progressive streaming not implemented in clean architecture
	c.JSON(http.StatusNotImplemented, gin.H{"error": "progressive streaming not implemented"})
}

// HandleDashManifest serves DASH manifest files
func (h *APIHandler) HandleDashManifest(c *gin.Context) {
	sessionID := c.Param("sessionId")
	h.serveManifestFile(c, sessionID, "manifest.mpd")
}

// HandleHlsPlaylist serves HLS playlist files
func (h *APIHandler) HandleHlsPlaylist(c *gin.Context) {
	sessionID := c.Param("sessionId")
	h.serveManifestFile(c, sessionID, "playlist.m3u8")
}

// HandleSegment serves DASH/HLS segments
func (h *APIHandler) HandleSegment(c *gin.Context) {
	sessionID := c.Param("sessionId")
	segmentName := c.Param("segmentName")
	h.serveSegmentFile(c, sessionID, segmentName)
}

// HandleDashSegmentSpecific serves DASH segments with specific naming pattern
func (h *APIHandler) HandleDashSegmentSpecific(c *gin.Context) {
	sessionID := c.Param("sessionId")
	segmentFile := c.Param("segmentFile")
	h.serveSegmentFile(c, sessionID, segmentFile)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	var logFilename string
	switch logType {
	case "stdout":
		logFilename = "ffmpeg-stdout.log"
	case "stderr":
		logFilename = "ffmpeg-stderr.log"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log type, use 'stdout' or 'stderr'"})
		return
	}

	logPath := filepath.Join(h.getSessionDirectory(sessionID, session), logFilename)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "log file not found"})
		return
	}

	// Read log file
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		logger.Error("failed to read log file", "path", logPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read log file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"log_type":   logType,
		"content":    string(logContent),
	})
}

// Helper methods

func (h *APIHandler) serveManifestFile(c *gin.Context, sessionID, filename string) {
	logger.Info("serveManifestFile called", "session_id", sessionID, "filename", filename, "method", c.Request.Method)
	
	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		logger.Error("session not found in serveManifestFile", "session_id", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	manifestPath := filepath.Join(h.getSessionDirectory(sessionID, session), filename)

	// Check if file exists
	fileInfo, err := os.Stat(manifestPath)
	if os.IsNotExist(err) {
		logger.Warn("manifest file not found", "path", manifestPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "manifest not found"})
		return
	}

	// Set appropriate content type
	contentType := "application/dash+xml"
	if filename == "playlist.m3u8" {
		contentType = "application/vnd.apple.mpegurl"
	}

	// Low-latency streaming optimizations
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Transfer-Encoding", "chunked")                    // Enable chunked transfer
	c.Header("Access-Control-Allow-Origin", "*")               // CORS for streaming
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
	c.Header("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")
	
	// DASH-specific low-latency headers
	if contentType == "application/dash+xml" {
		c.Header("X-Suggested-Presentation-Delay", "1.0")      // 1 second delay for low latency
	}
	
	// HLS-specific low-latency headers  
	if contentType == "application/vnd.apple.mpegurl" {
		c.Header("X-Playlist-Type", "VOD")
		c.Header("X-Version", "7")                              // Support for EXT-X-PART
	}
	
	// Handle HEAD requests properly
	if c.Request.Method == "HEAD" {
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		c.Status(http.StatusOK)
		return
	}
	
	// For DASH manifests, inject BaseURL
	if contentType == "application/dash+xml" {
		// Read the manifest
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			logger.Error("failed to read manifest file", "path", manifestPath, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read manifest"})
			return
		}
		
		// Get the absolute base URL
		proto := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			proto = "https"
		}
		
		// Get the host, preferring forwarded headers
		host := c.Request.Host
		if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		} else if origin := c.GetHeader("Origin"); origin != "" {
			// Extract host from Origin header if available
			if u, err := url.Parse(origin); err == nil {
				host = u.Host
			}
		} else if referer := c.GetHeader("Referer"); referer != "" {
			// Extract host from Referer header as fallback
			if u, err := url.Parse(referer); err == nil {
				host = u.Host
			}
		}
		
		// Replace backend:8080 with localhost:8080 for local development
		if host == "backend:8080" {
			host = "localhost:8080"
		}
		
		baseURL := fmt.Sprintf("%s://%s/api/playback/stream/%s", proto, host, sessionID)
		modifiedManifest := h.injectBaseURL(manifestData, baseURL)
		
		// Send the modified manifest
		c.Data(http.StatusOK, contentType, modifiedManifest)
		return
	}
	
	c.File(manifestPath)
}

func (h *APIHandler) serveSegmentFile(c *gin.Context, sessionID, segmentName string) {
	logger.Info("serveSegmentFile called", "session_id", sessionID, "segment_name", segmentName)
	
	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		logger.Error("session not found in serveSegmentFile", "session_id", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	sessionDir := h.getSessionDirectory(sessionID, session)
	logger.Info("session directory determined", "session_id", sessionID, "directory", sessionDir, "session_directory_path", session.DirectoryPath)
	
	segmentPath := filepath.Join(sessionDir, segmentName)

	// Check if file exists and get file info
	fileInfo, err := os.Stat(segmentPath)
	if os.IsNotExist(err) {
		logger.Warn("segment file not found", "path", segmentPath)
		
		// List files in the directory for debugging
		if files, listErr := os.ReadDir(sessionDir); listErr == nil {
			var fileNames []string
			for _, f := range files {
				fileNames = append(fileNames, f.Name())
			}
			logger.Warn("files in session directory", "directory", sessionDir, "files", fileNames)
		} else {
			logger.Error("failed to list directory contents", "directory", sessionDir, "error", listErr)
		}
		
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	// Set appropriate content type based on extension
	contentType := "video/mp4"
	switch filepath.Ext(segmentName) {
	case ".m4s":
		contentType = "video/iso.segment"
	case ".ts":
		contentType = "video/mp2t"
	case ".m3u8":
		contentType = "application/vnd.apple.mpegurl"
	}

	// Handle byte-range requests
	rangeHeader := c.Request.Header.Get("Range")
	if rangeHeader != "" {
		h.serveByteRange(c, segmentPath, fileInfo, contentType, rangeHeader)
		return
	}

	// Low-latency segment delivery optimizations
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")                         // Enable range requests
	c.Header("Transfer-Encoding", "chunked")                   // Chunked transfer for better streaming
	c.Header("Access-Control-Allow-Origin", "*")              // CORS support
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
	c.Header("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")
	
	// Different caching strategies based on segment type
	if filepath.Ext(segmentName) == ".m4s" || filepath.Ext(segmentName) == ".ts" {
		// Video/audio segments - cache aggressively
		c.Header("Cache-Control", "public, max-age=31536000, immutable") // 1 year for segments
		c.Header("ETag", fmt.Sprintf("\"%s\"", segmentName))             // Add ETag for validation
	} else {
		// Other files - shorter cache
		c.Header("Cache-Control", "public, max-age=60")                  // 1 minute for other files
	}
	
	// Enable HTTP/2 Server Push hints if available
	c.Header("Link", fmt.Sprintf("<%s>; rel=preload; as=video", segmentName))
	
	// Handle HEAD requests
	if c.Request.Method == "HEAD" {
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		c.Status(http.StatusOK)
		return
	}
	
	c.File(segmentPath)
}

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

// serveByteRange handles HTTP byte-range requests for efficient seeking
func (h *APIHandler) serveByteRange(c *gin.Context, filePath string, fileInfo os.FileInfo, contentType, rangeHeader string) {
	fileSize := fileInfo.Size()
	
	// Parse Range header (e.g., "bytes=0-1023")
	rangePrefix := "bytes="
	if !strings.HasPrefix(rangeHeader, rangePrefix) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range header"})
		return
	}
	
	rangeSpec := rangeHeader[len(rangePrefix):]
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range format"})
		return
	}
	
	var start, end int64
	var err error
	
	// Parse start
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range start"})
			return
		}
	} else {
		// Suffix range (e.g., "-500" means last 500 bytes)
		if parts[1] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range"})
			return
		}
		suffixLength, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLength <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid suffix length"})
			return
		}
		start = fileSize - suffixLength
		if start < 0 {
			start = 0
		}
		end = fileSize - 1
	}
	
	// Parse end
	if parts[1] != "" && parts[0] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range end"})
			return
		}
	} else if parts[0] != "" {
		// Open-ended range (e.g., "1024-")
		end = fileSize - 1
	}
	
	// Validate range
	if start > end || start >= fileSize {
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"error": "range not satisfiable"})
		return
	}
	if end >= fileSize {
		end = fileSize - 1
	}
	
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer file.Close()
	
	// Seek to start position
	_, err = file.Seek(start, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to seek"})
		return
	}
	
	// Set response headers
	contentLength := end - start + 1
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", fmt.Sprintf("\"%s-%d\"", fileInfo.Name(), fileInfo.ModTime().Unix()))
	
	// CORS headers for byte-range requests
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
	c.Header("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")
	
	// Set status to 206 Partial Content
	c.Status(http.StatusPartialContent)
	
	// Stream the requested range
	io.CopyN(c.Writer, file, contentLength)
}

// parseRangeHeader parses an HTTP Range header
func parseRangeHeader(rangeHeader string, fileSize int64) (start, end int64, err error) {
	// Simple implementation - extend as needed
	rangePrefix := "bytes="
	if !strings.HasPrefix(rangeHeader, rangePrefix) {
		return 0, 0, fmt.Errorf("invalid range header")
	}
	
	rangeSpec := rangeHeader[len(rangePrefix):]
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}
	
	// Parse start
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	}
	
	// Parse end
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
	} else {
		end = fileSize - 1
	}
	
	return start, end, nil
}

// injectBaseURL injects a BaseURL element into a DASH manifest
func (h *APIHandler) injectBaseURL(manifest []byte, baseURL string) []byte {
	manifestStr := string(manifest)
	
	// Check if BaseURL already exists
	if strings.Contains(manifestStr, "<BaseURL>") {
		return manifest
	}
	
	// Find the insertion point (after the ServiceDescription closing tag)
	insertPoint := strings.Index(manifestStr, "</ServiceDescription>")
	if insertPoint == -1 {
		// If no ServiceDescription, insert after ProgramInformation closing tag
		insertPoint = strings.Index(manifestStr, "</ProgramInformation>")
		if insertPoint == -1 {
			// If no ProgramInformation, insert after MPD opening tag
			insertPoint = strings.Index(manifestStr, ">")
			if insertPoint != -1 {
				insertPoint += 1
			}
		} else {
			insertPoint += len("</ProgramInformation>")
		}
	} else {
		insertPoint += len("</ServiceDescription>")
	}
	
	// Create the BaseURL element
	baseURLElement := fmt.Sprintf("\n\t<BaseURL>%s/</BaseURL>", baseURL)
	
	// Insert the BaseURL
	result := manifestStr[:insertPoint] + baseURLElement + manifestStr[insertPoint:]
	
	return []byte(result)
}

