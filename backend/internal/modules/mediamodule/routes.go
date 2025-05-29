package mediamodule

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
)

// getLibraries returns all media libraries
func (m *Module) getLibraries(c *gin.Context) {
	libraries, err := m.libraryManager.GetAllLibraries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get libraries: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"libraries": libraries,
		"count":     len(libraries),
	})
}

// createLibrary creates a new media library
func (m *Module) createLibrary(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Type string `json:"type" binding:"required"`
		Name string `json:"name"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}
	
	// Use provided name or path as name if not provided
	name := req.Name
	if name == "" {
		name = req.Path
	}
	
	library, err := m.libraryManager.CreateLibrary(name, req.Path, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create library: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "Library created successfully",
		"library": library,
	})
}

// deleteLibrary deletes a media library
func (m *Module) deleteLibrary(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	if err := m.libraryManager.DeleteLibrary(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete library: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Library deleted successfully",
		"id":      id,
	})
}

// getLibraryStats returns statistics for a library
func (m *Module) getLibraryStats(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	stats, err := m.libraryManager.GetLibraryStats(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get library stats: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// getLibraryFiles returns all files in a library
func (m *Module) getLibraryFiles(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	var mediaFiles []database.MediaFile
	var total int64
	
	// Get total count
	m.db.Model(&database.MediaFile{}).Where("library_id = ?", id).Count(&total)
	
	// Get paginated results
	result := m.db.Where("library_id = ?", id).
		Limit(limit).
		Offset(offset).
		Order("path").
		Find(&mediaFiles)
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get media files: %v", result.Error),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}

// getFiles returns all media files
func (m *Module) getFiles(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	var mediaFiles []database.MediaFile
	var total int64
	
	// Get total count
	m.db.Model(&database.MediaFile{}).Count(&total)
	
	// Get paginated results
	result := m.db.Limit(limit).
		Offset(offset).
		Order("id DESC").
		Find(&mediaFiles)
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get media files: %v", result.Error),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}

// getFile returns a specific media file
func (m *Module) getFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.Preload("MusicMetadata").First(&mediaFile, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media_file": mediaFile,
	})
}

// deleteFile deletes a media file
func (m *Module) deleteFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.First(&mediaFile, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	// First delete related metadata
	if err := m.db.Where("media_file_id = ?", id).Delete(&database.MusicMetadata{}).Error; err != nil {
		log.Printf("WARNING: Failed to delete music metadata for file %d: %v", id, err)
	}
	
	// Then delete the file record
	if err := m.db.Delete(&mediaFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete media file: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Media file deleted successfully",
		"id":      id,
	})
}

// streamFile streams a media file
func (m *Module) streamFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.First(&mediaFile, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	// Check if file exists
	if _, err := os.Stat(mediaFile.Path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found on disk",
		})
		return
	}
	
	// Determine content type
	contentType := getContentTypeFromPath(mediaFile.Path)
	
	// Set appropriate headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", filepath.Base(mediaFile.Path)))
	c.Header("Cache-Control", "no-cache")
	c.Header("Accept-Ranges", "bytes")
	
	// Serve the file
	c.File(mediaFile.Path)
}

// getFileMetadata returns metadata for a media file
func (m *Module) getFileMetadata(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Get music metadata
	var musicMetadata database.MusicMetadata
	if err := m.db.Where("media_file_id = ?", id).First(&musicMetadata).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Metadata not found for this file",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"metadata": musicMetadata,
	})
}

// Upload functionality has been removed as the app will not support media uploads

// Upload to library functionality has been removed as the app will not support media uploads

// extractMetadata extracts metadata from a media file
func (m *Module) extractMetadata(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	if err := m.metadataManager.ExtractMetadata(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to extract metadata: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Metadata extracted successfully",
		"id":      id,
	})
}

// updateMetadata updates metadata for a media file
func (m *Module) updateMetadata(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var musicMetadata database.MusicMetadata
	if err := c.ShouldBindJSON(&musicMetadata); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}
	
	// Ensure we update the correct record
	musicMetadata.MediaFileID = uint(id)
	
	// Check if metadata exists
	var existingMetadata database.MusicMetadata
	if err := m.db.Where("media_file_id = ?", id).First(&existingMetadata).Error; err != nil {
		// Create new metadata
		if err := m.db.Create(&musicMetadata).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create metadata: %v", err),
			})
			return
		}
	} else {
		// Update existing metadata
		musicMetadata.ID = existingMetadata.ID
		if err := m.db.Save(&musicMetadata).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to update metadata: %v", err),
			})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Metadata updated successfully",
		"id":      id,
	})
}

// processFile processes a media file
func (m *Module) processFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	jobID, err := m.fileProcessor.ProcessFile(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to process file: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "File processing started",
		"job_id":  jobID,
		"id":      id,
	})
}

// getProcessingStatus returns the status of processing jobs
func (m *Module) getProcessingStatus(c *gin.Context) {
	stats := m.fileProcessor.GetStats()
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// getHealth returns the health status of the media module
func (m *Module) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": m.version,
	})
}

// getStatus returns the status of the media module
func (m *Module) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":         m.id,
		"name":       m.name,
		"version":    m.version,
		"core":       m.core,
		"initialized": m.initialized,
	})
}

// getStats returns statistics about the media module
func (m *Module) getStats(c *gin.Context) {
	// Collect stats from components
	processorStats := m.fileProcessor.GetStats()
	metadataStats := m.metadataManager.GetStats()
	
	c.JSON(http.StatusOK, gin.H{
		"processor_stats": processorStats,
		"metadata_stats":  metadataStats,
	})
}

// getArtwork serves artwork for a media file with quality parameter support
func (m *Module) getArtwork(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Parse quality parameter (default to 100% for backend, but frontend should request 90%)
	qualityStr := c.DefaultQuery("quality", "100")
	quality, err := strconv.Atoi(qualityStr)
	if err != nil || quality < 1 || quality > 100 {
		quality = 100 // Default to 100% quality
	}
	
	// Check if media file exists
	var mediaFile database.MediaFile
	if err := m.db.First(&mediaFile, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	// Import the mediaasset module to use the asset manager
	assetManager := mediaassetmodule.GetAssetManager()
	if assetManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Asset manager not available",
		})
		return
	}
	
	// Get artwork assets for this media file
	assets, err := assetManager.GetAssetsByMediaFile(uint(id), mediaassetmodule.AssetTypeMusic)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No artwork found for this media file",
		})
		return
	}
	
	// Filter for artwork assets
	var artworkAsset *mediaassetmodule.AssetResponse
	for _, asset := range assets {
		if asset.Subtype == mediaassetmodule.SubtypeArtwork {
			artworkAsset = asset
			break
		}
	}
	
	if artworkAsset == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No artwork found for this media file",
		})
		return
	}
	
	// Get asset data with quality adjustment
	data, mimeType, err := assetManager.GetAssetDataWithQuality(artworkAsset.ID, quality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve artwork data",
			"details": err.Error(),
		})
		return
	}
	
	// Set appropriate headers
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	
	// Add quality info to headers for debugging
	c.Header("X-Image-Quality", strconv.Itoa(quality))
	c.Header("X-Original-MimeType", artworkAsset.MimeType)
	
	// Stream the data
	c.Data(http.StatusOK, mimeType, data)
}

// Helper function to get content type based on file extension
func getContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
