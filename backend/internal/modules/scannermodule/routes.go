package scannermodule

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the scanner module routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/scanner")
	{
		// Scanner status and configuration
		api.GET("/status", m.getGeneralStatus)
		api.GET("/config", m.getConfig)
		api.POST("/config", m.setConfig)
		
		// Scan job management
		api.POST("/scan", m.startGeneralScan)
		api.GET("/jobs", m.listScanJobs)
		api.POST("/cancel-all", m.cancelAllScans)
		
		// Individual scan job operations
		api.GET("/jobs/:id", m.getScanStatus)
		api.DELETE("/jobs/:id", m.cancelScan)
		api.POST("/resume/:id", m.resumeScan)
		
		// Real-time scan progress endpoint
		api.GET("/progress/:id", m.getScanProgress)
	}
}

// getScanStatus returns the status of a specific scan job
func (m *Module) getScanStatus(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	status, err := m.scannerManager.GetScanStatus(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Scan job not found",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// cancelScan cancels a specific scan job
func (m *Module) cancelScan(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	if err := m.scannerManager.StopScan(uint(jobID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Scan cancelled successfully",
	})
}
