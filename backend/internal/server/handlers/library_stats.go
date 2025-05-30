package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
)

// GetAllLibraryStats returns statistics for all libraries
func GetAllLibraryStats(c *gin.Context) {
	if scannerManager == nil {
		InitializeScannerCompat()
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
	libraryStats := make(map[uint]map[string]interface{}) 
	for _, lib := range libraries {
		stats, err := scannerManager.GetLibraryStats(lib.ID)
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
				} else {
					// This case should ideally not happen if all libraries are pre-populated.
					// If it can, initialize a basic map here before adding job details.
					// For example:
					// libraryStats[job.LibraryID] = map[string]interface{}{
					// 	"scan_status":      job.Status,
					// 	"progress":         job.Progress,
					// 	"files_found":      job.FilesFound,
					// 	"files_processed":  job.FilesProcessed,
					// 	"bytes_processed":  job.BytesProcessed,
					// 	"error":            "Base library stats not found, showing job data only",
					// }
				}
			}
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"library_stats": libraryStats,
	})
}
