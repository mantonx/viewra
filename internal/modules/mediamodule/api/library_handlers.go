// Package api - Library management handlers
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// GetLibraries handles GET /api/libraries
// It returns all media libraries configured in the system.
//
// Response: Array of Library objects with scan statistics
func (h *Handler) GetLibraries(c *gin.Context) {
	var libraries []database.MediaLibrary
	db := c.MustGet("db").(*gorm.DB)
	
	if err := db.Find(&libraries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Enrich with scan statistics
	librariesWithStats := h.enrichLibrariesWithStats(db, libraries)
	
	c.JSON(http.StatusOK, librariesWithStats)
}

// GetLibrary handles GET /api/libraries/:id
// It returns detailed information about a specific library.
//
// Path parameters:
//   - id: The library ID
//
// Response: Library object with statistics or 404 if not found
func (h *Handler) GetLibrary(c *gin.Context) {
	id := c.Param("id")
	
	var lib database.MediaLibrary
	db := c.MustGet("db").(*gorm.DB)
	
	if err := db.Where("id = ?", id).First(&lib).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Get statistics
	stats := h.getLibraryStats(db, uint(lib.ID))
	
	c.JSON(http.StatusOK, gin.H{
		"id":         lib.ID,
		"path":       lib.Path,
		"type":       lib.Type,
		"stats":      stats,
		"created_at": lib.CreatedAt,
		"updated_at": lib.UpdatedAt,
	})
}

// CreateLibrary handles POST /api/libraries
// It creates a new media library.
//
// Request body:
//   {
//     "name": "Movies",
//     "path": "/media/movies",
//     "type": "movie",
//     "scan_enabled": true,
//     "scan_interval": 3600
//   }
//
// Response: Created Library object
func (h *Handler) CreateLibrary(c *gin.Context) {
	var input database.MediaLibraryRequest
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Create library through manager
	lib := &database.MediaLibrary{
		Path: input.Path,
		Type: input.Type,
	}
	
	if err := h.libraryManager.CreateLibrary(c.Request.Context(), lib); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, lib)
}

// UpdateLibrary handles PUT /api/libraries/:id
// It updates an existing library configuration.
//
// Path parameters:
//   - id: The library ID
//
// Request body:
//   {
//     "path": "/new/path",
//     "type": "movie"
//   }
//
// Response: Success message
func (h *Handler) UpdateLibrary(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Update through manager
	err = h.libraryManager.UpdateLibrary(c.Request.Context(), uint32(id), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Library updated successfully"})
}

// DeleteLibrary handles DELETE /api/libraries/:id
// It deletes a library and optionally its associated media files.
//
// Path parameters:
//   - id: The library ID
//
// Query parameters:
//   - delete_files: Whether to delete media file records (default: false)
//
// Response: Success message
func (h *Handler) DeleteLibrary(c *gin.Context) {
	id := c.Param("id")
	// deleteFiles := c.Query("delete_files") == "true" // TODO: Implement file deletion option
	
	// Parse ID
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	// Delete through manager  
	err = h.libraryManager.DeleteLibrary(c.Request.Context(), uint32(idUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Library deleted successfully"})
}

// ScanLibrary handles POST /api/libraries/:id/scan
// It triggers a manual scan of a library.
//
// Path parameters:
//   - id: The library ID
//
// Query parameters:
//   - full: Whether to perform a full rescan (default: false)
//
// Response: Scan job information
func (h *Handler) ScanLibrary(c *gin.Context) {
	id := c.Param("id")
	fullScan := c.Query("full") == "true"
	
	// Parse ID
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	// TODO: Implement scan functionality
	// For now, return a placeholder response
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Library scan started",
		"job_id":  "scan-" + id,
		"full_scan": fullScan,
		"library_id": uint32(idUint),
	})
}

// GetLibraryScanStatus handles GET /api/libraries/:id/scan/status
// It returns the current scan status for a library.
//
// Path parameters:
//   - id: The library ID
//
// Response: Scan status information
func (h *Handler) GetLibraryScanStatus(c *gin.Context) {
	id := c.Param("id")
	
	// Parse ID
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	// TODO: Implement scan status tracking
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"library_id": uint32(idUint),
		"status": "idle",
		"last_scan": nil,
		"files_scanned": 0,
		"files_total": 0,
	})
}

// RefreshMetadata handles POST /api/libraries/:id/metadata/refresh
// It triggers metadata refresh for all items in a library.
//
// Path parameters:
//   - id: The library ID
//
// Query parameters:
//   - force: Force refresh even if metadata exists (default: false)
//
// Response: Refresh job information
func (h *Handler) RefreshMetadata(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"
	
	// Parse ID
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}
	
	// TODO: Implement metadata refresh functionality
	// For now, return a placeholder response
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Metadata refresh started",
		"job_id":  "refresh-" + id,
		"force": force,
		"library_id": uint32(idUint),
	})
}

// GetLibraryStats handles GET /api/libraries/:id/stats
// It returns detailed statistics for a library.
//
// Path parameters:
//   - id: The library ID
//
// Response: Library statistics
func (h *Handler) GetLibraryStats(c *gin.Context) {
	id := c.Param("id")
	
	var lib database.MediaLibrary
	db := c.MustGet("db").(*gorm.DB)
	
	// Check library exists
	if err := db.Where("id = ?", id).First(&lib).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	stats := h.getDetailedLibraryStats(db, uint(lib.ID))
	c.JSON(http.StatusOK, stats)
}

// Helper functions

// enrichLibrariesWithStats adds statistics to libraries
func (h *Handler) enrichLibrariesWithStats(db *gorm.DB, libraries []database.MediaLibrary) []gin.H {
	result := make([]gin.H, len(libraries))
	
	for i, lib := range libraries {
		stats := h.getLibraryStats(db, uint(lib.ID))
		
		result[i] = gin.H{
			"id":         lib.ID,
			"path":       lib.Path,
			"type":       lib.Type,
			"stats":      stats,
			"created_at": lib.CreatedAt,
			"updated_at": lib.UpdatedAt,
		}
	}
	
	return result
}

// getLibraryStats returns basic statistics for a library
func (h *Handler) getLibraryStats(db *gorm.DB, libraryID uint) gin.H {
	var fileCount int64
	var totalSize int64
	
	// Get file count and total size
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Count(&fileCount)
	
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("COALESCE(SUM(size_bytes), 0)").
		Scan(&totalSize)
	
	// Get counts by media type
	var typeCounts []struct {
		MediaType string
		Count     int64
	}
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("media_type, COUNT(*) as count").
		Group("media_type").
		Scan(&typeCounts)
	
	// Convert to map
	typeCountMap := make(map[string]int64)
	for _, tc := range typeCounts {
		typeCountMap[tc.MediaType] = tc.Count
	}
	
	return gin.H{
		"file_count":    fileCount,
		"total_size":    totalSize,
		"type_counts":   typeCountMap,
	}
}

// getDetailedLibraryStats returns detailed statistics for a library
func (h *Handler) getDetailedLibraryStats(db *gorm.DB, libraryID uint) gin.H {
	stats := h.getLibraryStats(db, libraryID)
	
	// Add codec statistics
	var codecStats []struct {
		VideoCodec string
		AudioCodec string
		Count      int64
	}
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("video_codec, audio_codec, COUNT(*) as count").
		Group("video_codec, audio_codec").
		Scan(&codecStats)
	
	// Add resolution statistics
	var resolutionStats []struct {
		Resolution string
		Count      int64
	}
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("resolution, COUNT(*) as count").
		Group("resolution").
		Scan(&resolutionStats)
	
	// Add container statistics
	var containerStats []struct {
		Container string
		Count     int64
	}
	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("container, COUNT(*) as count").
		Group("container").
		Scan(&containerStats)
	
	// Add recent additions
	var recentFiles []database.MediaFile
	db.Where("library_id = ?", libraryID).
		Order("created_at DESC").
		Limit(10).
		Find(&recentFiles)
	
	stats["codec_stats"] = codecStats
	stats["resolution_stats"] = resolutionStats
	stats["container_stats"] = containerStats
	stats["recent_additions"] = recentFiles
	
	return stats
}