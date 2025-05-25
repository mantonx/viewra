package handlers

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// GetMedia retrieves all media items with associated user information
func GetMedia(c *gin.Context) {
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
func StreamMedia(c *gin.Context) {
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

	// Check if file exists on disk
	filePath := mediaFile.Path
	file, err := os.Open(filePath)
	if err != nil {
		// Try alternative paths if the original path doesn't work
		pathVariants := []string{filePath}
		
		// Add common path mappings
		if strings.HasPrefix(filePath, "/app/") {
			pathVariants = append(pathVariants, strings.TrimPrefix(filePath, "/app"))
			pathVariants = append(pathVariants, filepath.Join(".", strings.TrimPrefix(filePath, "/app")))
		}
		
		// Try workspace-relative paths
		pwd, err := os.Getwd()
		if err == nil {
			pathVariants = append(pathVariants, filepath.Join(pwd, filePath))
			
			// Special handling for test data paths
			if strings.Contains(filePath, "data/test-music") {
				parts := strings.Split(filePath, "data/test-music")
				if len(parts) > 1 {
					relPath := "data/test-music" + parts[len(parts)-1]
					pathVariants = append(pathVariants, filepath.Join(pwd, relPath))
				}
			}
		}
		
		// Try each path variant
		var validPath string
		for _, path := range pathVariants {
			if f, err := os.Open(path); err == nil {
				file = f
				validPath = path
				break
			}
		}
		
		if validPath == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Media file not found on disk",
				"path":  filePath,
			})
			return
		}
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

	// Detect content type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// Default content types for common audio formats
		switch ext {
		case ".mp3":
			contentType = "audio/mpeg"
		case ".wav":
			contentType = "audio/wav"
		case ".flac":
			contentType = "audio/flac"
		case ".ogg":
			contentType = "audio/ogg"
		case ".m4a":
			contentType = "audio/mp4"
		case ".aac":
			contentType = "audio/aac"
		default:
			contentType = "application/octet-stream"
		}
	}

	// Set headers for streaming
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")
	
	// Handle range requests for seeking
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		// Parse range header (simplified implementation)
		if strings.HasPrefix(rangeHeader, "bytes=") {
			ranges := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(ranges, "-")
			if len(parts) == 2 {
				start, err1 := strconv.ParseInt(parts[0], 10, 64)
				var end int64
				if parts[1] != "" {
					end, _ = strconv.ParseInt(parts[1], 10, 64)
				} else {
					end = fileInfo.Size() - 1
				}
				
				if err1 == nil && start >= 0 && start <= end && end < fileInfo.Size() {
					// Seek to start position
					file.Seek(start, io.SeekStart)
					
					// Set partial content headers
					c.Header("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(fileInfo.Size(), 10))
					c.Header("Content-Length", strconv.FormatInt(end-start+1, 10))
					c.Status(http.StatusPartialContent)
					
					// Copy only the requested range
					io.CopyN(c.Writer, file, end-start+1)
					return
				}
			}
		}
	}

	// Stream the entire file
	c.Status(http.StatusOK)
	io.Copy(c.Writer, file)
}

// UploadMedia handles media file uploads
// TODO: Implement actual file upload functionality
func UploadMedia(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "File upload functionality coming soon",
		"status":  "not_implemented",
	})
}
