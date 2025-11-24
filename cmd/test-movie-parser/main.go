package main

import (
	"fmt"
	"os"

	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-movie-parser <filename>")
		os.Exit(1)
	}

	filename := os.Args[1]
	parser := parsers.NewDefaultParser()
	info, err := parser.ParseMovie(filename)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Title: '%s'\n", info.Title)
	if info.Year > 0 {
		fmt.Printf("Year: %d\n", info.Year)
	}
}
