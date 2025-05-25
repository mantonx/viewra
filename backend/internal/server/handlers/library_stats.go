package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
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
	libraryStats := make(map[uint]map[string]interface{})
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
				// Add scan job details to the library stats
				if _, ok := libraryStats[job.LibraryID]; !ok {
					libraryStats[job.LibraryID] = make(map[string]interface{})
				}
				
				libraryStats[job.LibraryID]["scan_status"] = job.Status
				libraryStats[job.LibraryID]["progress"] = job.Progress
				libraryStats[job.LibraryID]["files_found"] = job.FilesFound
				libraryStats[job.LibraryID]["files_processed"] = job.FilesProcessed
				libraryStats[job.LibraryID]["bytes_processed"] = job.BytesProcessed
			}
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"library_stats": libraryStats,
	})
}
