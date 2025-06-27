package playbackmodule

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/services"
)

// ContentAPIHandler handles content-addressable storage API endpoints
type ContentAPIHandler struct {
	sessionStore SessionStoreInterface
}

// SessionStoreInterface defines the interface for accessing session data
type SessionStoreInterface interface {
	GetSession(sessionID string) (*database.TranscodeSession, error)
	ListActiveSessionsByContentHash(contentHash string) ([]*database.TranscodeSession, error)
}

// NewContentAPIHandler creates a new content API handler
func NewContentAPIHandler(sessionStore SessionStoreInterface) *ContentAPIHandler {
	return &ContentAPIHandler{
		sessionStore: sessionStore,
	}
}

// ServeContent serves content files using content-addressable URLs
// GET /api/v1/content/:hash/:file
func (h *ContentAPIHandler) ServeContent(c *gin.Context) {
	contentHash := c.Param("hash")
	fileName := c.Param("file")

	logger.Info("ServeContent called", "hash", contentHash, "file", fileName, "hasSessionStore", h.sessionStore != nil)

	// Validate content hash format (simple validation)
	if len(contentHash) < 16 || len(contentHash) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content hash format",
		})
		return
	}

	// Get transcoding service from registry
	transcodingService, err := services.GetTranscodingService()
	if err != nil {
		logger.Warn("Transcoding service not available", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transcoding service not available",
		})
		return
	}

	// Get content store from transcoding service
	contentStore := transcodingService.GetContentStore()
	if contentStore == nil {
		logger.Warn("Content store not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get content metadata and path
	_, contentPath, err := contentStore.Get(contentHash)
	if err != nil {
		logger.Warn("Content not found in store, trying session fallback", "hash", contentHash, "error", err)
		
		// Try session fallback - look for active sessions with this content hash
		if sessionPath, fallbackErr := h.trySessionFallback(contentHash, fileName); fallbackErr == nil {
			logger.Info("Serving content from session fallback", "hash", contentHash, "file", fileName, "sessionPath", sessionPath)
			
			// Set appropriate headers for session content (less aggressive caching)
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			
			// Set content type based on file extension
			h.setContentHeaders(c, fileName)
			
			// Serve the file from session directory
			c.File(sessionPath)
			return
		}
		
		// If session fallback also failed, return 404
		logger.Warn("Content not found in store or active sessions", "hash", contentHash, "file", fileName)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Content not found",
		})
		return
	}

	// Construct file path
	filePath := filepath.Join(contentPath, fileName)

	// Security check: ensure file is within content directory
	if !strings.HasPrefix(filePath, contentPath) {
		logger.Warn("Path traversal attempt", "hash", contentHash, "file", fileName)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Set appropriate headers based on file type
	h.setContentHeaders(c, fileName)

	// Serve the file
	c.File(filePath)
}

// GetContentInfo returns metadata about stored content
// GET /api/v1/content/:hash/info
func (h *ContentAPIHandler) GetContentInfo(c *gin.Context) {
	contentHash := c.Param("hash")

	// Validate content hash format (simple validation)
	if len(contentHash) < 16 || len(contentHash) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content hash format",
		})
		return
	}

	// Get transcoding service from registry
	transcodingService, err := services.GetTranscodingService()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transcoding service not available",
		})
		return
	}

	// Get content store from transcoding service
	contentStore := transcodingService.GetContentStore()
	if contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get content metadata
	metadataInterface, _, err := contentStore.Get(contentHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Content not found",
		})
		return
	}

	// Return the metadata as-is (it's an interface{})
	c.JSON(http.StatusOK, metadataInterface)
}

// ListContentByMediaID returns all content versions for a media ID
// GET /api/v1/content/by-media/:mediaId
func (h *ContentAPIHandler) ListContentByMediaID(c *gin.Context) {
	mediaID := c.Param("mediaId")

	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Media ID is required",
		})
		return
	}

	// Get transcoding service from registry
	transcodingService, err := services.GetTranscodingService()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transcoding service not available",
		})
		return
	}

	// Get content store from transcoding service
	contentStore := transcodingService.GetContentStore()
	if contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get all content for this media ID
	contentList, err := contentStore.ListByMediaID(mediaID)
	if err != nil {
		logger.Error("Failed to list content by media ID", "mediaID", mediaID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve content",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media_id": mediaID,
		"content":  contentList,
		"count":    len(contentList),
	})
}

// GetContentStats returns storage statistics
// GET /api/v1/content/stats
func (h *ContentAPIHandler) GetContentStats(c *gin.Context) {
	// Get transcoding service from registry
	transcodingService, err := services.GetTranscodingService()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transcoding service not available",
		})
		return
	}

	// Get content store from transcoding service
	contentStore := transcodingService.GetContentStore()
	if contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	stats, err := contentStore.GetStats()
	if err != nil {
		logger.Error("Failed to get content stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CleanupExpiredContent removes expired content
// POST /api/v1/content/cleanup
func (h *ContentAPIHandler) CleanupExpiredContent(c *gin.Context) {
	// Get transcoding service from registry
	transcodingService, err := services.GetTranscodingService()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transcoding service not available",
		})
		return
	}

	// Get content store from transcoding service
	contentStore := transcodingService.GetContentStore()
	if contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get list of expired content
	expired, err := contentStore.ListExpired()
	if err != nil {
		logger.Error("Failed to list expired content", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list expired content",
		})
		return
	}

	// Remove expired content
	var removed []string
	var errors []string

	// Extract hash from each content item (interface{})
	for _, item := range expired {
		// Try to extract hash from the interface{}
		// This is a simplified approach - in reality you'd need proper type assertion
		if contentMap, ok := item.(map[string]interface{}); ok {
			if hash, ok := contentMap["hash"].(string); ok {
				if err := contentStore.Delete(hash); err != nil {
					errors = append(errors, hash)
					logger.Error("Failed to delete expired content", "hash", hash, "error", err)
				} else {
					removed = append(removed, hash)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"removed_count": len(removed),
		"error_count":   len(errors),
		"removed":       removed,
		"errors":        errors,
	})
}

// HandleSessionContent handles session-based content URLs
// These are temporary URLs used before content hash is available
// GET /api/v1/sessions/:sessionId/:file
func (h *ContentAPIHandler) HandleSessionContent(c *gin.Context) {
	sessionID := c.Param("sessionId")
	fileName := c.Param("file")

	logger.Info("HandleSessionContent called", "sessionID", sessionID, "file", fileName)

	// Get session from session store
	if h.sessionStore == nil {
		logger.Error("Session store not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Session store not available",
		})
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		logger.Warn("Session not found", "sessionID", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
		})
		return
	}

	// Check if session has content hash and redirect if available
	if session.ContentHash != "" {
		// Redirect to content-addressable URL
		redirectURL := "/api/v1/content/" + session.ContentHash + "/" + fileName
		logger.Info("Redirecting to content URL", "sessionID", sessionID, "contentHash", session.ContentHash, "redirectURL", redirectURL)
		c.Redirect(http.StatusMovedPermanently, redirectURL)
		return
	}

	// Get session directory from session
	sessionDir := session.DirectoryPath
	if sessionDir == "" {
		logger.Error("Session has no directory path", "sessionID", sessionID)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session directory not found",
		})
		return
	}

	// Construct file path
	filePath := filepath.Join(sessionDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("File not found in session directory", "sessionID", sessionID, "file", fileName, "path", filePath)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return
	}

	// Security check: ensure file is within session directory
	if !strings.HasPrefix(filePath, sessionDir) {
		logger.Warn("Path traversal attempt", "sessionID", sessionID, "file", fileName)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Set appropriate headers based on file type
	h.setContentHeaders(c, fileName)

	// Override cache headers for session-based content (not permanent)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("ETag", `"`+sessionID+`"`)

	// Serve the file
	c.File(filePath)
}

// setContentHeaders sets appropriate HTTP headers based on file type
func (h *ContentAPIHandler) setContentHeaders(c *gin.Context, fileName string) {
	// Set cache headers for static content
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year
	c.Header("ETag", `"`+c.Param("hash")+`"`)

	// Set content type based on file extension
	switch {
	case strings.HasSuffix(fileName, ".mpd"):
		c.Header("Content-Type", "application/dash+xml")
		c.Header("Access-Control-Allow-Origin", "*") // Required for DASH
		c.Header("Access-Control-Allow-Headers", "Content-Type")

	case strings.HasSuffix(fileName, ".m3u8"):
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Access-Control-Allow-Origin", "*") // Required for HLS
		c.Header("Access-Control-Allow-Headers", "Content-Type")

	case strings.HasSuffix(fileName, ".m4s"):
		c.Header("Content-Type", "video/iso.segment")

	case strings.HasSuffix(fileName, ".ts"):
		c.Header("Content-Type", "video/mp2t")

	case strings.HasSuffix(fileName, ".mp4"):
		c.Header("Content-Type", "video/mp4")

	case strings.HasSuffix(fileName, ".webm"):
		c.Header("Content-Type", "video/webm")

	default:
		// Let Gin determine the content type
	}

	// Add security headers
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
}

// trySessionFallback attempts to serve content from an active transcoding session
func (h *ContentAPIHandler) trySessionFallback(contentHash, fileName string) (string, error) {
	if h.sessionStore == nil {
		return "", fmt.Errorf("session store not available")
	}

	// Find active sessions with this content hash
	sessions, err := h.sessionStore.ListActiveSessionsByContentHash(contentHash)
	if err != nil {
		return "", fmt.Errorf("failed to list active sessions: %w", err)
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf("no active sessions found for content hash %s", contentHash)
	}

	// Use the first active session
	session := sessions[0]

	// Construct session directory path: /app/viewra-data/transcoding/sessions/{sessionID}
	sessionDir := filepath.Join("/app/viewra-data/transcoding/sessions", session.ID)
	filePath := filepath.Join(sessionDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file not found in session directory: %w", err)
	}

	return filePath, nil
}

// RegisterContentRoutes registers content API routes
func RegisterContentRoutes(router *gin.Engine, handler *ContentAPIHandler) {
	contentGroup := router.Group("/api/v1/content")
	{
		// Serve content files
		contentGroup.GET("/:hash/:file", handler.ServeContent)

		// Content metadata
		contentGroup.GET("/:hash/info", handler.GetContentInfo)

		// List content by media ID
		contentGroup.GET("/by-media/:mediaId", handler.ListContentByMediaID)

		// Storage statistics
		contentGroup.GET("/stats", handler.GetContentStats)

		// Cleanup operations
		contentGroup.POST("/cleanup", handler.CleanupExpiredContent)
	}

	// Support for legacy URL patterns (can be removed later)
	legacyGroup := router.Group("/content")
	{
		legacyGroup.GET("/:hash/*file", func(c *gin.Context) {
			// Redirect to new URL pattern
			hash := c.Param("hash")
			file := strings.TrimPrefix(c.Param("file"), "/")
			newURL := "/api/v1/content/" + hash + "/" + file
			c.Redirect(http.StatusMovedPermanently, newURL)
		})
	}
}
