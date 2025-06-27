package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	httputil "github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/http"
	"github.com/mantonx/viewra/internal/services"
)

// ContentAPIHandler handles content-addressable storage API endpoints
type ContentAPIHandler struct {
	contentStore services.ContentStore
	sessionStore services.SessionStore
	urlGenerator *storage.URLGenerator
}

// NewContentAPIHandler creates a new content API handler
func NewContentAPIHandler(contentStore services.ContentStore, sessionStore services.SessionStore) *ContentAPIHandler {
	return &ContentAPIHandler{
		contentStore: contentStore,
		sessionStore: sessionStore,
		urlGenerator: storage.NewURLGenerator("/api/v1", false, ""),
	}
}

// ServeContent serves content files using content-addressable URLs
// GET /api/v1/content/:hash/:file
func (h *ContentAPIHandler) ServeContent(c *gin.Context) {
	contentHash := c.Param("hash")
	fileName := c.Param("file")

	logger.Info("ContentAPIHandler.ServeContent called")
	logger.Info("Request details", 
		"hash", contentHash, 
		"file", fileName,
		"path", c.Request.URL.Path,
		"contentStore", h.contentStore != nil,
		"sessionStore", h.sessionStore != nil)

	// Validate content hash format (SHA256 = 64 chars)
	if len(contentHash) != 64 {
		logger.Warn("Invalid content hash length", "hash", contentHash, "length", len(contentHash))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content hash format",
		})
		return
	}
	logger.Info("Content hash validated", "hash", contentHash)

	// Check if content store is available
	if h.contentStore == nil {
		logger.Warn("Content store not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Content store not available",
		})
		return
	}

	// Try to get content metadata and path from storage
	logger.Info("Attempting to get content from store", "hash", contentHash)
	_, contentPath, err := h.contentStore.Get(contentHash)
	logger.Info("Content store result", "hash", contentHash, "contentPath", contentPath, "error", err)
	if err != nil {
		// Check if content files exist even without metadata
		// This handles the case where content was stored but metadata wasn't saved
		contentPath = h.getContentPath(contentHash)
		
		// Determine the correct subdirectory based on file type
		var directPath string
		switch {
		case strings.HasSuffix(fileName, ".mpd") || strings.HasSuffix(fileName, ".m3u8"):
			directPath = filepath.Join(contentPath, "manifests", fileName)
		case strings.Contains(fileName, "init-"):
			directPath = filepath.Join(contentPath, "init", fileName)
		case strings.Contains(fileName, "segment-") && strings.HasSuffix(fileName, ".m4s"):
			directPath = filepath.Join(contentPath, "video", fileName)
		default:
			directPath = filepath.Join(contentPath, fileName)
		}
		
		logger.Info("Checking for content without metadata", "path", directPath)
		
		if _, statErr := os.Stat(directPath); statErr == nil {
			logger.Info("Found content files without metadata, serving directly")
			// Set headers for streaming content
			httputil.SetContentHeaders(c, fileName)
			httputil.SetCacheHeaders(c, true, contentHash)
			c.File(directPath)
			return
		}
		logger.Info("Content not in store, trying session fallback", 
			"hash", contentHash, 
			"file", fileName,
			"error", err.Error())
		// Content not in storage yet, try to serve from active session
		if h.sessionStore != nil {
			sessionPath, fallbackErr := h.trySessionFallback(contentHash, fileName)
			if fallbackErr == nil {
				logger.Info("Serving content from active session", "hash", contentHash, "file", fileName, "sessionPath", sessionPath)
				// Set headers for streaming content (shorter cache time)
				httputil.SetContentHeaders(c, fileName)
				httputil.SetCacheHeaders(c, false, "") // Don't cache session content
				c.File(sessionPath)
				return
			} else {
				logger.Warn("Session fallback failed", "hash", contentHash, "error", fallbackErr)
			}
		} else {
			logger.Warn("Session store not available for fallback")
		}
		
		logger.Warn("Content not found", "hash", contentHash, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Content not found",
		})
		return
	}

	// Determine the correct subdirectory based on file type
	var filePath string
	switch {
	case strings.HasSuffix(fileName, ".mpd") || strings.HasSuffix(fileName, ".m3u8"):
		// Manifest files
		filePath = filepath.Join(contentPath, "manifests", fileName)
	case strings.Contains(fileName, "init-"):
		// Init segments
		filePath = filepath.Join(contentPath, "init", fileName)
	case strings.Contains(fileName, "segment-") && strings.HasSuffix(fileName, ".m4s"):
		// Video segments
		filePath = filepath.Join(contentPath, "video", fileName)
	case strings.HasSuffix(fileName, ".m4s") || strings.HasSuffix(fileName, ".ts"):
		// Try video directory first, then audio
		videoPath := filepath.Join(contentPath, "video", fileName)
		if _, err := os.Stat(videoPath); err == nil {
			filePath = videoPath
		} else {
			filePath = filepath.Join(contentPath, "audio", fileName)
		}
	default:
		// Default to root
		filePath = filepath.Join(contentPath, fileName)
	}

	logger.Info("Attempting to serve file from content store", "filePath", filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("File not found in content store", "filePath", filePath, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return
	}

	// Security check: ensure file is within content directory
	if !strings.HasPrefix(filePath, contentPath) {
		logger.Warn("Path traversal attempt", "hash", contentHash, "file", fileName)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Set appropriate headers based on file type
	httputil.SetContentHeaders(c, fileName)

	// Content-addressable storage can be cached forever
	httputil.SetCacheHeaders(c, true, contentHash)

	// Serve the file
	c.File(filePath)
}

// GetContentInfo returns metadata about stored content
// GET /api/v1/content/:hash/info
func (h *ContentAPIHandler) GetContentInfo(c *gin.Context) {
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
func (h *ContentAPIHandler) ListContentByMediaID(c *gin.Context) {
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
func (h *ContentAPIHandler) GetContentStats(c *gin.Context) {
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

// getContentPath returns the filesystem path for content (mirroring ContentStore logic)
func (h *ContentAPIHandler) getContentPath(contentHash string) string {
	// Use first 2 characters for sharding to avoid too many files in one directory
	return filepath.Join("/app/viewra-data/transcoding/content", contentHash[:2], contentHash)
}

// trySessionFallback attempts to serve content from an active transcoding session
func (h *ContentAPIHandler) trySessionFallback(contentHash, fileName string) (string, error) {
	logger.Info("trySessionFallback called", "hash", contentHash, "file", fileName)
	
	// Find active sessions with this content hash
	sessions, err := h.sessionStore.ListActiveSessionsByContentHash(contentHash)
	if err != nil {
		logger.Error("Failed to list active sessions", "error", err)
		return "", err
	}
	
	logger.Info("Found sessions", "count", len(sessions), "hash", contentHash)
	
	if len(sessions) == 0 {
		return "", fmt.Errorf("no active sessions found for content hash %s", contentHash)
	}
	
	// Use the first active session
	session := sessions[0]
	logger.Info("Using session", "sessionID", session.ID, "status", session.Status, "contentHash", session.ContentHash, "directoryPath", session.DirectoryPath)
	
	// Use the actual directory path from the session
	var sessionDir string
	if session.DirectoryPath != "" {
		// DirectoryPath is just the directory name, not the full path
		// Construct full path
		sessionDir = filepath.Join("/app/viewra-data/transcoding", session.DirectoryPath)
	} else {
		// Fallback: construct expected path based on container and provider
		// Format: {container}_{provider}_{sessionID}
		sessionDir = filepath.Join("/app/viewra-data/transcoding", fmt.Sprintf("dash_streaming_pipeline_%s", session.ID))
	}
	filePath := filepath.Join(sessionDir, fileName)
	
	logger.Info("Checking file", "path", filePath)
	
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("File not found in session", "path", filePath, "error", err)
		return "", fmt.Errorf("file not found in session directory: %w", err)
	}
	
	logger.Info("File found in session", "path", filePath)
	return filePath, nil
}
