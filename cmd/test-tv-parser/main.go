package main

import (
	"fmt"
	"os"

	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-tv-parser <filename>")
		os.Exit(1)
	}

	filename := os.Args[1]
	parser := parsers.NewDefaultParser()
	info, err := parser.ParseTVEpisode(filename)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Show Name: '%s'\n", info.ShowName)
	fmt.Printf("Season: %d\n", info.Season)
	fmt.Printf("Episode: %d\n", info.Episode)
	fmt.Printf("Episode Title: '%s'\n", info.EpisodeTitle)
}
