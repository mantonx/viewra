package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
	
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	// Use the new library-based resume method for better consistency
	err = scannerManager.ResumeScanByLibrary(uint32(libraryID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to resume scan",
			"details": err.Error(),
		})
		return
	}
	
	// Get the updated scan job status
	scanJob, statusErr := scannerManager.GetLibraryScanStatus(uint32(libraryID))
	if statusErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan status after resume",
			"details": statusErr.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":     "Scan resumed successfully",
		"library_id":  libraryID,
		"job_id":      scanJob.ID,
		"scan_job":    scanJob,
	})
}
