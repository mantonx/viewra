package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
)

func main() {
	fmt.Println("🎵 MusicBrainz Enrichment Demo")
	fmt.Println("=============================")
	
	// Initialize database
	database.Initialize()
	
	// Create database wrapper and plugin manager
	dbWrapper := &testDatabase{}
	logger := &testLogger{}
	manager := plugins.NewManager(dbWrapper, "data/plugins", logger)

	// Initialize plugin manager
	ctx := context.Background()
	if err := manager.Initialize(ctx); err != nil {
		log.Fatal("Failed to initialize plugin manager:", err)
	}

	// Enable MusicBrainz plugin
	if err := manager.EnablePlugin(ctx, "musicbrainz_enricher"); err != nil {
		log.Printf("Failed to enable MusicBrainz plugin: %v", err)
		return
	}

	// Get the plugin
	plugin, exists := manager.GetPlugin("musicbrainz_enricher")
	if !exists {
		log.Fatal("MusicBrainz plugin not found")
	}

	// Check if it implements ScannerHookPlugin
	hookPlugin, ok := plugin.(plugins.ScannerHookPlugin)
	if !ok {
		log.Fatal("Plugin doesn't implement ScannerHookPlugin")
	}

	// Get all music files with metadata
	db := database.GetDB()
	var musicFiles []struct {
		MediaFileID uint   `db:"media_file_id"`
		Title       string `db:"title"`
		Artist      string `db:"artist"`
		Album       string `db:"album"`
		Path        string `db:"path"`
	}
	
	query := `
		SELECT mm.media_file_id, mm.title, mm.artist, mm.album, mf.path 
		FROM music_metadata mm 
		JOIN media_files mf ON mm.media_file_id = mf.id 
		ORDER BY mm.media_file_id
	`
	
	rows, err := db.Raw(query).Rows()
	if err != nil {
		log.Fatal("Failed to query music files:", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var file struct {
			MediaFileID uint   `db:"media_file_id"`
			Title       string `db:"title"`
			Artist      string `db:"artist"`
			Album       string `db:"album"`
			Path        string `db:"path"`
		}
		
		if err := rows.Scan(&file.MediaFileID, &file.Title, &file.Artist, &file.Album, &file.Path); err != nil {
			continue
		}
		musicFiles = append(musicFiles, file)
	}
	
	fmt.Printf("\n📀 Found %d music files to enrich:\n", len(musicFiles))
	for i, file := range musicFiles {
		fmt.Printf("  %d. \"%s\" by %s (ID: %d)\n", i+1, file.Title, file.Artist, file.MediaFileID)
	}
	
	fmt.Println("\n🔍 Starting enrichment process...")
	fmt.Println("=" + fmt.Sprintf("%*s", 50, "="))
	
	// Process each file
	for i, file := range musicFiles {
		fmt.Printf("\n[%d/%d] Processing: \"%s\" by %s\n", i+1, len(musicFiles), file.Title, file.Artist)
		fmt.Printf("      File ID: %d\n", file.MediaFileID)
		
		// Check if already enriched
		var existingCount int64
		db.Raw("SELECT COUNT(*) FROM music_brainz_enrichments WHERE media_file_id = ?", file.MediaFileID).Scan(&existingCount)
		
		if existingCount > 0 {
			fmt.Printf("      ✅ Already enriched - fetching existing data...\n")
			
			// Show existing enrichment
			var enrichment struct {
				EnrichedTitle   string  `db:"enriched_title"`
				EnrichedArtist  string  `db:"enriched_artist"`
				EnrichedAlbum   string  `db:"enriched_album"`
				EnrichedYear    int     `db:"enriched_year"`
				MatchScore      float64 `db:"match_score"`
				MBRecordingID   string  `db:"musicbrainz_recording_id"`
			}
			
			query := "SELECT enriched_title, enriched_artist, enriched_album, enriched_year, match_score, musicbrainz_recording_id FROM music_brainz_enrichments WHERE media_file_id = ?"
			row := db.Raw(query, file.MediaFileID).Row()
			
			if err := row.Scan(&enrichment.EnrichedTitle, &enrichment.EnrichedArtist, &enrichment.EnrichedAlbum, &enrichment.EnrichedYear, &enrichment.MatchScore, &enrichment.MBRecordingID); err == nil {
				fmt.Printf("      📊 Match Score: %.0f%%\n", enrichment.MatchScore)
				fmt.Printf("      🎵 Title: %s\n", enrichment.EnrichedTitle)
				fmt.Printf("      👤 Artist: %s\n", enrichment.EnrichedArtist)
				fmt.Printf("      💿 Album: %s (%d)\n", enrichment.EnrichedAlbum, enrichment.EnrichedYear)
				fmt.Printf("      🆔 MusicBrainz ID: %s\n", enrichment.MBRecordingID)
			}
		} else {
			fmt.Printf("      🔄 Enriching with MusicBrainz...\n")
			
			// Trigger enrichment
			startTime := time.Now()
			err := hookPlugin.OnMediaFileScanned(file.MediaFileID, file.Path, map[string]interface{}{})
			if err != nil {
				fmt.Printf("      ❌ Error: %v\n", err)
				continue
			}
			
			// Wait for enrichment to complete
			time.Sleep(3 * time.Second)
			
			// Check if enrichment was created
			var enrichment struct {
				EnrichedTitle   string  `db:"enriched_title"`
				EnrichedArtist  string  `db:"enriched_artist"`
				EnrichedAlbum   string  `db:"enriched_album"`
				EnrichedYear    int     `db:"enriched_year"`
				MatchScore      float64 `db:"match_score"`
				MBRecordingID   string  `db:"musicbrainz_recording_id"`
			}
			
			query := "SELECT enriched_title, enriched_artist, enriched_album, enriched_year, match_score, musicbrainz_recording_id FROM music_brainz_enrichments WHERE media_file_id = ?"
			row := db.Raw(query, file.MediaFileID).Row()
			
			err = row.Scan(&enrichment.EnrichedTitle, &enrichment.EnrichedArtist, &enrichment.EnrichedAlbum, &enrichment.EnrichedYear, &enrichment.MatchScore, &enrichment.MBRecordingID)
			if err != nil {
				fmt.Printf("      ❌ No enrichment found (API may be unavailable)\n")
			} else {
				duration := time.Since(startTime)
				fmt.Printf("      ✅ Enriched in %v\n", duration.Round(time.Millisecond))
				fmt.Printf("      📊 Match Score: %.0f%%\n", enrichment.MatchScore)
				fmt.Printf("      🎵 Title: %s\n", enrichment.EnrichedTitle)
				fmt.Printf("      👤 Artist: %s\n", enrichment.EnrichedArtist)
				fmt.Printf("      💿 Album: %s (%d)\n", enrichment.EnrichedAlbum, enrichment.EnrichedYear)
				fmt.Printf("      🆔 MusicBrainz ID: %s\n", enrichment.MBRecordingID)
			}
		}
		
		// Add a small delay between files to respect rate limits
		if i < len(musicFiles)-1 {
			fmt.Printf("      ⏳ Waiting 2s (rate limiting)...\n")
			time.Sleep(2 * time.Second)
		}
	}
	
	// Show final statistics
	fmt.Println("\n" + "=" + fmt.Sprintf("%*s", 50, "="))
	fmt.Println("📈 Final Statistics")
	fmt.Println("=" + fmt.Sprintf("%*s", 50, "="))
	
	var totalEnrichments int64
	var avgMatchScore float64
	
	db.Raw("SELECT COUNT(*) FROM music_brainz_enrichments").Scan(&totalEnrichments)
	db.Raw("SELECT AVG(match_score) FROM music_brainz_enrichments").Scan(&avgMatchScore)
	
	fmt.Printf("🎯 Total Enrichments: %d\n", totalEnrichments)
	fmt.Printf("📊 Average Match Score: %.1f%%\n", avgMatchScore)
	
	// Show cache statistics
	var cacheEntries int64
	db.Raw("SELECT COUNT(*) FROM music_brainz_caches").Scan(&cacheEntries)
	fmt.Printf("💾 Cache Entries: %d\n", cacheEntries)
	
	fmt.Println("\n🎉 MusicBrainz enrichment demo completed!")
}

type testDatabase struct{}

func (d *testDatabase) GetDB() interface{} {
	return database.GetDB()
}

type testLogger struct{}

func (l *testLogger) Info(msg string, args ...interface{}) {
	// Suppress plugin manager logs for cleaner demo output
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("ERROR: %s %v\n", msg, args)
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	// Suppress debug logs for cleaner demo output
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("WARN: %s %v\n", msg, args)
} 