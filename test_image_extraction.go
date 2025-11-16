package main

import (
	"fmt"
	"os"

	"github.com/viewra/viewra/internal/infrastructure/images"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_image_extraction.go <movie-file-path>")
		os.Exit(1)
	}

	moviePath := os.Args[1]
	extractor := images.NewExtractor()

	fmt.Printf("Testing image extraction for: %s\n", moviePath)

	extracted, err := extractor.ExtractMovieImages(moviePath)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d images:\n", len(extracted.Images))
	for i, img := range extracted.Images {
		fmt.Printf("%d. Type: %s, Path: %s\n", i+1, img.Type, img.Path)
	}
}
