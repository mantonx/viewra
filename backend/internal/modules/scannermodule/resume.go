package scannermodule

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// resumeScan resumes a paused scan job
func (m *Module) resumeScan(c *gin.Context) {
	// Extract the job ID from the URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.Error("Invalid ID format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid ID format",
		})
		return
	}

	// Use the ID directly as a job ID
	jobID := uint32(id)
	
	// Check if the job exists and is in a paused or pending state
	scanJob, jobErr := m.scannerManager.GetScanStatus(jobID)
	if jobErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Scan job not found: " + jobErr.Error(),
		})
		return
	}
	
	// Make sure the job is in a state that can be resumed (allow failed jobs from system restarts)
	if scanJob.Status != "paused" && scanJob.Status != "pending" && scanJob.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot resume job with status: " + scanJob.Status,
		})
		return
	}
	
	// Get the library ID from the scan job
	libraryID := scanJob.LibraryID
	
	// Resume the scan job
	logger.Info("Resuming scan job %d for library %d", jobID, libraryID)
	
	err = m.scannerManager.ResumeScan(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to resume scan: " + err.Error(),
		})
		return
	}
	
	// Get updated job status
	updatedScanJob, err := m.scannerManager.GetScanStatus(jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get scan status after resume: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":    "Scan resumed successfully",
		"library_id": libraryID,
		"job_id":     jobID,
		"scan_job":   updatedScanJob,
	})
}
