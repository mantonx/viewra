package main

import (
	"fmt"
	"os"

	"github.com/viewra/viewra/internal/infrastructure/images"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_metadata.go <image-file-path>")
		os.Exit(1)
	}

	imagePath := os.Args[1]
	extractor := images.NewMetadataExtractor()

	fmt.Printf("Testing metadata extraction for: %s\n", imagePath)

	metadata, err := extractor.ExtractMetadata(imagePath)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMetadata:\n")
	fmt.Printf("  Width: %d\n", metadata.Width)
	fmt.Printf("  Height: %d\n", metadata.Height)
	fmt.Printf("  FileSize: %d bytes\n", metadata.FileSizeBytes)
	fmt.Printf("  MimeType: %s\n", metadata.MimeType)
	fmt.Printf("  FileHash: %s\n", metadata.FileHash)
}
