package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/scannermodule"
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// getScannerModule retrieves the scanner module from the module registry
func getScannerModule() (*scannermodule.Module, error) {
	module, exists := modulemanager.GetModule(scannermodule.ModuleID)
	if !exists {
		return nil, fmt.Errorf("scanner module not found")
	}
	
	scannerMod, ok := module.(*scannermodule.Module)
	if !ok {
		return nil, fmt.Errorf("invalid scanner module type")
	}
	
	return scannerMod, nil
}

// getScannerManager retrieves the scanner manager from the scanner module
func getScannerManager() (*scanner.Manager, error) {
	scannerMod, err := getScannerModule()
	if err != nil {
		return nil, fmt.Errorf("failed to get scanner module: %w", err)
	}
	
	if scannerMod == nil {
		return nil, fmt.Errorf("scanner module is nil")
	}
	
	manager := scannerMod.GetScannerManager()
	if manager == nil {
		return nil, fmt.Errorf("scanner manager is nil from module")
	}
	
	return manager, nil
}

// Legacy functions for backward compatibility (deprecated)
// These are kept for any existing code that might still reference them

var scannerManager *scanner.Manager

// InitializeScanner initializes the scanner manager (deprecated - use module system)
func InitializeScanner(eventBus events.EventBus) {
	// This function is now deprecated - scanner is initialized via module system
	// We can optionally get the scanner from module system for compatibility
	if manager, err := getScannerManager(); err == nil {
		scannerManager = manager
	}
}

// InitializeScannerCompat provides backward compatibility for scanner initialization (deprecated)
func InitializeScannerCompat() {
	// This function is now deprecated - scanner is initialized via module system
	// We can optionally get the scanner from module system for compatibility
	if manager, err := getScannerManager(); err == nil {
		scannerManager = manager
	}
}

// StartLibraryScan starts a scan for a specific media library
func StartLibraryScan(c *gin.Context) {
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
	
	scanJob, err := scannerManager.StartScan(uint(libraryID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to start scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message":  "Scan started successfully",
		"scan_job": scanJob,
	})
}

// StopScan stops a running scan job
func StopScan(c *gin.Context) {
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
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	err = scannerManager.StopScan(uint(jobID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to stop scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Scan stopped successfully",
	})
}

// GetScanStatus returns the status of a specific scan job
func GetScanStatus(c *gin.Context) {
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
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	scanJob, err := scannerManager.GetScanStatus(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Scan job not found",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"scan_job": scanJob,
	})
}

// GetAllScans returns all scan jobs
func GetAllScans(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"scan_jobs": scanJobs,
		"count":     len(scanJobs),
	})
}

// GetLibraryStats returns statistics for a library's scanned files
func GetLibraryStats(c *gin.Context) {
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
	
	stats, err := scannerManager.GetLibraryStats(uint(libraryID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get library statistics",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"library_id": libraryID,
		"stats":      stats,
	})
}

// GetMediaFiles returns media files for a specific library
func GetMediaFiles(c *gin.Context) {
	libraryIDStr := c.Param("id")
	libraryID, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	var mediaFiles []database.MediaFile
	var total int64
	
	db := database.GetDB()
	
	// Get total count
	db.Model(&database.MediaFile{}).Where("library_id = ?", libraryID).Count(&total)
	
	// Get paginated results
	result := db.Where("library_id = ?", libraryID).
		Limit(limit).
		Offset(offset).
		Order("path").
		Find(&mediaFiles)
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get media files",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}

// GetCurrentJobs returns the most relevant current job for each library
func GetCurrentJobs(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	// Get all scan jobs
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	// Group jobs by library and find the most relevant one for each
	currentJobs := make(map[uint]interface{})
	
	for _, job := range scanJobs {
		existing, exists := currentJobs[job.LibraryID]
		
		var shouldReplace bool
		if !exists {
			shouldReplace = true
		} else {
			// Extract the job from the existing enhanced job data
			existingJob := existing.(map[string]interface{})["job"].(database.ScanJob)
			shouldReplace = shouldReplaceJob(existingJob, job)
		}
		
		if shouldReplace {
			// Create enhanced job data with ETA information
			enhancedJob := map[string]interface{}{
				"job": job,
			}
			
			// Add ETA and performance metrics for running jobs
			if job.Status == "running" {
				if detailedStats, err := scannerManager.GetDetailedScanProgress(job.ID); err == nil {
					// Add ETA in proper timestamp format
					if etaStr, ok := detailedStats["eta"].(string); ok && etaStr != "" {
						enhancedJob["eta"] = etaStr
					}
					
					// Add performance metrics
					if filesPerSec, ok := detailedStats["files_per_second"].(float64); ok {
						enhancedJob["files_per_second"] = filesPerSec
					}
					
					// Add real-time progress
					if processedFiles, ok := detailedStats["processed_files"].(int64); ok {
						enhancedJob["real_time_files_processed"] = processedFiles
					}
					
					if processedBytes, ok := detailedStats["processed_bytes"].(int64); ok {
						enhancedJob["real_time_bytes_processed"] = processedBytes
					}
				}
				
				// Calculate ETA using basic progress estimation if detailed stats failed
				if enhancedJob["eta"] == nil && job.FilesFound > 0 && job.FilesProcessed > 0 {
					if job.StartedAt != nil {
						elapsed := time.Since(*job.StartedAt)
						progress := float64(job.FilesProcessed) / float64(job.FilesFound)
						if progress > 0 {
							totalDuration := elapsed.Seconds() / progress
							remainingDuration := totalDuration - elapsed.Seconds()
							if remainingDuration > 0 {
								eta := time.Now().Add(time.Duration(remainingDuration) * time.Second)
								enhancedJob["eta"] = eta.Format(time.RFC3339)
							}
						}
					}
				}
			} else if job.Status == "paused" {
				// For paused jobs, calculate potential ETA if resumed
				if job.FilesFound > 0 && job.FilesProcessed > 0 && job.StartedAt != nil {
					// Calculate average rate from completed work
					elapsed := time.Since(*job.StartedAt)
					if elapsed.Seconds() > 0 {
						avgFilesPerSec := float64(job.FilesProcessed) / elapsed.Seconds()
						if avgFilesPerSec > 0 {
							remainingFiles := job.FilesFound - job.FilesProcessed
							remainingSeconds := float64(remainingFiles) / avgFilesPerSec
							eta := time.Now().Add(time.Duration(remainingSeconds) * time.Second)
							enhancedJob["eta_if_resumed"] = eta.Format(time.RFC3339)
							enhancedJob["avg_files_per_second"] = avgFilesPerSec
						}
					}
				}
			}
			
			// Add time-based information
			if job.StartedAt != nil {
				enhancedJob["elapsed_time"] = time.Since(*job.StartedAt).String()
			}
			
			currentJobs[job.LibraryID] = enhancedJob
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"current_jobs": currentJobs,
		"count":        len(currentJobs),
	})
}

// shouldReplaceJob determines if newJob should replace existingJob
func shouldReplaceJob(existing, newJob database.ScanJob) bool {
	// Priority order: running > paused > failed > completed
	statusPriority := map[string]int{
		"running":   4,
		"paused":    3,
		"failed":    2,
		"completed": 1,
	}
	
	existingPriority := statusPriority[existing.Status]
	newPriority := statusPriority[newJob.Status]
	
	// Higher priority status wins
	if newPriority > existingPriority {
		return true
	}
	
	// Same priority, prefer more recent
	if newPriority == existingPriority {
		return newJob.UpdatedAt.After(existing.UpdatedAt)
	}
	
	return false
}

func GetScannerStats(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	db := database.GetDB()
	var totalFilesScanned int64
	var totalBytesScanned int64
	activeScans := 0

	// Get all libraries
	var allLibraries []database.MediaLibrary
	if err := db.Find(&allLibraries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve libraries",
			"details": err.Error(),
		})
		return
	}

	processedLibraryIDs := make(map[uint]bool)

	// Prioritize active (running/paused) scans
	scanJobs, err := scannerManager.GetAllScans() // Assuming this gets all types of jobs
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve scan jobs",
			"details": err.Error(),
		})
		return
	}

	for _, job := range scanJobs {
		if job.Status == string(utils.StatusRunning) || job.Status == string(utils.StatusPaused) {
			if job.Status == string(utils.StatusRunning) {
				activeScans++
			}
			// Use live progress for active scans if possible
			realTimeBytesProcessed := job.BytesProcessed
			realTimeFilesProcessed := int64(job.FilesProcessed)
			if detailedStats, detailErr := scannerManager.GetDetailedScanProgress(job.ID); detailErr == nil {
				if bytesProcessed, ok := detailedStats["processed_bytes"].(int64); ok {
					realTimeBytesProcessed = bytesProcessed
				}
				if filesProcessed, ok := detailedStats["processed_files"].(int64); ok {
					realTimeFilesProcessed = filesProcessed
				}
			}
			totalBytesScanned += realTimeBytesProcessed
			totalFilesScanned += realTimeFilesProcessed
			processedLibraryIDs[job.LibraryID] = true
		}
	}

	// For libraries not actively being scanned, use their latest completed scan job stats or sum media_files
	for _, lib := range allLibraries {
		if processedLibraryIDs[lib.ID] {
			continue // Already accounted for by an active/paused scan
		}

		var latestCompletedJob database.ScanJob
		errDb := db.Where("library_id = ? AND status = ?", lib.ID, string(utils.StatusCompleted)).Order("completed_at DESC").First(&latestCompletedJob).Error
		
		if errDb == nil { // Found a completed job
			totalBytesScanned += latestCompletedJob.BytesProcessed
			totalFilesScanned += int64(latestCompletedJob.FilesProcessed)
			// processedLibraryIDs[lib.ID] = true // Mark here if we add more stages
		} else if errors.Is(errDb, gorm.ErrRecordNotFound) {
			// No completed job, sum from media_files for this library if it wasn't touched by any job at all
			// (active/paused already handled, completed handled above)
			// This path means the library has no active, paused, or completed jobs.
			var libFileStats struct { Files int64; Bytes int64 }
			db.Model(&database.MediaFile{}).Where("library_id = ?", lib.ID).Select("COALESCE(COUNT(*), 0) as files, COALESCE(SUM(size), 0) as bytes").Scan(&libFileStats)
			totalBytesScanned += libFileStats.Bytes
			totalFilesScanned += libFileStats.Files
		} else {
			// DB error fetching latest completed job, log it but don't fail the whole stat
			log.Printf("[WARN] Error fetching latest completed scan for library %d: %v\n", lib.ID, errDb)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"active_scans":          activeScans,
		"total_files_scanned":   totalFilesScanned,
		"total_bytes_scanned":   totalBytesScanned,
	})
}

// GetScannerStatus returns all running and recent scan jobs
func GetScannerStatus(c *gin.Context) {
	if scannerManager == nil {
		InitializeScannerCompat()
	}
	
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"jobs": scanJobs,
		"count": len(scanJobs),
	})
}

// StartLibraryScanByID starts a scan for a library (for frontend compatibility)
func StartLibraryScanByID(c *gin.Context) {
	StartLibraryScan(c)
}

// StopLibraryScan pauses a scan for a specific library (finds job by library ID)
func StopLibraryScan(c *gin.Context) {
	libraryIDStr := c.Param("id")
	libraryID, err := strconv.ParseUint(libraryIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}
	
	if scannerManager == nil {
		InitializeScannerCompat()
	}
	
	// Find running scan job for this library
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	var jobID uint
	found := false
	for _, job := range scanJobs {
		if job.LibraryID == uint(libraryID) && job.Status == "running" {
			jobID = job.ID
			found = true
			break
		}
	}
	
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No running scan found for this library",
		})
		return
	}
	
	err = scannerManager.StopScan(jobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to stop scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Scan paused successfully",
		"library_id": libraryID,
		"job_id": jobID,
	})
}

// GetScanProgress handles GET /api/scanner/progress/:id requests
func GetScanProgress(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.Debug("Invalid scan job ID in progress request", "id", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid scan job ID",
		})
		return
	}

	// Safely get scanner manager with comprehensive error handling
	scannerManager, err := getScannerManager()
	if err != nil {
		logger.Error("Scanner manager not available for progress request", "job_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner manager not available",
			"details": err.Error(),
		})
		return
	}
	
	if scannerManager == nil {
		logger.Error("Scanner manager is nil for progress request", "job_id", id)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner manager is nil",
		})
		return
	}

	// Try to get detailed progress including worker stats
	detailedStats, err := func() (map[string]interface{}, error) {
		var detailedErr error
		defer func() {
			if r := recover(); r != nil {
				detailedErr = fmt.Errorf("panic in GetDetailedScanProgress: %v", r)
				logger.Error("Panic in GetDetailedScanProgress", "job_id", id, "panic", r)
			}
		}()
		stats, detailedErr := scannerManager.GetDetailedScanProgress(uint(id))
		return stats, detailedErr
	}()
	
	if err == nil && detailedStats != nil {
		// Get additional info from database to enrich the detailed stats
		if scanJob, scanErr := func() (*database.ScanJob, error) {
			var dbErr error
			defer func() {
				if r := recover(); r != nil {
					dbErr = fmt.Errorf("panic in GetScanStatus: %v", r)
					logger.Error("Panic in GetScanStatus", "job_id", id, "panic", r)
				}
			}()
			job, dbErr := scannerManager.GetScanStatus(uint(id))
			return job, dbErr
		}(); scanErr == nil && scanJob != nil {
			// Add database fields not included in detailed stats
			detailedStats["files_found"] = scanJob.FilesFound
			detailedStats["status"] = scanJob.Status
		}
		
		logger.Debug("Returning detailed progress stats", "job_id", id)
		// Return detailed stats for active jobs
		c.JSON(http.StatusOK, detailedStats)
		return
	}
	
	// If detailed stats are not available, fall back to basic progress
	progress, eta, filesPerSec, progressErr := func() (float64, string, float64, error) {
		var progErr error
		defer func() {
			if r := recover(); r != nil {
				progErr = fmt.Errorf("panic in GetScanProgress: %v", r)
				logger.Error("Panic in GetScanProgress", "job_id", id, "panic", r)
			}
		}()
		prog, etaStr, fps, progErr := scannerManager.GetScanProgress(uint(id))
		return prog, etaStr, fps, progErr
	}()
	
	if progressErr == nil {
		// Get additional info from database
		if scanJob, scanErr := func() (*database.ScanJob, error) {
			var dbErr error
			defer func() {
				if r := recover(); r != nil {
					dbErr = fmt.Errorf("panic in GetScanStatus: %v", r)
					logger.Error("Panic in GetScanStatus", "job_id", id, "panic", r)
				}
			}()
			job, dbErr := scannerManager.GetScanStatus(uint(id))
			return job, dbErr
		}(); scanErr == nil && scanJob != nil {
			logger.Debug("Returning basic progress with database info", "job_id", id)
			c.JSON(http.StatusOK, gin.H{
				"progress":        progress,
				"eta":             eta,
				"files_per_sec":   filesPerSec,
				"bytes_processed": scanJob.BytesProcessed,
				"files_processed": scanJob.FilesProcessed,
				"files_found":     scanJob.FilesFound,
				"status":          scanJob.Status,
			})
			return
		}
		
		logger.Debug("Returning basic progress without database info", "job_id", id)
		// Return basic progress without database info if DB access fails
		c.JSON(http.StatusOK, gin.H{
			"progress":      progress,
			"eta":           eta,
			"files_per_sec": filesPerSec,
		})
		return
	}
	
	// If no active scanner, try to get progress from database only
	if scanJob, dbErr := func() (*database.ScanJob, error) {
		var dbError error
		defer func() {
			if r := recover(); r != nil {
				dbError = fmt.Errorf("panic in GetScanStatus: %v", r)
				logger.Error("Panic in GetScanStatus", "job_id", id, "panic", r)
			}
		}()
		job, dbError := scannerManager.GetScanStatus(uint(id))
		return job, dbError
	}(); dbErr == nil && scanJob != nil {
		logger.Debug("Returning database-only progress", "job_id", id, "status", scanJob.Status)
		// Return database progress for inactive jobs
		c.JSON(http.StatusOK, gin.H{
			"progress":        scanJob.Progress,
			"eta":             "",
			"files_per_sec":   0,
			"bytes_processed": scanJob.BytesProcessed,
			"files_processed": scanJob.FilesProcessed,
			"files_found":     scanJob.FilesFound,
			"status":          scanJob.Status,
		})
		return
	}
	
	// All methods failed - this is likely during cleanup/deletion
	logger.Info("Scan job not found in progress request", "job_id", id, "detailed_error", err, "progress_error", progressErr)
	c.JSON(http.StatusNotFound, gin.H{
		"error": "Scan job not found or scanner manager unavailable",
		"details": fmt.Sprintf("Job %d may have been deleted or cleaned up", id),
		"job_id": id,
	})
}

// =============================================================================
// DIRECTORY-BASED SCAN HANDLERS
// =============================================================================

// StartDirectoryScan starts a scan on a specified directory path
func StartDirectoryScan(c *gin.Context) {
	var request struct {
		Directory string `json:"directory" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
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
	
	// For now, we'll use library ID 1 as default or create a temporary library
	// In a full implementation, you might want to create a temporary library entry
	// or modify the scanner to work with direct paths
	
	// First, let's try to get or create a library for this directory
	db := database.GetDB()
	
	// Check if a library exists for this path
	var library database.MediaLibrary
	err = db.Where("path = ?", request.Directory).First(&library).Error
	if err != nil {
		// Create a temporary library entry
		library = database.MediaLibrary{
			Path: request.Directory,
			Type: "mixed",
		}
		if err := db.Create(&library).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create temporary library",
				"details": err.Error(),
			})
			return
		}
	}
	
	scanJob, err := scannerManager.StartScan(library.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to start scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message":  "Scan started successfully",
		"scanId":   fmt.Sprintf("%d", scanJob.ID),
		"scan_job": scanJob,
	})
}

// ResumeScan resumes a paused scan job
func ResumeScan(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid scan ID",
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
	
	err = scannerManager.ResumeScan(uint(jobID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to resume scan",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Scan resumed successfully",
		"scan_id": jobID,
	})
}

// GetScanResults gets the final results of a completed scan
func GetScanResults(c *gin.Context) {
	jobIDStr := c.Param("id")
	jobID, err := strconv.ParseUint(jobIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid scan ID",
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
	
	scanJob, err := scannerManager.GetScanStatus(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Scan job not found",
		})
		return
	}
	
	// Get additional results from database
	db := database.GetDB()
	
	var mediaFiles []database.MediaFile
	var mediaCount int64
	
	if scanJob.LibraryID > 0 {
		db.Where("library_id = ?", scanJob.LibraryID).Find(&mediaFiles)
		db.Model(&database.MediaFile{}).Where("library_id = ?", scanJob.LibraryID).Count(&mediaCount)
	}
	
	// Calculate percentage
	var percentage float64
	if scanJob.FilesFound > 0 {
		percentage = float64(scanJob.FilesProcessed) / float64(scanJob.FilesFound) * 100
	}
	
	c.JSON(http.StatusOK, gin.H{
		"scan_job":     scanJob,
		"media_files":  mediaFiles,
		"media_count":  mediaCount,
		"percentage":   percentage,
	})
}

// CleanupOrphanedJobs removes scan jobs for libraries that no longer exist
func CleanupOrphanedJobs(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	jobsDeleted, err := scannerManager.CleanupOrphanedJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cleanup orphaned jobs",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":      "Orphaned scan jobs cleaned up successfully",
		"jobs_deleted": jobsDeleted,
	})
}

// GetMonitoringStatus returns the file monitoring status for all libraries
func GetMonitoringStatus(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	monitoringStatus := scannerManager.GetMonitoringStatus()
	
	c.JSON(http.StatusOK, gin.H{
		"monitoring_status": monitoringStatus,
		"monitoring_count":  len(monitoringStatus),
	})
}

// CleanupOrphanedAssets removes assets that reference non-existent media files
func CleanupOrphanedAssets(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	assetsRemoved, filesRemoved, err := scannerManager.CleanupOrphanedAssets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cleanup orphaned assets",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":        "Orphaned assets cleaned up successfully",
		"assets_removed": assetsRemoved,
		"files_removed":  filesRemoved,
	})
}

// CleanupOrphanedFiles removes asset files from disk that have no corresponding database records
func CleanupOrphanedFiles(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	filesRemoved, err := scannerManager.CleanupOrphanedFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to cleanup orphaned files",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":       "Orphaned files cleaned up successfully",
		"files_removed": filesRemoved,
	})
}

// DeleteScanJob removes a scan job and all its discovered files and assets
func DeleteScanJob(c *gin.Context) {
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
			"error": "Scanner module not available",
			"details": err.Error(),
		})
		return
	}
	
	// Check if the scan job exists first
	scanJob, err := scannerManager.GetScanStatus(uint(jobID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Scan job not found",
			"details": err.Error(),
		})
		return
	}
	
	// If the scan is running, we need to stop it first and wait for it to actually stop
	if scanJob.Status == "running" {
		logger.Info("Stopping running scan before deletion", "job_id", jobID)
		
		if err := scannerManager.StopScan(uint(jobID)); err != nil {
			logger.Error("Failed to stop scan job", "job_id", jobID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to stop running scan job",
				"details": err.Error(),
			})
			return
		}
		
		// Wait for the scan to actually stop (with timeout)
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-timeout:
				logger.Error("Timeout waiting for scan to stop", "job_id", jobID)
				c.JSON(http.StatusRequestTimeout, gin.H{
					"error": "Timeout waiting for scan to stop",
					"details": "The scan job did not stop within 30 seconds",
				})
				return
			case <-ticker.C:
				// Check if scan has stopped
				currentJob, err := scannerManager.GetScanStatus(uint(jobID))
				if err != nil {
					// Job might have been cleaned up already
					break
				}
				if currentJob.Status != "running" {
					logger.Info("Scan successfully stopped", "job_id", jobID, "final_status", currentJob.Status)
					goto scanStopped
				}
			}
		}
		
		scanStopped:
		// Give it a moment to fully clean up
		time.Sleep(1 * time.Second)
	}
	
	// Clean up the scan job and all its data
	logger.Info("Starting cleanup for scan job", "job_id", jobID)
	if err := scannerManager.CleanupScanJob(uint(jobID)); err != nil {
		logger.Error("Failed to cleanup scan job", "job_id", jobID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete scan job",
			"details": err.Error(),
		})
		return
	}
	
	logger.Info("Scan job successfully deleted", "job_id", jobID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Scan job and all its data removed successfully",
		"job_id":  jobID,
	})
}
