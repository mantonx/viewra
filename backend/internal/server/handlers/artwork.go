package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/metadata"
)

// GetArtwork serves album artwork for a media file
func GetArtwork(c *gin.Context) {
	mediaFileIDStr := c.Param("id")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	// Get artwork path
	artworkPath, err := metadata.GetArtworkPath(uint(mediaFileID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Artwork not found",
			"details": err.Error(),
		})
		return
	}

	// Determine content type based on file extension
	ext := filepath.Ext(artworkPath)
	var contentType string
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	default:
		contentType = "application/octet-stream"
	}

	// Set headers for caching
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	// Serve the file
	c.File(artworkPath)
}
