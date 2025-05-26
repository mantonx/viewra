package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/scannermodule"
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
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
		return nil, err
	}
	
	return scannerMod.GetScannerManager(), nil
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
	
	// Get active scan jobs
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	activeScans := 0
	var totalFilesScanned int64
	var totalBytesScanned int64
	
	// Track which libraries have active or paused scans
	var librariesWithScans []uint
	
	// Get progress from all scan jobs (running, paused, etc.)
	for _, job := range scanJobs {
		if job.Status == "running" {
			activeScans++
			// Try to get real-time progress for running jobs
			realTimeFilesProcessed := int64(job.FilesProcessed)
			realTimeBytesProcessed := job.BytesProcessed
			
			if detailedStats, err := scannerManager.GetDetailedScanProgress(job.ID); err == nil {
				if filesProcessed, ok := detailedStats["processed_files"].(int64); ok {
					realTimeFilesProcessed = filesProcessed
				}
				if bytesProcessed, ok := detailedStats["processed_bytes"].(int64); ok {
					realTimeBytesProcessed = bytesProcessed
				}
			}
			
			totalFilesScanned += realTimeFilesProcessed
			totalBytesScanned += realTimeBytesProcessed
			librariesWithScans = append(librariesWithScans, job.LibraryID)
		} else if job.Status == "paused" || job.Status == "completed" {
			// Include progress from paused and completed jobs
			totalFilesScanned += int64(job.FilesProcessed)
			totalBytesScanned += job.BytesProcessed
			librariesWithScans = append(librariesWithScans, job.LibraryID)
		}
	}
	
	// Add files from libraries that don't have any scan jobs
	if len(librariesWithScans) > 0 {
		db := database.GetDB()
		var completedFiles int64
		var completedBytes int64
		
		// Get files from libraries without any scan jobs
		db.Raw(`
			SELECT COALESCE(COUNT(*), 0) as files, COALESCE(SUM(size), 0) as bytes 
			FROM media_files mf 
			WHERE mf.library_id NOT IN (?)
		`, librariesWithScans).Scan(&struct {
			Files int64 `json:"files"`
			Bytes int64 `json:"bytes"`
		}{Files: completedFiles, Bytes: completedBytes})
		
		totalFilesScanned += completedFiles
		totalBytesScanned += completedBytes
	} else {
		// No scan jobs exist, count all files in database
		db := database.GetDB()
		db.Model(&database.MediaFile{}).Count(&totalFilesScanned)
		db.Model(&database.MediaFile{}).Select("COALESCE(SUM(size), 0)").Scan(&totalBytesScanned)
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid scan job ID",
		})
		return
	}

	// Try to get detailed progress including worker stats
	detailedStats, err := scannerManager.GetDetailedScanProgress(uint(id))
	if err == nil {
		// Get additional info from database to enrich the detailed stats
		scanJob, _ := scannerManager.GetScanStatus(uint(id))
		
		// Add database fields not included in detailed stats
		detailedStats["files_found"] = scanJob.FilesFound
		detailedStats["status"] = scanJob.Status
		
		// Return detailed stats for active jobs
		c.JSON(http.StatusOK, detailedStats)
		return
	}
	
	// If detailed stats are not available, fall back to basic progress
	progress, eta, filesPerSec, err := scannerManager.GetScanProgress(uint(id))
	if err != nil {
		// If no active scanner, try to get progress from database
		scanJob, dbErr := scannerManager.GetScanStatus(uint(id))
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
			"files_found":     scanJob.FilesFound,
			"status":          scanJob.Status,
		})
		return
	}

	// Get additional info from database
	scanJob, _ := scannerManager.GetScanStatus(uint(id))
	
	c.JSON(http.StatusOK, gin.H{
		"progress":        progress,
		"eta":             eta,
		"files_per_sec":   filesPerSec,
		"bytes_processed": scanJob.BytesProcessed,
		"files_processed": scanJob.FilesProcessed,
		"files_found":     scanJob.FilesFound,
		"status":          scanJob.Status,
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
		"scanId":       jobID,
		"status":       scanJob.Status,
		"totalFiles":   scanJob.FilesFound,
		"processedFiles": scanJob.FilesProcessed,
		"mediaFiles":   mediaCount,
		"percentage":   percentage,
		"results":      mediaFiles,
		"scan_job":     scanJob,
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
