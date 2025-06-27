package playbackmodule

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/services"
)

// ContentAPIHandlerV2 handles content-addressable storage API endpoints using service interfaces
type ContentAPIHandlerV2 struct {
	contentStore services.ContentStore
	sessionStore SessionStoreInterface
}

// NewContentAPIHandlerV2 creates a new content API handler with service interfaces
func NewContentAPIHandlerV2(contentStore services.ContentStore, sessionStore SessionStoreInterface) *ContentAPIHandlerV2 {
	return &ContentAPIHandlerV2{
		contentStore: contentStore,
		sessionStore: sessionStore,
	}
}

// ServeContent serves content files using content-addressable URLs
// GET /api/v1/content/:hash/:file
func (h *ContentAPIHandlerV2) ServeContent(c *gin.Context) {
	contentHash := c.Param("hash")
	fileName := c.Param("file")

	// Validate content hash format (simple check)
	if len(contentHash) != 64 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content hash format",
		})
		return
	}

	// Check if content store is available
	if h.contentStore == nil {
		logger.Warn("Content store not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get content metadata and path
	metadata, contentPath, err := h.contentStore.Get(contentHash)
	if err != nil {
		logger.Warn("Content not found", "hash", contentHash, "error", err)
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

	// Extract format from metadata if possible
	format := ""
	if metaMap, ok := metadata.(map[string]interface{}); ok {
		if f, ok := metaMap["format"].(string); ok {
			format = f
		}
	}

	// Set appropriate headers based on file type
	h.setContentHeaders(c, fileName, format)

	// Serve the file
	c.File(filePath)
}

// GetContentInfo returns metadata about stored content
// GET /api/v1/content/:hash/info
func (h *ContentAPIHandlerV2) GetContentInfo(c *gin.Context) {
	contentHash := c.Param("hash")

	// Validate content hash format
	if len(contentHash) != 64 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content hash format",
		})
		return
	}

	// Check if content store is available
	if h.contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get content metadata
	metadata, _, err := h.contentStore.Get(contentHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Content not found",
		})
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// ListContentByMediaID returns all content versions for a media ID
// GET /api/v1/content/by-media/:mediaId
func (h *ContentAPIHandlerV2) ListContentByMediaID(c *gin.Context) {
	mediaID := c.Param("mediaId")

	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Media ID is required",
		})
		return
	}

	// Check if content store is available
	if h.contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get all content for this media ID
	contentList, err := h.contentStore.ListByMediaID(mediaID)
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
func (h *ContentAPIHandlerV2) GetContentStats(c *gin.Context) {
	// Check if content store is available
	if h.contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	stats, err := h.contentStore.GetStats()
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
func (h *ContentAPIHandlerV2) CleanupExpiredContent(c *gin.Context) {
	// Check if content store is available
	if h.contentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Get list of expired content
	expired, err := h.contentStore.ListExpired()
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

	for _, content := range expired {
		// Extract hash from content metadata
		var contentHash string
		if metaMap, ok := content.(map[string]interface{}); ok {
			if hash, ok := metaMap["hash"].(string); ok {
				contentHash = hash
			}
		}

		if contentHash != "" {
			if err := h.contentStore.Delete(contentHash); err != nil {
				errors = append(errors, contentHash)
				logger.Error("Failed to delete expired content", "hash", contentHash, "error", err)
			} else {
				removed = append(removed, contentHash)
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
func (h *ContentAPIHandlerV2) HandleSessionContent(c *gin.Context) {
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
	h.setContentHeaders(c, fileName, "")

	// Override cache headers for session-based content (not permanent)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("ETag", `"`+sessionID+`"`)

	// Serve the file
	c.File(filePath)
}

// setContentHeaders sets appropriate HTTP headers based on file type
func (h *ContentAPIHandlerV2) setContentHeaders(c *gin.Context, fileName string, format string) {
	// Set cache headers for static content (only for content-addressable URLs)
	if hash := c.Param("hash"); hash != "" {
		c.Header("Cache-Control", "public, max-age=31536000") // 1 year
		c.Header("ETag", `"`+hash+`"`)
	}

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

// RegisterContentRoutesV2 registers content API routes using the V2 handler
func RegisterContentRoutesV2(router *gin.Engine, handler *ContentAPIHandlerV2) {
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

	// Support for legacy URL patterns
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
