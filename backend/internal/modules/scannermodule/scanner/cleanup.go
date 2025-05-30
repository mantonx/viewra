package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
	"gorm.io/gorm"
)

// CleanupService handles comprehensive cleanup of media files, assets, and disk files
type CleanupService struct {
	db *gorm.DB
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(db *gorm.DB) *CleanupService {
	return &CleanupService{db: db}
}

// CleanupLibraryData removes all data associated with a library
func (c *CleanupService) CleanupLibraryData(libraryID uint) error {
	logger.Info("Starting comprehensive cleanup for library", "library_id", libraryID)
	
	// Get library info for asset path cleanup
	var library database.MediaLibrary
	if err := c.db.First(&library, libraryID).Error; err != nil {
		logger.Warn("Could not find library for cleanup", "library_id", libraryID, "error", err)
		// Continue with cleanup anyway
	}
	
	// 1. Get all media files for this library (for asset cleanup)
	var mediaFiles []database.MediaFile
	if err := c.db.Where("library_id = ?", libraryID).Find(&mediaFiles).Error; err != nil {
		logger.Error("Failed to find media files for cleanup", "library_id", libraryID, "error", err)
		return fmt.Errorf("failed to find media files: %w", err)
	}
	
	logger.Info("Found media files to clean up", "library_id", libraryID, "count", len(mediaFiles))
	
	// 2. Clean up assets for each media file
	totalAssetsRemoved := 0
	totalAssetFilesRemoved := 0
	
	for _, mediaFile := range mediaFiles {
		assetsRemoved, filesRemoved, err := c.cleanupMediaFileAssets(mediaFile.ID)
		if err != nil {
			logger.Error("Failed to cleanup assets for media file", "media_file_id", mediaFile.ID, "error", err)
			// Continue with other files
		}
		totalAssetsRemoved += assetsRemoved
		totalAssetFilesRemoved += filesRemoved
	}
	
	// 3. Remove media files
	result := c.db.Where("library_id = ?", libraryID).Delete(&database.MediaFile{})
	if result.Error != nil {
		logger.Error("Failed to delete media files", "library_id", libraryID, "error", result.Error)
		return fmt.Errorf("failed to delete media files: %w", result.Error)
	}
	
	logger.Info("Cleanup completed for library", 
		"library_id", libraryID,
		"media_files_removed", result.RowsAffected,
		"assets_removed", totalAssetsRemoved,
		"asset_files_removed", totalAssetFilesRemoved)
	
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
	
	// Clean up assets for each media file discovered by this job
	totalAssetsRemoved := 0
	totalAssetFilesRemoved := 0
	
	for _, mediaFile := range mediaFiles {
		assetsRemoved, filesRemoved, err := c.cleanupMediaFileAssets(mediaFile.ID)
		if err != nil {
			logger.Error("Failed to cleanup assets for media file", "media_file_id", mediaFile.ID, "error", err)
			// Continue with other files
		}
		totalAssetsRemoved += assetsRemoved
		totalAssetFilesRemoved += filesRemoved
	}
	
	// Remove media files discovered by this scan job
	result := c.db.Where("scan_job_id = ?", scanJobID).Delete(&database.MediaFile{})
	if result.Error != nil {
		logger.Error("Failed to delete media files for scan job", "scan_job_id", scanJobID, "error", result.Error)
		return fmt.Errorf("failed to delete media files: %w", result.Error)
	}
	
	logger.Info("Scan job cleanup completed", 
		"scan_job_id", scanJobID, 
		"library_id", scanJob.LibraryID,
		"media_files_removed", result.RowsAffected,
		"assets_removed", totalAssetsRemoved,
		"asset_files_removed", totalAssetFilesRemoved)
	
	return nil
}

// cleanupMediaFileAssets removes all assets for a specific media file
func (c *CleanupService) cleanupMediaFileAssets(mediaFileID uint) (int, int, error) {
	// Get all assets for this media file
	var assets []mediaassetmodule.MediaAsset
	if err := c.db.Where("media_file_id = ?", mediaFileID).Find(&assets).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to find assets for media file %d: %w", mediaFileID, err)
	}
	
	if len(assets) == 0 {
		return 0, 0, nil // No assets to clean up
	}
	
	logger.Debug("Found assets to clean up", "media_file_id", mediaFileID, "count", len(assets))
	
	// Remove asset files from disk
	filesRemoved := 0
	for _, asset := range assets {
		if err := c.removeAssetFile(asset.RelativePath); err != nil {
			logger.Warn("Failed to remove asset file", "path", asset.RelativePath, "error", err)
			// Continue with other files
		} else {
			filesRemoved++
		}
	}
	
	// Remove asset records from database
	result := c.db.Where("media_file_id = ?", mediaFileID).Delete(&mediaassetmodule.MediaAsset{})
	if result.Error != nil {
		return 0, filesRemoved, fmt.Errorf("failed to delete asset records: %w", result.Error)
	}
	
	return int(result.RowsAffected), filesRemoved, nil
}

// removeAssetFile removes an asset file from disk
func (c *CleanupService) removeAssetFile(relativePath string) error {
	if relativePath == "" {
		return fmt.Errorf("empty relative path")
	}
	
	// Construct full path - assets are typically stored in a configured assets directory
	// We need to get the assets base path from configuration
	assetsBasePath := c.getAssetsBasePath()
	fullPath := filepath.Join(assetsBasePath, relativePath)
	
	// Safety check - make sure we're not deleting files outside the assets directory
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, assetsBasePath) {
		return fmt.Errorf("path traversal attempt detected: %s", relativePath)
	}
	
	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		logger.Debug("Asset file already deleted", "path", cleanPath)
		return nil // File doesn't exist, that's fine
	}
	
	// Remove the file
	if err := os.Remove(cleanPath); err != nil {
		return fmt.Errorf("failed to remove file %s: %w", cleanPath, err)
	}
	
	logger.Debug("Removed asset file", "path", cleanPath)
	
	// Try to remove empty parent directories
	c.removeEmptyDirs(filepath.Dir(cleanPath), assetsBasePath)
	
	return nil
}

// getAssetsBasePath returns the base path for asset storage
func (c *CleanupService) getAssetsBasePath() string {
	// TODO: Get this from configuration
	// For now, use a default path relative to the current working directory
	return "./data/assets"
}

// removeEmptyDirs removes empty directories up to the base path
func (c *CleanupService) removeEmptyDirs(dirPath, basePath string) {
	if dirPath == basePath || dirPath == "." || dirPath == "/" {
		return
	}
	
	// Check if directory is empty
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return // Can't read directory, skip
	}
	
	if len(entries) == 0 {
		// Directory is empty, try to remove it
		if err := os.Remove(dirPath); err == nil {
			logger.Debug("Removed empty directory", "path", dirPath)
			// Recursively try to remove parent directories
			c.removeEmptyDirs(filepath.Dir(dirPath), basePath)
		}
	}
}

// CleanupOrphanedAssets removes assets that reference non-existent media files
func (c *CleanupService) CleanupOrphanedAssets() (int, int, error) {
	logger.Info("Starting cleanup of orphaned assets")
	
	// Find assets that reference non-existent media files
	var orphanedAssets []mediaassetmodule.MediaAsset
	err := c.db.Raw(`
		SELECT ma.* FROM media_assets ma 
		LEFT JOIN media_files mf ON ma.media_file_id = mf.id 
		WHERE mf.id IS NULL
	`).Scan(&orphanedAssets).Error
	
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find orphaned assets: %w", err)
	}
	
	if len(orphanedAssets) == 0 {
		logger.Info("No orphaned assets found")
		return 0, 0, nil
	}
	
	logger.Info("Found orphaned assets", "count", len(orphanedAssets))
	
	// Remove orphaned asset files from disk
	filesRemoved := 0
	for _, asset := range orphanedAssets {
		if err := c.removeAssetFile(asset.RelativePath); err != nil {
			logger.Warn("Failed to remove orphaned asset file", "path", asset.RelativePath, "error", err)
		} else {
			filesRemoved++
		}
	}
	
	// Remove orphaned asset records from database
	var orphanedAssetIDs []uint
	for _, asset := range orphanedAssets {
		orphanedAssetIDs = append(orphanedAssetIDs, asset.ID)
	}
	
	result := c.db.Where("id IN ?", orphanedAssetIDs).Delete(&mediaassetmodule.MediaAsset{})
	if result.Error != nil {
		return 0, filesRemoved, fmt.Errorf("failed to delete orphaned asset records: %w", result.Error)
	}
	
	logger.Info("Orphaned assets cleanup completed", 
		"records_removed", result.RowsAffected, 
		"files_removed", filesRemoved)
	
	return int(result.RowsAffected), filesRemoved, nil
} 