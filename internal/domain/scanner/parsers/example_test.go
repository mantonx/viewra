package parsers_test

import (
	"fmt"

	"github.com/viewra/viewra/internal/domain/scanner/parsers"
)

// Example demonstrates how to use the movie parser
func ExampleParseMovie() {
	info, err := parsers.ParseMovie("Inception.2010.1080p.BluRay.mkv")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Title: %s\n", info.Title)
	fmt.Printf("Year: %d\n", info.Year)
	fmt.Printf("Resolution: %s\n", info.Resolution)
	fmt.Printf("Quality: %s\n", info.Quality)

	// Output:
	// Title: Inception
	// Year: 2010
	// Resolution: 1080p
	// Quality: BluRay
}

// Example demonstrates how to use the TV show parser
func ExampleParseTVEpisode() {
	info, err := parsers.ParseTVEpisode("Breaking.Bad.S01E01.Pilot.mkv")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Show: %s\n", info.ShowName)
	fmt.Printf("Season: %d\n", info.Season)
	fmt.Printf("Episode: %d\n", info.Episode)
	fmt.Printf("Title: %s\n", info.EpisodeTitle)

	// Output:
	// Show: Breaking Bad
	// Season: 1
	// Episode: 1
	// Title: Pilot
}

// Example demonstrates how to use the default parser interface
func ExampleDefaultParser() {
	parser := parsers.NewDefaultParser()

	// Parse a movie
	movieInfo, _ := parser.ParseMovie("The Matrix (1999).mkv")
	fmt.Printf("Movie: %s (%d)\n", movieInfo.Title, movieInfo.Year)

	// Parse a TV episode
	tvInfo, _ := parser.ParseTVEpisode("Friends.S01E01.mkv")
	fmt.Printf("TV: %s S%02dE%02d\n", tvInfo.ShowName, tvInfo.Season, tvInfo.Episode)

	// Output:
	// Movie: The Matrix (1999)
	// TV: Friends S01E01
}
