package utils

import (
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// ScanJobStatus represents the possible states of a scan job
type ScanJobStatus string

const (
	StatusPending   ScanJobStatus = "pending"
	StatusRunning   ScanJobStatus = "running"
	StatusPaused    ScanJobStatus = "paused"
	StatusCompleted ScanJobStatus = "completed"
	StatusFailed    ScanJobStatus = "failed"
)

// ScanJobCleanupDays defines how many days old completed jobs should be kept
const ScanJobCleanupDays = 30

// LibraryStats represents statistics for a scanned library
type LibraryStats struct {
	TotalFiles     int64             `json:"total_files"`
	TotalSize      int64             `json:"total_size"`
	ExtensionStats []ExtensionStat   `json:"extension_stats"`
}

// ExtensionStat represents file count by extension
type ExtensionStat struct {
	Extension string `json:"extension"`
	Count     int64  `json:"count"`
}

// ValidateScanJob checks if a scan job can be started for the given library
func ValidateScanJob(db *gorm.DB, libraryID uint) error {
	// Check if library exists
	var library database.MediaLibrary
	if err := db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("library not found: %w", err)
	}

	// Check if there's already a running scan for this library
	var existingJob database.ScanJob
	err := db.Where("library_id = ? AND status IN ?", libraryID, []string{
		string(StatusPending), 
		string(StatusRunning),
	}).First(&existingJob).Error
	
	if err == nil {
		return fmt.Errorf("scan already running for library %d (job ID: %d)", libraryID, existingJob.ID)
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("database error while checking for existing scans: %w", err)
	}

	return nil
}

// CreateScanJob creates a new scan job in the database
func CreateScanJob(db *gorm.DB, libraryID uint) (*database.ScanJob, error) {
	scanJob := database.ScanJob{
		LibraryID: libraryID,
		Status:    string(StatusPending),
	}

	if err := db.Create(&scanJob).Error; err != nil {
		return nil, fmt.Errorf("failed to create scan job: %w", err)
	}

	return &scanJob, nil
}

// UpdateJobStatus updates the status of a scan job
func UpdateJobStatus(db *gorm.DB, jobID uint, status ScanJobStatus, errorMsg string) error {
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
	case StatusCompleted, StatusFailed:
		updates["completed_at"] = &now
	}

	return db.Model(&database.ScanJob{}).Where("id = ?", jobID).Updates(updates).Error
}

// GetLibraryStatistics calculates and returns statistics for a library
func GetLibraryStatistics(db *gorm.DB, libraryID uint) (*LibraryStats, error) {
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

// CleanupOldScanJobs removes old completed/failed scan jobs
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
