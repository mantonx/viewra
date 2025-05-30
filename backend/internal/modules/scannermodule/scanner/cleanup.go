package scanner

import (
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// CleanupService handles comprehensive cleanup of media files
// Note: Asset cleanup is now handled by the entity-based asset system
type CleanupService struct {
	db *gorm.DB
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(db *gorm.DB) *CleanupService {
	return &CleanupService{db: db}
}

// CleanupLibraryData removes all data associated with a library
func (c *CleanupService) CleanupLibraryData(libraryID uint) error {
	logger.Info("Starting cleanup for library", "library_id", libraryID)
	
	// Get library info for logging
	var library database.MediaLibrary
	if err := c.db.First(&library, libraryID).Error; err != nil {
		logger.Warn("Could not find library for cleanup", "library_id", libraryID, "error", err)
		// Continue with cleanup anyway
	}
	
	// Get all media files for this library (for logging)
	var mediaFiles []database.MediaFile
	if err := c.db.Where("library_id = ?", libraryID).Find(&mediaFiles).Error; err != nil {
		logger.Error("Failed to find media files for cleanup", "library_id", libraryID, "error", err)
		return fmt.Errorf("failed to find media files: %w", err)
	}
	
	logger.Info("Found media files to clean up", "library_id", libraryID, "count", len(mediaFiles))
	
	// Note: Asset cleanup is now handled separately by the entity-based asset system
	// Assets are no longer tied to media files, so they don't need cleanup here
	
	// Remove media files and their metadata
	result := c.db.Where("library_id = ?", libraryID).Delete(&database.MediaFile{})
	if result.Error != nil {
		logger.Error("Failed to delete media files", "library_id", libraryID, "error", result.Error)
		return fmt.Errorf("failed to delete media files: %w", result.Error)
	}
	
	logger.Info("Cleanup completed for library", 
		"library_id", libraryID,
		"media_files_removed", result.RowsAffected)
	
	return nil
}

// CleanupScanJobData removes data created by a specific scan job
func (c *CleanupService) CleanupScanJobData(scanJobID uint) error {
	logger.Info("Starting cleanup for scan job", "scan_job_id", scanJobID)
	
	// Get the scan job to find the library
	var scanJob database.ScanJob
	if err := c.db.First(&scanJob, scanJobID).Error; err != nil {
		return fmt.Errorf("scan job not found: %w", err)
	}
	
	// Find all media files discovered by this specific scan job
	var mediaFiles []database.MediaFile
	if err := c.db.Where("scan_job_id = ?", scanJobID).Find(&mediaFiles).Error; err != nil {
		logger.Error("Failed to find media files for scan job", "scan_job_id", scanJobID, "error", err)
		return fmt.Errorf("failed to find media files for scan job: %w", err)
	}
	
	logger.Info("Found media files to clean up for scan job", "scan_job_id", scanJobID, "count", len(mediaFiles))
	
	// Note: Asset cleanup is now handled separately by the entity-based asset system
	// Assets are no longer tied to media files, so they don't need cleanup here
	
	// Remove media files discovered by this scan job
	result := c.db.Where("scan_job_id = ?", scanJobID).Delete(&database.MediaFile{})
	if result.Error != nil {
		logger.Error("Failed to delete media files for scan job", "scan_job_id", scanJobID, "error", result.Error)
		return fmt.Errorf("failed to delete media files: %w", result.Error)
	}
	
	logger.Info("Scan job cleanup completed", 
		"scan_job_id", scanJobID, 
		"library_id", scanJob.LibraryID,
		"media_files_removed", result.RowsAffected)
	
	return nil
}

// CleanupOrphanedAssets is now deprecated - use the entity-based asset system cleanup instead
// This function is kept for compatibility but does nothing
func (c *CleanupService) CleanupOrphanedAssets() (int, int, error) {
	logger.Info("Orphaned asset cleanup is now handled by the entity-based asset system")
	logger.Info("Use the /api/v1/assets/cleanup endpoint for asset cleanup")
	return 0, 0, nil
}

// CleanupOrphanedFiles is now deprecated - use the entity-based asset system cleanup instead
// This function is kept for compatibility but does nothing
func (c *CleanupService) CleanupOrphanedFiles() (int, error) {
	logger.Info("Orphaned file cleanup is now handled by the entity-based asset system")
	logger.Info("Use the /api/v1/assets/cleanup endpoint for asset cleanup")
	return 0, nil
} 