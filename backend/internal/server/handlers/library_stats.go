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
			// For libraries where we can't get base stats, provide minimal structure
			libraryStats[lib.ID] = map[string]interface{}{
				"total_files":      0,
				"total_size":       0,
				"extension_stats":  map[string]interface{}{},
				"stats_error":      "Failed to retrieve base stats", // Keep error info but don't use "error" key
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
					
					// If we don't have base stats, use scan job data as fallback
					if entry["total_files"] == 0 && job.FilesFound > 0 {
						entry["total_files"] = job.FilesFound  // Use files found from scan as best estimate
					}
					if entry["total_size"] == 0 && job.BytesProcessed > 0 {
						entry["total_size"] = job.BytesProcessed  // Use bytes processed so far
					}
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
