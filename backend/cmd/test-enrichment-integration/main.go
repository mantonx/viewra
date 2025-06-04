package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/enrichmentmodule"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

func main() {
	fmt.Println("=== Enrichment Integration Test ===")

	// Initialize configuration
	if err := config.Load(""); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database.Initialize()
	db := database.GetDB()

	// Initialize event bus
	eventBus := events.GetGlobalEventBus()

	// Initialize enrichment module
	enrichmentModule := enrichmentmodule.NewModule(db, eventBus)
	if err := enrichmentModule.Init(); err != nil {
		log.Fatalf("Failed to initialize enrichment module: %v", err)
	}

	if err := enrichmentModule.Start(); err != nil {
		log.Fatalf("Failed to start enrichment module: %v", err)
	}
	defer enrichmentModule.Stop()

	fmt.Println("✅ Enrichment module started")

	// Initialize plugin module
	pluginConfig := &pluginmodule.PluginModuleConfig{
		PluginDir:       "./data/plugins",
		EnabledCore:     []string{},
		EnabledExternal: []string{},
		LibraryConfigs:  make(map[string]pluginmodule.LibraryPluginSettings),
	}

	pluginModule := pluginmodule.NewPluginModule(db, pluginConfig)
	ctx := context.Background()

	if err := pluginModule.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin module: %v", err)
	}
	defer pluginModule.Shutdown(ctx)

	// Connect enrichment module to plugin system
	enrichmentModule.SetExternalPluginManager(pluginModule.GetExternalManager())

	fmt.Println("✅ Plugin module initialized and connected")

	// List available external plugins
	plugins := pluginModule.ListAllPlugins()
	fmt.Printf("📋 Found %d plugins:\n", len(plugins))

	var musicbrainzPlugin, tmdbPlugin string
	for _, plugin := range plugins {
		fmt.Printf("  - %s (%s) - Enabled: %v\n", plugin.Name, plugin.ID, plugin.Enabled)
		if plugin.ID == "musicbrainz_enricher" {
			musicbrainzPlugin = plugin.ID
		}
		if plugin.ID == "tmdb_enricher" {
			tmdbPlugin = plugin.ID
		}
	}

	// Test loading external plugins
	if musicbrainzPlugin != "" {
		fmt.Printf("\n🎵 Testing MusicBrainz plugin...\n")
		if err := pluginModule.LoadExternalPlugin(ctx, musicbrainzPlugin); err != nil {
			fmt.Printf("❌ Failed to load MusicBrainz plugin: %v\n", err)
		} else {
			fmt.Printf("✅ MusicBrainz plugin loaded successfully\n")
			
			// Test notification
			testNotifyPlugin(pluginModule, "test-audio-file-123", "/test/path/song.mp3", map[string]string{
				"title":  "Test Song",
				"artist": "Test Artist",
				"album":  "Test Album",
			})
		}
	}

	if tmdbPlugin != "" {
		fmt.Printf("\n🎬 Testing TMDb plugin...\n")
		if err := pluginModule.LoadExternalPlugin(ctx, tmdbPlugin); err != nil {
			fmt.Printf("❌ Failed to load TMDb plugin: %v\n", err)
		} else {
			fmt.Printf("✅ TMDb plugin loaded successfully\n")
			
			// Test notification
			testNotifyPlugin(pluginModule, "test-movie-file-456", "/test/path/movie.mkv", map[string]string{
				"title": "The Matrix",
				"year":  "1999",
			})
		}
	}

	// Test enrichment module directly
	fmt.Printf("\n🔧 Testing enrichment module directly...\n")
	testEnrichmentData := map[string]interface{}{
		"title":  "Test Enriched Title",
		"artist": "Test Enriched Artist",
		"genre":  "Test Genre",
	}

	if err := enrichmentModule.RegisterEnrichmentData("direct-test-123", "test_source", testEnrichmentData, 0.95); err != nil {
		fmt.Printf("❌ Failed to register enrichment data: %v\n", err)
	} else {
		fmt.Printf("✅ Enrichment data registered successfully\n")
		
		// Check status
		if status, err := enrichmentModule.GetEnrichmentStatus("direct-test-123"); err != nil {
			fmt.Printf("❌ Failed to get enrichment status: %v\n", err)
		} else {
			fmt.Printf("✅ Enrichment status: %+v\n", status)
		}
	}

	fmt.Printf("\n⏱️  Waiting 5 seconds for background processing...\n")
	time.Sleep(5 * time.Second)

	fmt.Println("\n=== Integration Test Completed ===")
}

func testNotifyPlugin(pluginModule *pluginmodule.PluginModule, mediaFileID, filePath string, metadata map[string]string) {
	extMgr := pluginModule.GetExternalManager()
	if extMgr != nil {
		fmt.Printf("📢 Notifying plugins about file: %s\n", filePath)
		extMgr.NotifyMediaFileScanned(mediaFileID, filePath, metadata)
		time.Sleep(2 * time.Second) // Give plugins time to process
	}
} 