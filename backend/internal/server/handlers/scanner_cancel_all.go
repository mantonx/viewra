package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CancelAllScans stops all running scan jobs and marks them as paused
func CancelAllScans(c *gin.Context) {
	if scannerManager == nil {
		InitializeScanner()
	}
	
	count, err := scannerManager.CancelAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cancel running scans",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "All scans paused successfully",
		"count":   count,
	})
}
