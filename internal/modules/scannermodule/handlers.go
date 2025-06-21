package scannermodule

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
)

// ScanConfig represents the configuration for a scanner
type ScanConfig struct {
	Paths       []string `json:"paths"`
	Recursive   bool     `json:"recursive"`
	ForceRescan bool     `json:"forceRescan"`
	Types       []string `json:"types"`
}

// getGeneralStatus returns the general status of the scanner
func (m *Module) getGeneralStatus(c *gin.Context) {
	if m.scannerManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Scanner manager not initialized",
			"status": "error",
		})
		return
	}

	// Get active scan count from jobs table
	var activeJobs []database.ScanJob
	err := m.db.Where("status = ?", "running").Find(&activeJobs).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"status": "error",
		})
		return
	}

	status := "idle"
	if len(activeJobs) > 0 {
		status = "scanning"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       status,
		"active_jobs":  len(activeJobs),
		"scanner_id":   m.ID(),
		"scanner_name": m.Name(),
	})
}

// startGeneralScan starts a general scan operation
func (m *Module) startGeneralScan(c *gin.Context) {
	// Make sure we have a scanner manager
	if m.scannerManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner manager not initialized",
		})
		return
	}

	// Get the database connection if needed
	if m.db == nil {
		m.db = database.GetDB()
	}

	// Get all libraries
	var libraries []database.MediaLibrary
	if err := m.db.Find(&libraries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve libraries: " + err.Error(),
		})
		return
	}

	// Check if libraries exist
	if len(libraries) == 0 {
		// Create a test library if none exists
		testLibrary := database.MediaLibrary{
			Path: "/app/data/test-music",
			Type: "music",
		}

		if err := m.db.Create(&testLibrary).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "No libraries found and could not create test library: " + err.Error(),
			})
			return
		}

		logger.Info("Created test library with ID: %d", testLibrary.ID)

		// Start the scan with the new library
		scanJob, err := m.scannerManager.StartScan(testLibrary.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"scan_job": scanJob,
			"message":  "General scan started successfully",
		})
		return
	}

	// Use the first available library for scanning
	libraryID := libraries[0].ID

	// Start the scan
	scanJob, err := m.scannerManager.StartScan(libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scan_job": scanJob,
		"message":  "General scan started successfully",
	})
}

// getConfig returns the current scanner configuration
func (m *Module) getConfig(c *gin.Context) {
	// For now, return a mock config
	config := ScanConfig{
		Paths:       []string{"/media", "/music"},
		Recursive:   true,
		ForceRescan: false,
		Types:       []string{"audio", "video"},
	}

	c.JSON(http.StatusOK, config)
}

// setConfig updates the scanner configuration
func (m *Module) setConfig(c *gin.Context) {
	var config ScanConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration: " + err.Error(),
		})
		return
	}

	// For now, just log the config, but in a real implementation
	// we'd save it to the database or configuration store
	logger.Info("Scanner config updated: %v", config)

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
		"config":  config,
	})
}

// GetActiveScans returns a list of active scans
func (m *Module) GetActiveScans() ([]database.ScanJob, error) {
	if m.db == nil {
		m.db = database.GetDB()
	}

	var activeJobs []database.ScanJob
	err := m.db.Where("status = ?", "running").Find(&activeJobs).Error
	return activeJobs, err
}

// listScanJobs returns all scan jobs
func (m *Module) listScanJobs(c *gin.Context) {
	// Make sure we have a scanner manager
	if m.scannerManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner manager not initialized",
		})
		return
	}

	jobs, err := m.scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs": jobs,
	})
}

// cancelAllScans cancels all active scan jobs
func (m *Module) cancelAllScans(c *gin.Context) {
	// Make sure we have a scanner manager
	if m.scannerManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner manager not initialized",
		})
		return
	}

	count, err := m.scannerManager.CancelAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "All scans cancelled successfully",
		"cancelled_count": count,
	})
}
