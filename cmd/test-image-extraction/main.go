package main

import (
	"context"
	"fmt"
	"log"

	infraImages "github.com/mantonx/viewra/internal/infrastructure/images"
)

func main() {
	ctx := context.Background()

	// Test TV Show image extraction
	fmt.Println("=== Testing TV Show Image Extraction ===")
	testTVShowExtraction(ctx)

	fmt.Println("\n=== Testing Music Album Image Extraction ===")
	testMusicAlbumExtraction(ctx)
}

func testTVShowExtraction(ctx context.Context) {
	extractor := infraImages.NewExtractor()
	showDir := "/cifs/fictionalserver/tv/1883"

	extracted, err := extractor.ExtractTVShowImages(showDir)
	if err != nil {
		log.Printf("ERROR: Failed to extract TV show images: %v", err)
		return
	}

	fmt.Printf("Found %d images for TV show:\n", len(extracted.Images))
	for i, img := range extracted.Images {
		fmt.Printf("  %d. Type: %-15s Path: %s\n", i+1, img.Type, img.Path)
	}
}

func testMusicAlbumExtraction(ctx context.Context) {
	extractor := infraImages.NewExtractor()
	albumDir := "/cifs/fictionalserver/music/65daysofstatic/The Fall of Math (2004)[FLAC]"

	extracted, err := extractor.ExtractMusicAlbumImages(albumDir)
	if err != nil {
		log.Printf("ERROR: Failed to extract music album images: %v", err)
		return
	}

	fmt.Printf("Found %d images for music album:\n", len(extracted.Images))
	for i, img := range extracted.Images {
		fmt.Printf("  %d. Type: %-15s Path: %s\n", i+1, img.Type, img.Path)
	}
}
