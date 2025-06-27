package utils

import (
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// ScanJobStatus represents the possible states of a scan job.
// These states track the lifecycle of a media library scanning operation.
type ScanJobStatus string

const (
	StatusPending   ScanJobStatus = "pending"
	StatusRunning   ScanJobStatus = "running"
	StatusPaused    ScanJobStatus = "paused"
	StatusCompleted ScanJobStatus = "completed"
	StatusFailed    ScanJobStatus = "failed"
)

// ScanJobCleanupDays defines how many days old completed jobs should be kept.
// After this period, old scan job records are automatically removed to prevent
// database bloat while maintaining a reasonable audit trail.
const ScanJobCleanupDays = 30

// LibraryStats represents statistics for a scanned library.
// These statistics provide insights into the composition and size of
// a media library after scanning.
type LibraryStats struct {
	TotalFiles     int64           `json:"total_files"`
	TotalSize      int64           `json:"total_size"`
	ExtensionStats []ExtensionStat `json:"extension_stats"`
}

// ExtensionStat represents file count by extension.
// Used to show the distribution of file types in a library,
// helping identify the most common media formats.
type ExtensionStat struct {
	Extension string `json:"extension"`
	Count     int64  `json:"count"`
}

// ValidateScanJob checks if a scan job can be started for the given library.
// This function performs several validations:
//   - Verifies the library exists
//   - Cleans up old failed/paused jobs
//   - Prevents duplicate scans on the same library
//   - Prevents duplicate scans on the same path
//
// Returns an error if scanning cannot proceed.
func ValidateScanJob(db *gorm.DB, libraryID uint32) error {
	// Check if library exists
	var library database.MediaLibrary
	if err := db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("library not found: %w", err)
	}

	// Clean up old failed/paused scan jobs for this library (keep only the most recent paused job)
	var oldJobs []database.ScanJob
	err := db.Where("library_id = ? AND status IN ?", libraryID, []string{
		string(StatusPaused),
		string(StatusFailed),
	}).Order("updated_at DESC").Find(&oldJobs).Error

	if err == nil && len(oldJobs) > 1 {
		// Keep the most recent paused/failed job, delete the rest
		jobsToDelete := oldJobs[1:] // Skip the first (most recent) one
		var idsToDelete []uint32
		for _, job := range jobsToDelete {
			idsToDelete = append(idsToDelete, job.ID)
		}

		if len(idsToDelete) > 0 {
			result := db.Where("id IN ?", idsToDelete).Delete(&database.ScanJob{})
			if result.Error == nil && result.RowsAffected > 0 {
				fmt.Printf("Cleaned up %d old scan jobs for library %d\n", result.RowsAffected, libraryID)
			}
		}
	}

	// Check if there's already a running scan for this library ID
	var existingJobForLibrary database.ScanJob
	err = db.Where("library_id = ? AND status IN ?", libraryID, []string{
		string(StatusPending),
		string(StatusRunning),
	}).First(&existingJobForLibrary).Error

	if err == nil {
		return fmt.Errorf("scan already running for library %d (job ID: %d)", libraryID, existingJobForLibrary.ID)
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("database error while checking for existing scans: %w", err)
	}

	// Check if there's already a running scan for the same path (from any library)
	var existingJobForPath database.ScanJob
	err = db.Joins("JOIN media_libraries ON media_libraries.id = scan_jobs.library_id").
		Where("media_libraries.path = ? AND scan_jobs.status IN ?", library.Path, []string{
			string(StatusPending),
			string(StatusRunning),
		}).
		First(&existingJobForPath).Error

	if err == nil {
		return fmt.Errorf("scan already running for path '%s' (job ID: %d, library ID: %d)",
			library.Path, existingJobForPath.ID, existingJobForPath.LibraryID)
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("database error while checking for existing path scans: %w", err)
	}

	return nil
}

// CreateScanJob creates a new scan job in the database.
// The job is created in "pending" status and will be picked up
// by the scanner when it becomes available.
func CreateScanJob(db *gorm.DB, libraryID uint32) (*database.ScanJob, error) {
	scanJob := database.ScanJob{
		LibraryID: libraryID,
		Status:    string(StatusPending),
	}

	if err := db.Create(&scanJob).Error; err != nil {
		return nil, fmt.Errorf("failed to create scan job: %w", err)
	}

	return &scanJob, nil
}

// UpdateJobStatus updates the status of a scan job.
// This function handles all status transitions and updates relevant
// timestamps (started_at, completed_at) based on the new status.
// For failed jobs, an error message can be provided.
func UpdateJobStatus(db *gorm.DB, jobID uint32, status ScanJobStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status": string(status),
	}

	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	now := time.Now()
	switch status {
	case StatusRunning:
		updates["started_at"] = &now
		updates["resumed_at"] = &now
		updates["status_message"] = ""
		updates["error_message"] = ""
	case StatusCompleted, StatusFailed:
		updates["completed_at"] = &now
	}

	return db.Model(&database.ScanJob{}).Where("id = ?", jobID).Updates(updates).Error
}

// GetLibraryStatistics calculates and returns statistics for a library.
// Provides aggregate data including:
//   - Total number of media files
//   - Total storage size used
//   - Breakdown of files by extension (top 10)
//
// This information is useful for library management and optimization.
func GetLibraryStatistics(db *gorm.DB, libraryID uint32) (*LibraryStats, error) {
	var stats struct {
		TotalFiles int64 `json:"total_files"`
		TotalSize  int64 `json:"total_size"`
	}

	err := db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("COUNT(*) as total_files, COALESCE(SUM(size), 0) as total_size").
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get basic library stats: %w", err)
	}

	// Get file extension breakdown
	var extensionStats []ExtensionStat
	err = db.Model(&database.MediaFile{}).
		Where("library_id = ?", libraryID).
		Select("LOWER(SUBSTR(path, LENGTH(path) - INSTR(REVERSE(path), '.') + 1)) as extension, COUNT(*) as count").
		Group("extension").
		Order("count DESC").
		Limit(10).
		Scan(&extensionStats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get extension stats: %w", err)
	}

	return &LibraryStats{
		TotalFiles:     stats.TotalFiles,
		TotalSize:      stats.TotalSize,
		ExtensionStats: extensionStats,
	}, nil
}

// CleanupOldScanJobs removes old completed/failed scan jobs.
// Jobs older than ScanJobCleanupDays are deleted to prevent database bloat.
// This should be run periodically as part of maintenance tasks.
// Returns the number of jobs cleaned up.
func CleanupOldScanJobs(db *gorm.DB) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -ScanJobCleanupDays)

	result := db.Where("status IN ? AND completed_at < ?", []string{
		string(StatusCompleted),
		string(StatusFailed),
		string(StatusPaused),
	}, cutoff).Delete(&database.ScanJob{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old scan jobs: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// CleanupSkippedFiles removes files from the database that should be skipped
// based on the SkippedExtensions list (e.g., trickplay files, subtitles, etc.).
//
// This function is useful for cleaning up libraries that were scanned before
// skip rules were implemented or updated. It processes files in batches
// for better performance with large libraries.
func CleanupSkippedFiles(db *gorm.DB, libraryID uint32) error {
	// Get all media files for this library
	var mediaFiles []database.MediaFile
	if err := db.Where("library_id = ?", libraryID).Find(&mediaFiles).Error; err != nil {
		return fmt.Errorf("failed to get media files: %w", err)
	}

	var filesToDelete []string
	skippedCount := 0

	for _, file := range mediaFiles {
		if IsSkippedFile(file.Path) {
			filesToDelete = append(filesToDelete, file.ID)
			skippedCount++
		}
	}

	if len(filesToDelete) > 0 {
		// Delete in batches for better performance
		batchSize := 100
		for i := 0; i < len(filesToDelete); i += batchSize {
			end := i + batchSize
			if end > len(filesToDelete) {
				end = len(filesToDelete)
			}

			batch := filesToDelete[i:end]
			if err := db.Where("id IN ?", batch).Delete(&database.MediaFile{}).Error; err != nil {
				return fmt.Errorf("failed to delete skipped files batch: %w", err)
			}
		}

		fmt.Printf("Cleaned up %d skipped files (trickplay, subtitles, etc.) from library %d\n",
			skippedCount, libraryID)
	}

	return nil
}

// CleanupDuplicateScanJobs removes duplicate scan jobs for a library, keeping only the most recent one.
// This maintenance function helps clean up situations where multiple scan jobs
// were created for the same library due to bugs or race conditions.
// The most recent job is preserved while older duplicates are removed.
func CleanupDuplicateScanJobs(db *gorm.DB, libraryID uint32) error {
	// Get all scan jobs for this library
	var scanJobs []database.ScanJob
	if err := db.Where("library_id = ?", libraryID).Order("created_at DESC").Find(&scanJobs).Error; err != nil {
		return fmt.Errorf("failed to get scan jobs: %w", err)
	}

	if len(scanJobs) <= 1 {
		return nil // No duplicates
	}

	// Keep the most recent job, delete the rest
	jobsToDelete := scanJobs[1:] // Skip the first (most recent) one
	var idsToDelete []uint32
	for _, job := range jobsToDelete {
		idsToDelete = append(idsToDelete, job.ID)
	}

	if len(idsToDelete) > 0 {
		result := db.Where("id IN ?", idsToDelete).Delete(&database.ScanJob{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete duplicate scan jobs: %w", result.Error)
		}

		fmt.Printf("Cleaned up %d duplicate scan jobs for library %d, kept job %d\n",
			result.RowsAffected, libraryID, scanJobs[0].ID)
	}

	return nil
}
