package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/utils"
)

// GetAllLibraryStats returns statistics for all libraries
func GetAllLibraryStats(c *gin.Context) {
	if scannerManager == nil {
		InitializeScanner()
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
	libraryStats := make(map[uint]*utils.LibraryStats)
	for _, lib := range libraries {
		stats, err := scannerManager.GetLibraryStats(lib.ID)
		if err != nil {
			continue // Skip libraries with errors
		}
		libraryStats[lib.ID] = stats
	}
	
	// Get active scans and their details
	scanJobs, err := scannerManager.GetAllScans()
	if err == nil {
		for _, job := range scanJobs {
			if job.Status == "running" || job.Status == "paused" {
				// Create stats entry if it doesn't exist
				if _, ok := libraryStats[job.LibraryID]; !ok {
					libraryStats[job.LibraryID] = &utils.LibraryStats{}
				}
				
				// Create enhanced response with scan job details
				enhancedStats := map[string]interface{}{
					"total_files":      libraryStats[job.LibraryID].TotalFiles,
					"total_size":       libraryStats[job.LibraryID].TotalSize,
					"extension_stats":  libraryStats[job.LibraryID].ExtensionStats,
					"scan_status":      job.Status,
					"progress":         job.Progress,
					"files_found":      job.FilesFound,
					"files_processed":  job.FilesProcessed,
					"bytes_processed":  job.BytesProcessed,
				}
				
				c.JSON(http.StatusOK, gin.H{
					"library_stats": enhancedStats,
				})
				return
			}
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"library_stats": libraryStats,
	})
}
