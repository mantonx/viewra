package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// StartSafeguardedLibraryScan starts a scan using the enhanced safeguards system
func StartSafeguardedLibraryScan(c *gin.Context) {
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
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Use safeguarded scan start
	result, err := scannerManager.StartSafeguardedScan(uint32(libraryID))
	if err != nil {
		logger.Error("Safeguarded scan start failed", "library_id", libraryID, "error", err)

		// Check if operation was rolled back
		if result != nil && result.WasRolledBack {
			c.JSON(http.StatusConflict, gin.H{
				"error":       "Failed to start scan",
				"details":     err.Error(),
				"rolled_back": true,
				"duration":    result.Duration.Milliseconds(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to start scan",
				"details": err.Error(),
			})
		}
		return
	}

	logger.Info("Safeguarded scan started successfully",
		"library_id", libraryID,
		"job_id", result.JobID,
		"duration", result.Duration.Milliseconds())

	c.JSON(http.StatusOK, gin.H{
		"message":     result.Message,
		"job_id":      result.JobID,
		"library_id":  libraryID,
		"operation":   string(result.Operation),
		"duration":    result.Duration.Milliseconds(),
		"safeguarded": true,
	})
}

// PauseSafeguardedLibraryScan pauses a scan using the enhanced safeguards system
func PauseSafeguardedLibraryScan(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Use safeguarded scan pause
	result, err := scannerManager.PauseSafeguardedScan(uint32(jobID))
	if err != nil {
		logger.Error("Safeguarded scan pause failed", "job_id", jobID, "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Failed to pause scan",
			"details":  err.Error(),
			"job_id":   jobID,
			"duration": result.Duration.Milliseconds(),
		})
		return
	}

	logger.Info("Safeguarded scan paused successfully",
		"job_id", jobID,
		"duration", result.Duration.Milliseconds())

	c.JSON(http.StatusOK, gin.H{
		"message":     result.Message,
		"job_id":      result.JobID,
		"operation":   string(result.Operation),
		"duration":    result.Duration.Milliseconds(),
		"safeguarded": true,
	})
}

// ResumeSafeguardedLibraryScan resumes a scan using the enhanced safeguards system
func ResumeSafeguardedLibraryScan(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Use safeguarded scan resume
	result, err := scannerManager.ResumeSafeguardedScan(uint32(jobID))
	if err != nil {
		logger.Error("Safeguarded scan resume failed", "job_id", jobID, "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    "Failed to resume scan",
			"details":  err.Error(),
			"job_id":   jobID,
			"duration": result.Duration.Milliseconds(),
		})
		return
	}

	logger.Info("Safeguarded scan resumed successfully",
		"job_id", jobID,
		"duration", result.Duration.Milliseconds())

	c.JSON(http.StatusOK, gin.H{
		"message":     result.Message,
		"job_id":      result.JobID,
		"operation":   string(result.Operation),
		"duration":    result.Duration.Milliseconds(),
		"safeguarded": true,
	})
}

// DeleteSafeguardedLibraryScan deletes a scan using the enhanced safeguards system
func DeleteSafeguardedLibraryScan(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Use safeguarded scan deletion
	result, err := scannerManager.DeleteSafeguardedScan(uint32(jobID))
	if err != nil {
		logger.Error("Safeguarded scan deletion failed", "job_id", jobID, "error", err)

		// Check if operation was rolled back
		if result != nil && result.WasRolledBack {
			c.JSON(http.StatusConflict, gin.H{
				"error":       "Failed to delete scan",
				"details":     err.Error(),
				"rolled_back": true,
				"duration":    result.Duration.Milliseconds(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to delete scan",
				"details": err.Error(),
			})
		}
		return
	}

	logger.Info("Safeguarded scan deleted successfully",
		"job_id", jobID,
		"duration", result.Duration.Milliseconds())

	c.JSON(http.StatusOK, gin.H{
		"message":     result.Message,
		"job_id":      result.JobID,
		"operation":   string(result.Operation),
		"duration":    result.Duration.Milliseconds(),
		"safeguarded": true,
	})
}

// GetSafeguardStatus returns the status of the safeguard system
func GetSafeguardStatus(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	status := scannerManager.GetSafeguardStatus()

	c.JSON(http.StatusOK, gin.H{
		"safeguard_status": status,
	})
}

// ForceEmergencyCleanup performs emergency cleanup of all orphaned data
func ForceEmergencyCleanup(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	logger.Info("Emergency cleanup initiated by user request")

	// Perform comprehensive cleanup
	var results []map[string]interface{}

	// 1. Cleanup orphaned jobs
	orphanedJobs, err := scannerManager.CleanupOrphanedJobs()
	if err != nil {
		logger.Error("Emergency cleanup: orphaned jobs failed", "error", err)
	}
	results = append(results, map[string]interface{}{
		"type":    "orphaned_jobs",
		"removed": orphanedJobs,
		"error":   err,
	})

	// 2. Cleanup old completed jobs
	oldJobs, err := scannerManager.CleanupOldJobs()
	if err != nil {
		logger.Error("Emergency cleanup: old jobs failed", "error", err)
	}
	results = append(results, map[string]interface{}{
		"type":    "old_jobs",
		"removed": oldJobs,
		"error":   err,
	})

	// 3. Cleanup orphaned assets
	// CleanupOrphanedAssets method was deprecated - skipping this cleanup step
	logger.Warn("Skipping deprecated CleanupOrphanedAssets - functionality removed")
	// Continue with emergency cleanup without this step
	// // CleanupOrphanedAssets deprecated - using dummy values
	assetsRemoved, filesRemoved := 0, 0
	err = errors.New("CleanupOrphanedAssets is deprecated")
	logger.Warn("Skipping deprecated CleanupOrphanedAssets - functionality removed")
	if err != nil {
		logger.Error("Emergency cleanup: orphaned assets failed", "error", err)
	}
	results = append(results, map[string]interface{}{
		"type":           "orphaned_assets",
		"assets_removed": assetsRemoved,
		"files_removed":  filesRemoved,
		"error":          err,
	})

	// 4. Cancel all active scans
	cancelledScans, err := scannerManager.CancelAllScans()
	if err != nil {
		logger.Error("Emergency cleanup: cancel scans failed", "error", err)
	}
	results = append(results, map[string]interface{}{
		"type":      "cancelled_scans",
		"cancelled": cancelledScans,
		"error":     err,
	})

	logger.Info("Emergency cleanup completed", "total_operations", len(results))

	c.JSON(http.StatusOK, gin.H{
		"message":   "Emergency cleanup completed",
		"results":   results,
		"timestamp": time.Now(),
	})
}
