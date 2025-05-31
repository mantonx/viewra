package mediamodule

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
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
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.Preload("MusicMetadata").Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
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
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	// Then delete the file record
	if err := m.db.Delete(&mediaFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete media file: %v", err),
		})
		return
	}

	// TODO: With new schema, metadata deletion would be through Artist/Album/Track relationships
	// For now, just return success since MediaFile deletion is already handled
	// if err := m.db.Where("media_file_id = ?", idStr).Delete(&database.MusicMetadata{}).Error; err != nil {
	//	 return c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete music metadata"})
	// }

	c.JSON(http.StatusOK, gin.H{
		"message": "Media file deleted successfully",
		"id":      idStr,
	})
}

// streamFile streams a media file
func (m *Module) streamFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
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
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Get music metadata
	var musicMetadata database.MusicMetadata
	if err := m.db.Where("media_file_id = ?", idStr).First(&musicMetadata).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Metadata not found for this file",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"metadata": musicMetadata,
	})
}

// getFileAlbumId returns the album UUID for a media file for the new asset system
func (m *Module) getFileAlbumId(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Generate the deterministic album UUID using the file ID
	albumIDString := fmt.Sprintf("album-placeholder-%s", idStr)
	albumID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(albumIDString))
	
	c.JSON(http.StatusOK, gin.H{
		"media_file_id": idStr,
		"album_id": albumID.String(),
		"asset_url": fmt.Sprintf("/api/v1/assets/entity/album/%s/preferred/cover", albumID.String()),
	})
}

// getFileAlbumArtwork serves album artwork for a media file using the new asset system
func (m *Module) getFileAlbumArtwork(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Generate the deterministic album UUID using the file ID
	albumIDString := fmt.Sprintf("album-placeholder-%s", idStr)
	albumID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(albumIDString))
	
	// Get quality parameter
	qualityStr := c.Query("quality")
	quality := 0 // Default to original quality
	if qualityStr != "" {
		if q, err := strconv.Atoi(qualityStr); err == nil && q > 0 && q <= 100 {
			quality = q
		}
	}
	
	// Try to get assets directly using a simple database query
	var asset struct {
		ID     uuid.UUID `json:"id"`
		Path   string    `json:"path"`
		Format string    `json:"format"`
	}
	
	// Query the database directly for the preferred cover asset
	err := m.db.Table("media_assets").
		Select("id, path, format").
		Where("entity_type = ? AND entity_id = ? AND type = ? AND preferred = ?", 
			"album", albumID.String(), "cover", true).
		First(&asset).Error
	
	if err != nil {
		// Try any cover asset for this album
		err = m.db.Table("media_assets").
			Select("id, path, format").
			Where("entity_type = ? AND entity_id = ? AND type = ?", 
				"album", albumID.String(), "cover").
			First(&asset).Error
	}
	
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No album artwork found",
			"album_id": albumID.String(),
		})
		return
	}
	
	// Serve the file directly
	fullPath := filepath.Join("/app/viewra-data/assets", asset.Path)
	
	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Artwork file not found on disk",
			"path": fullPath,
		})
		return
	}
	
	// Set appropriate headers
	c.Header("Content-Type", asset.Format)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
	
	if quality > 0 {
		c.Header("X-Quality", qualityStr)
	}
	
	// Serve the file
	c.File(fullPath)
}

// Upload functionality has been removed as the app will not support media uploads

// Upload to library functionality has been removed as the app will not support media uploads

// extractMetadata extracts metadata from a media file
func (m *Module) extractMetadata(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	// Get the media file from database
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", idStr).First(&mediaFile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}
	
	if err := m.metadataManager.ExtractMetadata(&mediaFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to extract metadata: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Metadata extracted successfully",
		"id":      idStr,
	})
}

// updateMetadata updates metadata for a media file
func (m *Module) updateMetadata(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
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
	musicMetadata.MediaFileID = idStr
	
	// Check if metadata exists
	var existingMetadata database.MusicMetadata
	if err := m.db.Where("media_file_id = ?", idStr).First(&existingMetadata).Error; err != nil {
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
		"id":      idStr,
	})
}

// processFile processes a media file
func (m *Module) processFile(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file ID",
		})
		return
	}
	
	jobID, err := m.fileProcessor.ProcessFile(idStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to process file: %v", err),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "File processing started",
		"job_id":  jobID,
		"id":      idStr,
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

