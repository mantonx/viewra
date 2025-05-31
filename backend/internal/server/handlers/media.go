// Media handler with event support
package handlers

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/utils"
)

// MediaHandler handles media-related API endpoints
type MediaHandler struct {
	eventBus events.EventBus
}

// NewMediaHandler creates a new media handler with event bus
func NewMediaHandler(eventBus events.EventBus) *MediaHandler {
	return &MediaHandler{
		eventBus: eventBus,
	}
}

// GetMedia retrieves all media items with associated user information
func (h *MediaHandler) GetMedia(c *gin.Context) {
	var mediaFiles []database.MediaFile
	db := database.GetDB()

	result := db.Preload("MusicMetadata").Find(&mediaFiles)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve media",
			"details": result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"media": mediaFiles,
		"count": len(mediaFiles),
	})
}

// GetMediaByID retrieves a specific media file by ID
func (h *MediaHandler) GetMediaByID(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := strconv.ParseUint(mediaIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media ID",
		})
		return
	}

	// Get the media file from database with music metadata
	var mediaFile database.MediaFile
	db := database.GetDB()
	result := db.Preload("MusicMetadata").First(&mediaFile, mediaID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	c.JSON(http.StatusOK, mediaFile)
}

// StreamMedia serves the actual media file content for streaming
func (h *MediaHandler) StreamMedia(c *gin.Context) {
	mediaIDStr := c.Param("id")
	mediaID, err := strconv.ParseUint(mediaIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media ID",
		})
		return
	}

	// Get the media file from database
	var mediaFile database.MediaFile
	db := database.GetDB()
	result := db.First(&mediaFile, mediaID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Resolve the file path using the path resolver
	pathResolver := utils.NewPathResolver()
	validPath, err := pathResolver.ResolvePath(mediaFile.Path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
			"path":  mediaFile.Path,
		})
		return
	}

	// Open the resolved file
	file, err := os.Open(validPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open media file",
		})
		return
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get file info",
		})
		return
	}

	// Get content type using utility
	contentType := utils.GetContentType(validPath)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(validPath)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Set headers for streaming
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")

	// Consider it a playback start if it's a media file
	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		// Get user info if available
		userID := uint(0) // Default to 0 for anonymous

		// Publish playback started event
		if h.eventBus != nil {
			// Get title/artist for music files
			var title, artist, album string
			var musicMetadata database.MusicMetadata

			if err := db.Where("media_file_id = ?", mediaID).First(&musicMetadata).Error; err == nil {
				title = musicMetadata.Title
				artist = musicMetadata.Artist
				album = musicMetadata.Album
			} else {
				title = filepath.Base(mediaFile.Path)
			}

			playEvent := events.NewSystemEvent(
				events.EventPlaybackStarted,
				"Playback Started",
				fmt.Sprintf("Started streaming: %s - %s", artist, title),
			)
			playEvent.Data = map[string]interface{}{
				"mediaId":   mediaID,
				"userId":    userID,
				"timestamp": time.Now().Unix(),
				"title":     title,
				"artist":    artist,
				"album":     album,
				"path":      mediaFile.Path,
			}
			h.eventBus.PublishAsync(playEvent)
		}
	}

	// Stream the file to the client
	c.DataFromReader(http.StatusOK, fileInfo.Size(), contentType, file, nil)
}

// UploadMedia functionality removed as app won't support uploads
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Upload functionality is not supported",
	})
}

// Keep original function-based handlers for backward compatibility
// These will delegate to the struct-based handlers

// GetMedia function-based handler for backward compatibility
func GetMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.GetMedia(c)
}

// UploadMedia function-based handler for backward compatibility
func UploadMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.UploadMedia(c)
}

// StreamMedia function-based handler for backward compatibility
func StreamMedia(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &MediaHandler{}
	handler.StreamMedia(c)
}

// GetArtwork serves album artwork for a media file - deprecated, use entity-based asset system instead
func GetArtwork(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":       "This endpoint is deprecated",
		"message":     "Artwork is now managed through the entity-based asset system",
		"suggestion":  "Use /api/v1/assets/entity/{type}/{id}/preferred/cover for album artwork",
		"documentation": "See /api/v1/assets/ endpoints for the new asset management system",
	})
}

// Helper function to check if a file is a music file
func isMusicFile(extension string) bool {
	ext := strings.ToLower(extension)
	musicExtensions := []string{".mp3", ".flac", ".aac", ".ogg", ".wav", ".m4a"}

	for _, e := range musicExtensions {
		if ext == e {
			return true
		}
	}

	return false
}

// LoadTestMusicData loads test music data for development - deprecated
func LoadTestMusicData(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Load test music data functionality is deprecated",
		"message": "Test data loading is no longer supported",
		"suggestion": "Use real media files and the scanner system instead",
	})
}
