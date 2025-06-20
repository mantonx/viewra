package playbackmodule

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/pkg/plugins"
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
	var request plugins.TranscodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("failed to bind JSON request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Info("JSON request bound successfully", "input_path", request.InputPath, "container", request.Container)

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
	var request struct {
		SessionID    string  `json:"session_id" binding:"required"`
		SeekPosition float64 `json:"seek_position" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the original session
	originalSession, err := h.manager.GetSession(request.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Create a new transcode request with seek offset
	if originalSession.Request == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no request data in session"})
		return
	}

	seekRequest := *originalSession.Request
	seekRequest.Seek = time.Duration(request.SeekPosition) * time.Second

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
	health := gin.H{
		"status":  "healthy",
		"enabled": h.manager.IsEnabled(),
		"uptime":  time.Since(time.Now()).String(), // This would be tracked properly in real implementation
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
	if session.Request != nil && (session.Request.Container == "dash" || session.Request.Container == "hls") {
		manifestURL := fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID)
		if session.Request.Container == "hls" {
			manifestURL = fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", sessionID)
		}
		c.Redirect(http.StatusFound, manifestURL)
		return
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
	// Get the session to find directory
	session, err := h.manager.GetSession(sessionID)
	if err != nil {
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

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
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

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600")
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
	if session != nil && session.Request != nil {
		switch session.Request.Container {
		case "dash":
			dirName = fmt.Sprintf("dash_%s_%s", session.Provider, sessionID)
		case "hls":
			dirName = fmt.Sprintf("hls_%s_%s", session.Provider, sessionID)
		default:
			dirName = fmt.Sprintf("software_%s_%s", session.Provider, sessionID)
		}
	} else {
		// Fallback - try to find directory
		dirName = h.findSessionDirectory(sessionID)
	}

	return filepath.Join(cfg.Transcoding.DataDir, dirName)
}

func (h *APIHandler) findSessionDirectory(sessionID string) string {
	cfg := config.Get()

	// Try common patterns
	patterns := []string{
		fmt.Sprintf("dash_*_%s", sessionID),
		fmt.Sprintf("hls_*_%s", sessionID),
		fmt.Sprintf("software_*_%s", sessionID),
		fmt.Sprintf("*_%s", sessionID),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(cfg.Transcoding.DataDir, pattern))
		if err == nil && len(matches) > 0 {
			return filepath.Base(matches[0])
		}
	}

	// Default fallback
	return sessionID
}
