// +build ignore

package main

import (
	"fmt"
	"log"

	"github.com/viewra/viewra/internal/infrastructure/metadata/nfo"
)

func main() {
	// Test movie NFO parsing
	fmt.Println("=== Testing Movie NFO Parsing ===")
	movieFile := "/cifs/fictionalserver/movies/Happy Gilmore (1996)/Happy Gilmore (1996) [imdbid-tt0116483] - [Bluray-2160p][DV HDR10][DTS-HD MA 5.1][x265]-B0MBARDiERS.mkv"
	
	nfoPath, err := nfo.FindMovieNFO(movieFile)
	if err != nil {
		log.Printf("Error finding movie NFO: %v", err)
	} else {
		fmt.Printf("Found NFO: %s\n", nfoPath)
		
		metadata, err := nfo.ParseMovieNFO(nfoPath)
		if err != nil {
			log.Printf("Error parsing movie NFO: %v", err)
		} else {
			fmt.Printf("Title: %s\n", metadata.Title)
			fmt.Printf("Year: %d\n", metadata.Year)
			fmt.Printf("Director: %s\n", metadata.Director)
			fmt.Printf("Plot: %s\n", metadata.Plot[:100]+"...")
			fmt.Printf("IMDb ID: %s\n", metadata.IMDbID)
			fmt.Printf("TMDb ID: %d\n", metadata.TMDbID)
			fmt.Printf("Genre: %s\n", metadata.Genre)
			fmt.Printf("Rating: %s\n", metadata.ContentRating)
		}
	}

	fmt.Println("\n=== Testing TV Episode NFO Parsing ===")
	episodeFile := "/cifs/fictionalserver/tv/Chicago P.D. (2014)/Season 01/Chicago P.D. (2014) - S01E01 - Stepping Stone [WEBRip-1080p][EAC3 5.1][x265]-iVy.mkv"
	
	nfoPath, err = nfo.FindEpisodeNFO(episodeFile)
	if err != nil {
		log.Printf("Error finding episode NFO: %v", err)
	} else {
		fmt.Printf("Found NFO: %s\n", nfoPath)
		
		metadata, err := nfo.ParseEpisodeNFO(nfoPath)
		if err != nil {
			log.Printf("Error parsing episode NFO: %v", err)
		} else {
			fmt.Printf("Title: %s\n", metadata.Title)
			fmt.Printf("Season: %d, Episode: %d\n", metadata.Season, metadata.Episode)
			fmt.Printf("Air Date: %s\n", metadata.AirDate.Format("2006-01-02"))
			fmt.Printf("Plot: %s\n", metadata.Plot[:100]+"...")
			fmt.Printf("TVDb ID: %d\n", metadata.TVDbID)
		}
	}
}
