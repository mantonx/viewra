package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
)

func main() {
	fmt.Println("Clearing all media assets...")

	// Initialize database
	database.Initialize()

	// Initialize modules
	db := database.GetDB()
	if err := modulemanager.LoadAll(db); err != nil {
		log.Fatalf("Failed to initialize modules: %v", err)
	}

	// Get current stats
	manager := mediaassetmodule.GetAssetManager()
	if manager == nil {
		log.Fatalf("Asset manager is nil")
	}

	stats, err := manager.GetStats()
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Printf("Current assets: %d (Total size: %d bytes)\n", stats.TotalAssets, stats.TotalSize)

	// Clear all assets from database
	result := db.Exec("DELETE FROM media_assets")
	if result.Error != nil {
		log.Fatalf("Failed to clear assets: %v", result.Error)
	}

	fmt.Printf("Deleted %d asset records from database\n", result.RowsAffected)

	// Clear the artwork directory
	artworkPath := "./viewra-data/artwork"
	if err := os.RemoveAll(artworkPath); err != nil {
		log.Printf("Warning: Failed to remove artwork directory: %v", err)
	} else {
		fmt.Printf("Cleared artwork directory: %s\n", artworkPath)
	}

	// Recreate the artwork directory structure
	if err := os.MkdirAll(artworkPath, 0755); err != nil {
		log.Printf("Warning: Failed to recreate artwork directory: %v", err)
	} else {
		fmt.Printf("Recreated artwork directory\n")
	}

	// Verify cleanup
	finalStats, err := manager.GetStats()
	if err != nil {
		log.Printf("Warning: Failed to get final stats: %v", err)
	} else {
		fmt.Printf("Final assets: %d (Total size: %d bytes)\n", finalStats.TotalAssets, finalStats.TotalSize)
	}

	fmt.Println("✅ Asset cleanup complete! You can now restart the scan.")
} 