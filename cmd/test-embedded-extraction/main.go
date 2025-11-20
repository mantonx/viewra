package main

import (
	"fmt"
	"log"

	"github.com/viewra/viewra/internal/infrastructure/images"
)

func main() {
	// Test embedded extraction
	extractor := images.NewEmbeddedExtractor()

	// Test album directory without folder.jpg
	albumDir := "/cifs/fictionalserver/music/A Perfect Circle/Eat the Elephant (2018)[FLAC 24bit]"
	fmt.Printf("Testing embedded album artwork extraction from: %s\n\n", albumDir)

	embeddedImg, err := extractor.ExtractAlbumArtFromFirstTrack(albumDir)
	if err != nil {
		log.Fatalf("Error extracting embedded artwork: %v", err)
	}

	if embeddedImg != nil {
		fmt.Printf("✅ Successfully extracted embedded album artwork!\n")
		fmt.Printf("   Temporary file: %s\n", embeddedImg.Path)
		fmt.Printf("   Image type: %s\n", embeddedImg.Type)
		fmt.Printf("   Priority: %d\n", embeddedImg.Priority)
	} else {
		fmt.Printf("❌ No embedded artwork found\n")
	}

	// Test artist directory
	artistDir := "/cifs/fictionalserver/music/A Perfect Circle"
	fmt.Printf("\nTesting embedded artist artwork extraction from: %s\n\n", artistDir)

	artistImg, err := extractor.ExtractArtistArtFromFirstTrack(artistDir)
	if err != nil {
		log.Fatalf("Error extracting embedded artist artwork: %v", err)
	}

	if artistImg != nil {
		fmt.Printf("✅ Successfully extracted embedded artist artwork!\n")
		fmt.Printf("   Temporary file: %s\n", artistImg.Path)
		fmt.Printf("   Image type: %s\n", artistImg.Type)
		fmt.Printf("   Priority: %d\n", artistImg.Priority)
	} else {
		fmt.Printf("❌ No embedded artwork found\n")
	}
}
