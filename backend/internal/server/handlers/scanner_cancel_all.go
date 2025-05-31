package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CancelAllScans cancels all currently running scans
func CancelAllScans(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	jobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get active scans",
			"details": err.Error(),
		})
		return
	}

	cancelledCount := 0
	var errors []string

	for _, job := range jobs {
		if job.Status == "running" || job.Status == "paused" {
			if err := scannerManager.StopScan(job.ID); err != nil {
				errors = append(errors, err.Error())
			} else {
				cancelledCount++
			}
		}
	}

	response := gin.H{
		"message":         "Scan cancellation requested",
		"cancelled_count": cancelledCount,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}
