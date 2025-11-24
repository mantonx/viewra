package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	appImages "github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/app/config"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	infraImages "github.com/mantonx/viewra/internal/infrastructure/images"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/image"
)

func main() {
	ctx := context.Background()

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := persistence.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create repositories
	imageRepo := persistence.NewImageRepository(db)

	// Create image services
	cacheService := infraImages.NewCacheService("data/cache/images")
	transformer := infraImages.NewTransformer()

	// Create use case
	extractUseCase := images.NewExtractTVShowImagesUseCase(imageRepo, cacheService, transformer)

	// Test extracting images for The Acolyte
	showDir := "/cifs/fictionalserver/tv/The Acolyte (2024)"
	showID := 588 // The Acolyte's ID in the database

	fmt.Printf("\n=== Testing Image Extraction and Save for The Acolyte ===\n")
	fmt.Printf("Show Directory: %s\n", showDir)
	fmt.Printf("Show Entity ID: %d\n\n", showID)

	// Execute extraction
	err = extractUseCase.Execute(ctx, showDir, domainImages.MediaTypeTVShow, showID)
	if err != nil {
		log.Fatalf("ERROR: Failed to extract and save images: %v", err)
	}

	fmt.Println("\n✓ Image extraction completed successfully!")

	// Verify images were saved to database
	fmt.Println("\n=== Verifying Images in Database ===")
	savedImages, err := imageRepo.GetByEntity(ctx, domainImages.MediaTypeTVShow, showID)
	if err != nil {
		log.Fatalf("ERROR: Failed to query saved images: %v", err)
	}

	fmt.Printf("Found %d images in database:\n", len(savedImages))
	for i, img := range savedImages {
		fmt.Printf("  %d. Type: %-15s Source: %-10s Path: %s\n",
			i+1, img.ImageType, img.SourceType,
			func() string {
				if img.FilePath != nil {
					return *img.FilePath
				}
				return "(no path)"
			}())
	}
}
