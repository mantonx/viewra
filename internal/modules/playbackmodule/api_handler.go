package playbackmodule

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
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
		MediaFileID  string  `json:"media_file_id"`
		Container    string  `json:"container"`
		SeekPosition float64 `json:"seek_position,omitempty"` // Optional seek position in seconds
	}
	
	parseErr := json.Unmarshal(bodyBytes, &mediaRequest)
	logger.Info("media request parse result", "error", parseErr, "media_file_id", mediaRequest.MediaFileID, "container", mediaRequest.Container)
	
	if parseErr == nil && mediaRequest.MediaFileID != "" {
		// Handle media file based request
		logger.Info("handling media file based request", "media_file_id", mediaRequest.MediaFileID, "container", mediaRequest.Container, "seek_position", mediaRequest.SeekPosition)
		session, err := h.manager.StartTranscodeFromMediaFile(mediaRequest.MediaFileID, mediaRequest.Container, mediaRequest.SeekPosition)
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
	
	// Fall back to direct transcode request
	var request plugins.TranscodeRequest
	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		logger.Error("failed to parse JSON request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.manager.StartTranscode(&request)
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

	stats, err := cleanupService.GetCleanupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "manual cleanup triggered",
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

// Helper methods

func (h *APIHandler) serveManifestFile(c *gin.Context, sessionID, filename string) {
	logger.Info("serveManifestFile called", "session_id", sessionID, "filename", filename)
	
	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		logger.Error("session not found in serveManifestFile", "session_id", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	manifestPath := filepath.Join(h.getSessionDirectory(sessionID, session), filename)

	// Check if file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
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
	
	c.File(manifestPath)
}

func (h *APIHandler) serveSegmentFile(c *gin.Context, sessionID, segmentName string) {
	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	segmentPath := filepath.Join(h.getSessionDirectory(sessionID, session), segmentName)

	// Check if file exists
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		logger.Warn("segment file not found", "path", segmentPath)
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
	
	c.File(segmentPath)
}

func (h *APIHandler) getSessionDirectory(sessionID string, session *database.TranscodeSession) string {
	cfg := config.Get()

	// If session has directory path, use it
	if session != nil && session.DirectoryPath != "" {
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

