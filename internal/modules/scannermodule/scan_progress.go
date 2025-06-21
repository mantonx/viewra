package scannermodule

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// getScanProgress handles GET /api/scanner/progress/:id requests
func (m *Module) getScanProgress(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	// Try to get detailed progress including worker stats
	detailedStats, err := m.scannerManager.GetDetailedScanProgress(uint32(jobID))
	if err == nil {
		// Return detailed stats for active jobs
		c.JSON(http.StatusOK, detailedStats)
		return
	}

	// If no detailed stats available, fall back to basic progress
	progress, eta, filesPerSec, err := m.scannerManager.GetScanProgress(uint32(jobID))
	if err != nil {
		// If no active scanner, try to get progress from database
		scanJob, dbErr := m.scannerManager.GetScanStatus(uint32(jobID))
		if dbErr != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Scan job not found",
			})
			return
		}

		// Return database progress for inactive jobs
		c.JSON(http.StatusOK, gin.H{
			"progress":        scanJob.Progress,
			"eta":             "",
			"files_per_sec":   0,
			"bytes_processed": scanJob.BytesProcessed,
			"files_processed": scanJob.FilesProcessed,
		})
		return
	}

	// Return basic real-time progress for active jobs
	c.JSON(http.StatusOK, gin.H{
		"progress":      progress,
		"eta":           eta,
		"files_per_sec": filesPerSec,
	})
}
