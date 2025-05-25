package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
	"github.com/yourusername/viewra/internal/scanner"
)

var scannerManager *scanner.Manager

// InitializeScanner initializes the scanner manager
func InitializeScanner() {
	scannerManager = scanner.NewManager(database.GetDB())
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
	
	if scannerManager == nil {
		InitializeScanner()
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
	
	if scannerManager == nil {
		InitializeScanner()
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
	
	if scannerManager == nil {
		InitializeScanner()
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
	if scannerManager == nil {
		InitializeScanner()
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
	
	if scannerManager == nil {
		InitializeScanner()
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

// GetScannerStats returns overall scanner statistics
func GetScannerStats(c *gin.Context) {
	if scannerManager == nil {
		InitializeScanner()
	}
	
	// Get active scan count
	scanJobs, err := scannerManager.GetAllScans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan jobs",
			"details": err.Error(),
		})
		return
	}
	
	activeScans := 0
	for _, job := range scanJobs {
		if job.Status == "running" {
			activeScans++
		}
	}
	
	// Get total files and bytes scanned from database
	var totalFilesScanned int64
	var totalBytesScanned int64
	
	db := database.GetDB()
	db.Model(&database.MediaFile{}).Count(&totalFilesScanned)
	db.Model(&database.MediaFile{}).Select("COALESCE(SUM(size), 0)").Scan(&totalBytesScanned)
	
	c.JSON(http.StatusOK, gin.H{
		"active_scans":          activeScans,
		"total_files_scanned":   totalFilesScanned,
		"total_bytes_scanned":   totalBytesScanned,
	})
}

// GetScannerStatus returns all running and recent scan jobs
func GetScannerStatus(c *gin.Context) {
	if scannerManager == nil {
		InitializeScanner()
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
		InitializeScanner()
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
