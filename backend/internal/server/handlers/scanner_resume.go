package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// ResumeLibraryScan resumes a previously paused scan for a specific library
func ResumeLibraryScan(c *gin.Context) {
	libraryIDStr := c.Param("id")
	libraryID, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	if scannerManager == nil {
		InitializeScanner()
	}
	
	// Find paused scan job for this library
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	// Find the most recent paused scan job for this library
	var mostRecentJob database.ScanJob
	var jobID uint
	found := false
	
	for _, job := range scanJobs {
		if job.LibraryID == uint(libraryID) && job.Status == "paused" {
			if !found || job.UpdatedAt.After(mostRecentJob.UpdatedAt) {
				mostRecentJob = job
				jobID = job.ID
				found = true
			}
		}
	}
	
	if !found {
		// If there's no paused scan, start a new one
		StartLibraryScan(c)
		return
	}
	
	// Resume the paused scan
	scanJob, err := scannerManager.ResumeScan(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to resume scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":     "Scan resumed successfully",
		"library_id":  libraryID,
		"job_id":      jobID,
		"scan_job":    scanJob,
	})
}
