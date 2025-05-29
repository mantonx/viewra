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
	fmt.Println("Debugging FileStore file saving via Manager...")

	// Initialize database
	database.Initialize()

	// Initialize modules (including media asset module)
	db := database.GetDB()
	if err := modulemanager.LoadAll(db); err != nil {
		log.Fatalf("Failed to initialize modules: %v", err)
	}

	// Get the asset manager
	manager := mediaassetmodule.GetAssetManager()
	if manager == nil {
		log.Fatalf("Asset manager is nil")
	}

	fmt.Println("✓ Asset manager retrieved successfully")

	// Test data - simulate what a real music file would send
	testData := []byte("fake album artwork data")

	// Create asset request like the music plugin would
	request := &mediaassetmodule.AssetRequest{
		MediaFileID: 999999, // Fake media file ID
		Type:        mediaassetmodule.AssetTypeMusic,
		Category:    mediaassetmodule.CategoryAlbum,
		Subtype:     mediaassetmodule.SubtypeArtwork,
		Data:        testData,
		MimeType:    "image/jpeg",
	}

	fmt.Printf("Saving asset via Manager:\n")
	fmt.Printf("  MediaFileID: %d\n", request.MediaFileID)
	fmt.Printf("  Type: %s\n", request.Type)
	fmt.Printf("  Category: %s\n", request.Category)
	fmt.Printf("  Subtype: %s\n", request.Subtype)
	fmt.Printf("  Data size: %d bytes\n", len(request.Data))
	fmt.Printf("  MimeType: %s\n", request.MimeType)

	// Save using the Manager (like the scanner would)
	asset, err := manager.SaveAsset(request)
	if err != nil {
		log.Fatalf("Failed to save asset via Manager: %v", err)
	}

	fmt.Printf("✓ Asset saved successfully:\n")
	fmt.Printf("  ID: %d\n", asset.ID)
	fmt.Printf("  RelativePath: %s\n", asset.RelativePath)
	fmt.Printf("  Hash: %s\n", asset.Hash)
	fmt.Printf("  Size: %d\n", asset.Size)

	// Test if the file actually exists on disk
	pathUtil := mediaassetmodule.GetDefaultPathUtil()
	fullPath := pathUtil.GetFullPath(asset.RelativePath)
	
	if _, err := os.Stat(fullPath); err == nil {
		fmt.Printf("✓ File exists on disk: %s\n", fullPath)
		
		// Test reading via manager
		data, err := manager.GetAssetData(asset.ID)
		if err != nil {
			fmt.Printf("✗ Failed to read asset data via Manager: %v\n", err)
		} else {
			if string(data) == string(testData) {
				fmt.Println("✓ File content matches via Manager!")
			} else {
				fmt.Printf("✗ File content mismatch via Manager: expected %q, got %q\n", string(testData), string(data))
			}
		}
	} else {
		fmt.Printf("✗ File does not exist on disk: %s (error: %v)\n", fullPath, err)
	}

	// Test the asset serving through the database
	exists, assetResponse, err := manager.ExistsAsset(request.MediaFileID, request.Type, request.Category)
	if err != nil {
		fmt.Printf("✗ Error checking asset existence: %v\n", err)
	} else if exists {
		fmt.Printf("✓ Asset exists in database: %s\n", assetResponse.RelativePath)
	} else {
		fmt.Println("✗ Asset does not exist in database")
	}

	// Cleanup
	fmt.Println("Cleaning up test asset...")
	if err := manager.RemoveAsset(asset.ID); err != nil {
		fmt.Printf("WARNING: Failed to cleanup asset: %v\n", err)
	}
	
	fmt.Println("Debug complete!")
} 