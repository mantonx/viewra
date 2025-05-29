package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
	"gorm.io/gorm"
)

// ArtworkCleanupMigration handles cleanup of corrupted artwork files
type ArtworkCleanupMigration struct {
	db *gorm.DB
}

// NewArtworkCleanupMigration creates a new artwork cleanup migration
func NewArtworkCleanupMigration() *ArtworkCleanupMigration {
	db := database.GetDB()
	return &ArtworkCleanupMigration{db: db}
}

// Run executes the artwork cleanup migration
func (m *ArtworkCleanupMigration) Run() error {
	log.Println("Starting artwork cleanup migration...")
	
	// Get all media files with music metadata
	var musicMetadata []database.MusicMetadata
	if err := m.db.Where("has_artwork = ?", true).Find(&musicMetadata).Error; err != nil {
		return fmt.Errorf("failed to fetch music metadata: %w", err)
	}
	
	log.Printf("Found %d music files with artwork to validate", len(musicMetadata))
	
	var (
		validCount    int
		invalidCount  int
		cleanedCount  int
		errorCount    int
	)

	// Get asset manager
	manager := mediaassetmodule.GetAssetManager()
	if manager == nil {
		log.Println("WARNING: Asset manager not available, running integrity validation on new system")
		return nil
	}
	
	for _, meta := range musicMetadata {
		log.Printf("Validating artwork for media file ID %d...", meta.MediaFileID)
		
		var exists bool
		// Check if artwork already exists in new system
		if manager != nil {
			var err error
			exists, _, err = manager.ExistsAsset(meta.MediaFileID, mediaassetmodule.AssetTypeMusic, mediaassetmodule.CategoryAlbum)
			if err != nil {
				fmt.Printf("WARNING: Failed to check asset existence for media file %d: %v\n", meta.MediaFileID, err)
				errorCount++
				continue
			}
		}

		if !exists {
			log.Printf("No artwork found for media file ID %d in new asset system", meta.MediaFileID)
			invalidCount++
			
			// Update has_artwork flag in database
			if updateErr := m.db.Model(&meta).Update("has_artwork", false).Error; updateErr != nil {
				log.Printf("ERROR: Failed to update has_artwork flag for media file ID %d: %v", meta.MediaFileID, updateErr)
				errorCount++
			} else {
				cleanedCount++
				log.Printf("Updated has_artwork flag for media file ID %d", meta.MediaFileID)
			}
		} else {
			log.Printf("Valid artwork found for media file ID %d", meta.MediaFileID)
			validCount++
		}
	}
	
	// Run integrity validation on the asset system
	if err := manager.ValidateAssetIntegrity(); err != nil {
		log.Printf("WARNING: Asset integrity validation failed: %v", err)
	}
	
	// Additional cleanup: remove orphaned artwork files from old system
	orphanedCount, err := m.cleanupOrphanedArtwork()
	if err != nil {
		log.Printf("WARNING: Failed to cleanup orphaned artwork: %v", err)
	}
	
	log.Printf("Artwork cleanup migration completed:")
	log.Printf("  Valid artwork files: %d", validCount)
	log.Printf("  Invalid artwork files: %d", invalidCount)
	log.Printf("  Cleaned up files: %d", cleanedCount)
	log.Printf("  Orphaned files removed: %d", orphanedCount)
	log.Printf("  Errors encountered: %d", errorCount)
	
	if errorCount > 0 {
		return fmt.Errorf("migration completed with %d errors", errorCount)
	}
	
	return nil
}

// cleanupOrphanedArtwork removes old artwork files that don't have corresponding media files
func (m *ArtworkCleanupMigration) cleanupOrphanedArtwork() (int, error) {
	// Determine old artwork directory
	cacheDir := "./data/artwork"
	if _, err := os.Stat("/app/data"); err == nil {
		cacheDir = "/app/data/artwork"
	}
	
	// Check if old artwork directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return 0, nil // No old artwork directory, nothing to clean
	}
	
	// Get all media file IDs from database
	var mediaFileIDs []uint
	if err := m.db.Model(&database.MediaFile{}).Pluck("id", &mediaFileIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to get media file IDs: %w", err)
	}
	
	// Create a map for fast lookup
	validIDs := make(map[uint]bool)
	for _, id := range mediaFileIDs {
		validIDs[id] = true
	}
	
	// Scan old artwork directory
	var orphanedCount int
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Parse media file ID from filename (old format: mediaFileID_hash_timestamp.ext)
		filename := info.Name()
		var mediaFileID uint
		if _, err := fmt.Sscanf(filename, "%d_", &mediaFileID); err != nil {
			log.Printf("WARNING: Could not parse media file ID from old artwork filename: %s", filename)
			return nil
		}
		
		// Check if media file exists
		if !validIDs[mediaFileID] {
			log.Printf("Removing orphaned old artwork file: %s (media file ID %d not found)", filename, mediaFileID)
			if removeErr := os.Remove(path); removeErr != nil {
				log.Printf("ERROR: Failed to remove orphaned artwork file %s: %v", path, removeErr)
			} else {
				orphanedCount++
			}
		}
		
		return nil
	})
	
	return orphanedCount, err
}

// RunArtworkCleanupMigration is a helper function to run the migration
func RunArtworkCleanupMigration() error {
	migration := NewArtworkCleanupMigration()
	return migration.Run()
} 