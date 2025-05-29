package main

import (
	"flag"
	"log"
	"os"

	"github.com/mantonx/viewra/internal/database"
)

func main() {
	var artworkCleanup = flag.Bool("artwork-cleanup", false, "Run artwork cleanup migration")
	flag.Parse()

	// Initialize database
	database.Initialize()

	if *artworkCleanup {
		log.Println("Running artwork cleanup migration...")
		if err := RunArtworkCleanupMigration(); err != nil {
			log.Fatalf("Artwork cleanup migration failed: %v", err)
		}
		log.Println("Artwork cleanup migration completed successfully!")
		os.Exit(0)
	}

	log.Println("No migration specified. Use -artwork-cleanup to run artwork cleanup.")
	flag.Usage()
	os.Exit(1)
} 