package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
)

// GetAllLibraryStats returns statistics for all libraries
func GetAllLibraryStats(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	// Get all libraries
	var libraries []database.MediaLibrary
	db := database.GetDB()
	if err := db.Find(&libraries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get libraries",
			"details": err.Error(),
		})
		return
	}
	
	// Get stats for each library
	libraryStats := make(map[uint32]map[string]interface{}) 
	for _, lib := range libraries {
		stats, err := scannerManager.GetLibraryStats(uint32(lib.ID))
		if err != nil {
			// For libraries where GetLibraryStats fails, get basic counts directly from MediaFile table
			var totalFiles int64
			var totalSize int64
			
			if err := db.Model(&database.MediaFile{}).Where("library_id = ?", lib.ID).Count(&totalFiles).Error; err == nil {
				// Successfully got file count
				db.Model(&database.MediaFile{}).Where("library_id = ?", lib.ID).Select("COALESCE(SUM(size), 0)").Scan(&totalSize)
				
				libraryStats[lib.ID] = map[string]interface{}{
					"total_files":      totalFiles,
					"total_size":       totalSize,
					"extension_stats":  map[string]interface{}{},
				}
			} else {
				// Complete failure - provide minimal structure
				libraryStats[lib.ID] = map[string]interface{}{
					"total_files":      0,
					"total_size":       0,
					"extension_stats":  map[string]interface{}{},
					"stats_error":      "Failed to retrieve library stats",
				}
			}
			continue 
		}
		libraryStats[lib.ID] = map[string]interface{}{
			"total_files":      stats.TotalFiles,
			"total_size":       stats.TotalSize,
			"extension_stats":  stats.ExtensionStats,
		}
	}
	
	// Enhance with active scans and their details
	scanJobs, err := scannerManager.GetAllScans()
	if err == nil {
		for _, job := range scanJobs {
			if job.Status == "running" || job.Status == "paused" {
				if entry, ok := libraryStats[job.LibraryID]; ok {
					entry["scan_status"] = job.Status
					entry["progress"] = job.Progress
					entry["files_found"] = job.FilesFound
					entry["files_processed"] = job.FilesProcessed
					entry["bytes_processed"] = job.BytesProcessed
					
					// Don't override accurate MediaFile counts with scan job data
					// The MediaFile count is more reliable than scan job's files_found
				}
			}
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"library_stats": libraryStats,
	})
}

// GetLibraryMetrics returns detailed metrics for a specific library
func GetLibraryMetrics(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	libraryIDStr := c.Param("id")
	libraryID, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}

	stats, err := scannerManager.GetLibraryStats(uint32(libraryID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get library metrics",
			"details": err.Error(),
		})
		return
	}

	// Get the media library details
	db := database.GetDB()
	var library database.MediaLibrary
	if err := db.First(&library, libraryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Library not found",
		})
		return
	}

	// Get additional database metrics
	var totalSize int64
	var totalFiles int64
	var avgFileSize float64

	db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("COALESCE(COUNT(*), 0) as total_files, COALESCE(SUM(size), 0) as total_size").
		Row().Scan(&totalFiles, &totalSize)

	if totalFiles > 0 {
		avgFileSize = float64(totalSize) / float64(totalFiles)
	}

	// Get scan history for this library
	scanHistory, err := scannerManager.GetAllScans()
	var libraryScans []interface{}
	if err == nil {
		// Filter to only this library
		for _, scan := range scanHistory {
			if scan.LibraryID == uint32(libraryID) {
				libraryScans = append(libraryScans, scan)
			}
		}
	}

	// Combine all metrics
	response := gin.H{
		"library":          library,
		"scanner_stats":    stats,
		"scan_history":     libraryScans,
		"total_files":      totalFiles,
		"total_size_bytes": totalSize,
		"avg_file_size":    avgFileSize,
	}

	c.JSON(http.StatusOK, response)
}
