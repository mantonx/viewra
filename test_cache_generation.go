package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/viewra/viewra/internal/application/images"
	domainImages "github.com/viewra/viewra/internal/domain/images"
	infraImages "github.com/viewra/viewra/internal/infrastructure/images"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
	imageRepo "github.com/viewra/viewra/internal/infrastructure/persistence/image"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("=== Image Cache Generation Test ===\n")

	// Find a movie file with associated images
	testMovieDir := "/tmp/test-library"

	// Create test movie directory with a sample image
	os.MkdirAll(testMovieDir, 0755)
	moviePath := filepath.Join(testMovieDir, "Test_Movie_(2020).mkv")

	// Create dummy movie file
	f, _ := os.Create(moviePath)
	f.WriteString("dummy movie content")
	f.Close()

	// Create a simple test poster image (1x1 red pixel PNG)
	posterPath := filepath.Join(testMovieDir, "Test_Movie_(2020)-poster.png")
	posterData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	os.WriteFile(posterPath, posterData, 0644)

	fmt.Printf("✓ Test movie created: %s\n", moviePath)
	fmt.Printf("✓ Test poster created: %s\n", posterPath)

	// Setup database
	db, err := sql.Open("sqlite3", "./data/viewra.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Setup dependencies
	baseRepo := common.NewBaseRepository(db, "sqlite3")
	imageRepository := imageRepo.NewRepository(baseRepo)

	cacheDir := "./data/cache/images"
	os.MkdirAll(cacheDir, 0755)
	cacheService := infraImages.NewCacheService(cacheDir)

	// Create extractor use case
	extractMovieImages := images.NewExtractMovieImagesUseCase(imageRepository, cacheService)

	// Execute extraction
	fmt.Printf("\n🔍 Extracting images for: %s\n", moviePath)
	err = extractMovieImages.Execute(context.Background(), moviePath, domainImages.MediaTypeMovie, 999, nil)
	if err != nil {
		log.Fatalf("❌ Failed to extract images: %v", err)
	}

	// Check results
	fmt.Println("\n=== Results ===")

	// Check database
	var totalImages, cachedImages int
	db.QueryRow("SELECT COUNT(*), COUNT(local_cache_path) FROM media_images WHERE entity_id = 999").Scan(&totalImages, &cachedImages)
	fmt.Printf("📊 Database: %d total images, %d with cache paths\n", totalImages, cachedImages)

	// Check cache directory
	entries, _ := os.ReadDir(cacheDir)
	fmt.Printf("📁 Cache directory: %d files\n", len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		fmt.Printf("   - %s (%d bytes)\n", entry.Name(), info.Size())
	}

	// Show image records
	rows, _ := db.Query("SELECT id, image_type, file_path, local_cache_path FROM media_images WHERE entity_id = 999")
	defer rows.Close()
	fmt.Println("\n📝 Image records:")
	for rows.Next() {
		var id int
		var imageType, filePath string
		var cachePath *string
		rows.Scan(&id, &imageType, &filePath, &cachePath)
		cacheStr := "❌ nil"
		if cachePath != nil {
			cacheStr = "✅ " + *cachePath
		}
		fmt.Printf("   ID=%d Type=%s\n   Path=%s\n   Cache=%s\n\n", id, imageType, filePath, cacheStr)
	}

	if cachedImages > 0 {
		fmt.Println("✅ Test PASSED! Image cache is being populated during scan.")
	} else {
		fmt.Println("❌ Test FAILED! No images were cached.")
	}
}
