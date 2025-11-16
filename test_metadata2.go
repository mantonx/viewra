package main

import (
	"fmt"
	"os"

	"github.com/viewra/viewra/internal/infrastructure/images"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_metadata2.go <image-file-path>")
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
	if metadata.Width != nil {
		w := *metadata.Width
		fmt.Printf("  Width: %d\n", w)
	}
	if metadata.Height != nil {
		h := *metadata.Height
		fmt.Printf("  Height: %d\n", h)
	}
	if metadata.FileSizeBytes != nil {
		s := *metadata.FileSizeBytes
		fmt.Printf("  FileSize: %d bytes\n", s)
	}
	if metadata.MimeType != nil {
		m := *metadata.MimeType
		fmt.Printf("  MimeType: %s\n", m)
	}
	if metadata.FileHash != nil {
		fh := *metadata.FileHash
		fmt.Printf("  FileHash: %s\n", fh)
	}
}
