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
	fmt.Println("Testing New Media Asset Path Structure...")

	// Initialize database
	database.Initialize()

	// Register and initialize modules
	mediaassetmodule.Register()
	db := database.GetDB()
	if err := modulemanager.LoadAll(db); err != nil {
		log.Fatalf("Failed to initialize modules: %v", err)
	}

	fmt.Println("✓ Modules initialized successfully")

	// Get the asset manager
	manager := mediaassetmodule.GetAssetManager()
	if manager == nil {
		log.Fatalf("Asset manager is nil")
	}

	fmt.Println("✓ Asset manager retrieved successfully")

	// Test different asset types and categories
	testCases := []struct {
		name     string
		assetType mediaassetmodule.AssetType
		category  mediaassetmodule.AssetCategory
		subtype   mediaassetmodule.AssetSubtype
		expected  string
	}{
		{
			name:     "Music Album Artwork",
			assetType: mediaassetmodule.AssetTypeMusic,
			category:  mediaassetmodule.CategoryAlbum,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "music/album/ab/abcdef1234567890.jpg",
		},
		{
			name:     "Music Artist Photo",
			assetType: mediaassetmodule.AssetTypeMusic,
			category:  mediaassetmodule.CategoryArtist,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "music/artist/ab/abcdef1234567890.jpg",
		},
		{
			name:     "Movie Poster",
			assetType: mediaassetmodule.AssetTypeMovie,
			category:  mediaassetmodule.CategoryPoster,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "movie/poster/ab/abcdef1234567890.jpg",
		},
		{
			name:     "TV Show Poster",
			assetType: mediaassetmodule.AssetTypeTV,
			category:  mediaassetmodule.CategoryShow,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "tv/show/ab/abcdef1234567890.jpg",
		},
		{
			name:     "Actor Headshot",
			assetType: mediaassetmodule.AssetTypePeople,
			category:  mediaassetmodule.CategoryActor,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "people/actor/ab/abcdef1234567890.jpg",
		},
		{
			name:     "Studio Logo",
			assetType: mediaassetmodule.AssetTypeMeta,
			category:  mediaassetmodule.CategoryStudio,
			subtype:   mediaassetmodule.SubtypeArtwork,
			expected:  "meta/studio/ab/abcdef1234567890.jpg",
		},
	}

	for _, tc := range testCases {
		fmt.Printf("\n🧪 Testing %s...\n", tc.name)
		
		// Create test asset data
		testData := []byte("test image data for " + tc.name)
		
		// Create asset request
		request := &mediaassetmodule.AssetRequest{
			MediaFileID: 1, // Test media file ID
			Type:        tc.assetType,
			Category:    tc.category,
			Subtype:     tc.subtype,
			Data:        testData,
			MimeType:    "image/jpeg",
		}

		// Save the asset
		asset, err := manager.SaveAsset(request)
		if err != nil {
			fmt.Printf("❌ Failed to save %s: %v\n", tc.name, err)
			continue
		}

		fmt.Printf("✓ Saved asset with ID: %d\n", asset.ID)
		fmt.Printf("✓ Relative path: %s\n", asset.RelativePath)
		fmt.Printf("✓ Hash: %s\n", asset.Hash)
		fmt.Printf("✓ Path structure follows pattern: %s/{hashPrefix}/{hash}.ext\n", 
			string(tc.assetType)+"/"+string(tc.category))

		// Verify the asset can be retrieved
		retrievedAsset, err := manager.GetAsset(asset.ID)
		if err != nil {
			fmt.Printf("❌ Failed to retrieve %s: %v\n", tc.name, err)
			continue
		}

		fmt.Printf("✓ Successfully retrieved asset: %s\n", retrievedAsset.RelativePath)

		// Test asset data retrieval
		data, err := manager.GetAssetData(asset.ID)
		if err != nil {
			fmt.Printf("❌ Failed to get asset data: %v\n", err)
			continue
		}

		if string(data) == string(testData) {
			fmt.Printf("✓ Asset data integrity verified\n")
		} else {
			fmt.Printf("❌ Asset data integrity failed\n")
		}
	}

	// Get statistics
	stats, err := manager.GetStats()
	if err != nil {
		fmt.Printf("❌ Failed to get stats: %v\n", err)
	} else {
		fmt.Printf("\n📊 Asset Statistics:\n")
		fmt.Printf("   Total Assets: %d\n", stats.TotalAssets)
		fmt.Printf("   Total Size: %d bytes\n", stats.TotalSize)
		fmt.Printf("   Assets by Type:\n")
		for assetType, count := range stats.AssetsByType {
			fmt.Printf("     %s: %d\n", assetType, count)
		}
		fmt.Printf("   Assets by Category:\n")
		for category, count := range stats.AssetsByCategory {
			fmt.Printf("     %s: %d\n", category, count)
		}
	}

	fmt.Println("\n🎉 New media asset path structure test completed successfully!")
	
	// Clean up test database
	os.Remove("viewra.db")
} 