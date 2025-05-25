// Media handler with event support
package handlers

import (
	"fmt"
	"io"
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
	"github.com/mantonx/viewra/internal/metadata"
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
	var media []database.Media
	db := database.GetDB()
	
	result := db.Preload("User").Find(&media)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve media",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media": media,
		"count": len(media),
	})
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
		userID := uint(0)  // Default to 0 for anonymous
		
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

// UploadMedia handles media file uploads
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	// Get the library ID from query parameter
	libraryIDStr := c.Query("libraryId")
	libraryID, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	// Check if the library exists
	db := database.GetDB()
	var library database.MediaLibrary
	if err := db.First(&library, libraryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Library not found",
		})
		return
	}
	
	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded",
		})
		return
	}
	defer file.Close()
	
	// Create output directory if it doesn't exist
	uploadPath := filepath.Join(library.Path, "uploads")
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create upload directory",
		})
		return
	}
	
	// Generate a unique filename
	filename := header.Filename
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Format("20060102_150405")
	uniqueFilename := fmt.Sprintf("%s_%s%s", base, timestamp, ext)
	filePath := filepath.Join(uploadPath, uniqueFilename)
	
	// Create the output file
	output, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create output file",
		})
		return
	}
	defer output.Close()
	
	// Copy the file
	_, err = io.Copy(output, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save uploaded file",
		})
		return
	}
	
	// Get the file size
	fileInfo, err := output.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get file info",
		})
		return
	}
	fileSize := fileInfo.Size()
	
	// Calculate file hash
	fileHash, err := utils.CalculateFileHash(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to calculate file hash",
		})
		return
	}
	
	// Create media file record
	mediaFile := database.MediaFile{
		Path:      filePath,
		Size:      fileSize,
		Hash:      fileHash,
		LibraryID: uint(libraryID),
		LastSeen:  time.Now(),
	}
	
	if err := db.Create(&mediaFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create media file record",
		})
		return
	}
	
	// Process metadata if it's a music file
	var metadataID uint
	if isMusicFile(ext) {
		mmd, err := processMusicMetadata(filePath, mediaFile.ID)
		if err == nil {
			metadataID = mmd.ID
		}
	}
	
	// Publish media upload event
	if h.eventBus != nil {
		uploadEvent := events.NewSystemEvent(
			events.EventMediaFileUploaded, 
			"Media File Uploaded",
			fmt.Sprintf("File uploaded: %s (%.2f MB)", uniqueFilename, float64(fileSize)/(1024*1024)),
		)
		uploadEvent.Data = map[string]interface{}{
			"mediaFileId": mediaFile.ID,
			"libraryId":   libraryID,
			"path":        filePath,
			"filename":    uniqueFilename,
			"size":        fileSize,
			"hash":        fileHash,
			"type":        ext,
			"metadataId":  metadataID,
		}
		h.eventBus.PublishAsync(uploadEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":     "File uploaded successfully",
		"mediaFileId": mediaFile.ID,
		"path":        filePath,
		"size":        fileSize,
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

// Helper function to process music metadata
func processMusicMetadata(filePath string, mediaFileID uint) (*database.MusicMetadata, error) {
	// Get the media file for metadata extraction
	db := database.GetDB()
	var mediaFile database.MediaFile
	if err := db.First(&mediaFile, mediaFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to find media file: %w", err)
	}
	
	// Extract metadata using the metadata package
	md, err := metadata.ExtractMusicMetadata(filePath, &mediaFile)
	if err != nil {
		return nil, err
	}
	
	// Create music metadata record
	musicMetadata := database.MusicMetadata{
		MediaFileID: mediaFileID,
		Title:       md.Title,
		Artist:      md.Artist,
		Album:       md.Album,
		Genre:       md.Genre,
		Year:        md.Year,
		Track:       md.Track,
		Duration:    md.Duration,
	}
	
	// Save to database
	if err := db.Create(&musicMetadata).Error; err != nil {
		return nil, err
	}
	
	return &musicMetadata, nil
}
