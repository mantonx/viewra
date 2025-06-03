package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
)

// ScanConfigResponse represents the scanning configuration response
type ScanConfigResponse struct {
	ParallelScanningEnabled bool `json:"parallel_scanning_enabled"`
	WorkerCount             int  `json:"worker_count"`
	BatchSize               int  `json:"batch_size"`
	ChannelBufferSize       int  `json:"channel_buffer_size"`
	SmartHashEnabled        bool `json:"smart_hash_enabled"`
	AsyncMetadataEnabled    bool `json:"async_metadata_enabled"`
	MetadataWorkerCount     int  `json:"metadata_worker_count"`
}

// ScanConfigRequest represents the scanning configuration request
type ScanConfigRequest struct {
	Profile             string `json:"profile,omitempty"` // "default", "conservative", "aggressive"
	ParallelScanning    *bool  `json:"parallel_scanning,omitempty"`
	WorkerCount         *int   `json:"worker_count,omitempty"`
	BatchSize           *int   `json:"batch_size,omitempty"`
	SmartHashEnabled    *bool  `json:"smart_hash_enabled,omitempty"`
	AsyncMetadata       *bool  `json:"async_metadata,omitempty"`
	MetadataWorkerCount *int   `json:"metadata_worker_count,omitempty"`
}

// GetScanConfig returns the current scanning configuration
func GetScanConfig(c *gin.Context) {
	_, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Get current configuration from scanner manager
	config := scanner.DefaultScanConfig()

	response := ScanConfigResponse{
		ParallelScanningEnabled: config.ParallelScanningEnabled,
		WorkerCount:             config.WorkerCount,
		BatchSize:               config.BatchSize,
		ChannelBufferSize:       config.ChannelBufferSize,
		SmartHashEnabled:        config.SmartHashEnabled,
		AsyncMetadataEnabled:    config.AsyncMetadataEnabled,
		MetadataWorkerCount:     config.MetadataWorkerCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"config": response,
	})
}

// UpdateScanConfig updates the scanning configuration
func UpdateScanConfig(c *gin.Context) {
	scannerManager, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	var request ScanConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get base configuration
	var config *scanner.ScanConfig
	switch request.Profile {
	case "conservative":
		config = scanner.ConservativeScanConfig()
	case "aggressive":
		config = scanner.AggressiveScanConfig()
	default:
		config = scanner.DefaultScanConfig()
	}

	// Apply individual overrides
	if request.ParallelScanning != nil {
		config.ParallelScanningEnabled = *request.ParallelScanning
	}
	if request.WorkerCount != nil {
		config.WorkerCount = *request.WorkerCount
	}
	if request.BatchSize != nil {
		config.BatchSize = *request.BatchSize
	}
	if request.SmartHashEnabled != nil {
		config.SmartHashEnabled = *request.SmartHashEnabled
	}
	if request.AsyncMetadata != nil {
		config.AsyncMetadataEnabled = *request.AsyncMetadata
	}
	if request.MetadataWorkerCount != nil {
		config.MetadataWorkerCount = *request.MetadataWorkerCount
	}

	// Update scanner manager configuration
	scannerManager.SetParallelMode(config.ParallelScanningEnabled)

	response := ScanConfigResponse{
		ParallelScanningEnabled: config.ParallelScanningEnabled,
		WorkerCount:             config.WorkerCount,
		BatchSize:               config.BatchSize,
		ChannelBufferSize:       config.ChannelBufferSize,
		SmartHashEnabled:        config.SmartHashEnabled,
		AsyncMetadataEnabled:    config.AsyncMetadataEnabled,
		MetadataWorkerCount:     config.MetadataWorkerCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Scan configuration updated successfully",
		"config":  response,
	})
}

// GetScanPerformanceStats returns performance statistics for scanning operations
func GetScanPerformanceStats(c *gin.Context) {
	_, err := getScannerManager()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scanner module not available",
			"details": err.Error(),
		})
		return
	}

	// Get recent scan jobs with timing information
	var scanJobs []struct {
		ID              uint    `json:"id"`
		LibraryID       uint    `json:"library_id"`
		Status          string  `json:"status"`
		FilesFound      int     `json:"files_found"`
		FilesProcessed  int     `json:"files_processed"`
		BytesProcessed  int64   `json:"bytes_processed"`
		StartedAt       string  `json:"started_at,omitempty"`
		CompletedAt     string  `json:"completed_at,omitempty"`
		DurationSeconds int     `json:"duration_seconds,omitempty"`
		FilesPerSecond  float64 `json:"files_per_second,omitempty"`
		MBPerSecond     float64 `json:"mb_per_second,omitempty"`
	}

	db := database.GetDB()

	// Query recent completed scan jobs
	rows, err := db.Raw(`
		SELECT 
			id, library_id, status, files_found, files_processed, bytes_processed,
			started_at, completed_at,
			CASE 
				WHEN started_at IS NOT NULL AND completed_at IS NOT NULL 
				THEN CAST((julianday(completed_at) - julianday(started_at)) * 86400 AS INTEGER)
				ELSE 0 
			END as duration_seconds,
			CASE 
				WHEN started_at IS NOT NULL AND completed_at IS NOT NULL AND 
					 (julianday(completed_at) - julianday(started_at)) * 86400 > 0
				THEN files_processed / ((julianday(completed_at) - julianday(started_at)) * 86400)
				ELSE 0 
			END as files_per_second,
			CASE 
				WHEN started_at IS NOT NULL AND completed_at IS NOT NULL AND 
					 (julianday(completed_at) - julianday(started_at)) * 86400 > 0
				THEN (bytes_processed / 1048576.0) / ((julianday(completed_at) - julianday(started_at)) * 86400)
				ELSE 0 
			END as mb_per_second
		FROM scan_jobs 
		WHERE status = 'completed' 
		ORDER BY completed_at DESC 
		LIMIT 10
	`).Rows()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get scan performance stats",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var job struct {
			ID              uint    `json:"id"`
			LibraryID       uint    `json:"library_id"`
			Status          string  `json:"status"`
			FilesFound      int     `json:"files_found"`
			FilesProcessed  int     `json:"files_processed"`
			BytesProcessed  int64   `json:"bytes_processed"`
			StartedAt       *string `json:"started_at,omitempty"`
			CompletedAt     *string `json:"completed_at,omitempty"`
			DurationSeconds int     `json:"duration_seconds,omitempty"`
			FilesPerSecond  float64 `json:"files_per_second,omitempty"`
			MBPerSecond     float64 `json:"mb_per_second,omitempty"`
		}

		if err := rows.Scan(
			&job.ID, &job.LibraryID, &job.Status, &job.FilesFound,
			&job.FilesProcessed, &job.BytesProcessed, &job.StartedAt,
			&job.CompletedAt, &job.DurationSeconds, &job.FilesPerSecond,
			&job.MBPerSecond,
		); err != nil {
			continue
		}

		scanJobs = append(scanJobs, struct {
			ID              uint    `json:"id"`
			LibraryID       uint    `json:"library_id"`
			Status          string  `json:"status"`
			FilesFound      int     `json:"files_found"`
			FilesProcessed  int     `json:"files_processed"`
			BytesProcessed  int64   `json:"bytes_processed"`
			StartedAt       string  `json:"started_at,omitempty"`
			CompletedAt     string  `json:"completed_at,omitempty"`
			DurationSeconds int     `json:"duration_seconds,omitempty"`
			FilesPerSecond  float64 `json:"files_per_second,omitempty"`
			MBPerSecond     float64 `json:"mb_per_second,omitempty"`
		}{
			ID:              job.ID,
			LibraryID:       job.LibraryID,
			Status:          job.Status,
			FilesFound:      job.FilesFound,
			FilesProcessed:  job.FilesProcessed,
			BytesProcessed:  job.BytesProcessed,
			StartedAt:       safeString(job.StartedAt),
			CompletedAt:     safeString(job.CompletedAt),
			DurationSeconds: job.DurationSeconds,
			FilesPerSecond:  job.FilesPerSecond,
			MBPerSecond:     job.MBPerSecond,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_scans": scanJobs,
	})
}

// safeString converts a string pointer to a string
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
