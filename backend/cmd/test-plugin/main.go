package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mantonx/viewra/internal/plugins"
	"gorm.io/gorm"
)

// Simple logger implementation for testing
type TestLogger struct{}

func (l *TestLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

func (l *TestLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}

func (l *TestLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] %s %v", msg, fields)
}

func (l *TestLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

// Simple database wrapper for testing
type TestDatabase struct{}

func (d *TestDatabase) GetDB() interface{} {
	// Return a mock GORM DB for testing
	// In a real test, you'd use an in-memory SQLite database
	// For now, we'll return nil and modify the plugin manager to handle it
	return (*gorm.DB)(nil)
}

func main() {
	fmt.Println("Testing MusicBrainz Enricher Plugin Discovery and Registration")
	fmt.Println("=============================================================")

	// Get the plugin directory
	pluginDir := filepath.Join("data", "plugins")
	
	// Create plugin manager
	logger := &TestLogger{}
	db := &TestDatabase{}
	manager := plugins.NewManager(db, pluginDir, logger)

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize plugin manager (this will discover plugins)
	if err := manager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin manager: %v", err)
	}

	// List discovered plugins
	fmt.Println("\nDiscovered Plugins:")
	fmt.Println("------------------")
	pluginList := manager.ListPlugins()
	for _, info := range pluginList {
		fmt.Printf("- ID: %s\n", info.ID)
		fmt.Printf("  Name: %s\n", info.Name)
		fmt.Printf("  Version: %s\n", info.Version)
		fmt.Printf("  Type: %s\n", info.Type)
		fmt.Printf("  Status: %s\n", info.Status)
		fmt.Printf("  Path: %s\n", info.InstallPath)
		fmt.Println()
	}

	// Check if MusicBrainz enricher was discovered
	mbInfo, exists := manager.GetPluginInfo("musicbrainz_enricher")
	if !exists {
		fmt.Println("❌ MusicBrainz Enricher plugin not discovered")
		fmt.Println("\nMake sure the plugin.yml file exists in data/plugins/musicbrainz_enricher/")
		os.Exit(1)
	}

	fmt.Printf("✅ MusicBrainz Enricher plugin discovered: %s v%s\n", mbInfo.Name, mbInfo.Version)

	// Try to load the plugin
	fmt.Println("\nAttempting to load MusicBrainz Enricher plugin...")
	if err := manager.LoadPlugin(ctx, "musicbrainz_enricher"); err != nil {
		log.Printf("Failed to load plugin: %v", err)
		fmt.Println("❌ Plugin loading failed")
	} else {
		fmt.Println("✅ Plugin loaded successfully")
		
		// Get the loaded plugin
		plugin, exists := manager.GetPlugin("musicbrainz_enricher")
		if exists {
			fmt.Printf("Plugin instance: %T\n", plugin)
			
			// Check health
			if err := plugin.Health(); err != nil {
				fmt.Printf("Plugin health check failed: %v\n", err)
			} else {
				fmt.Println("✅ Plugin health check passed")
			}
		}
	}

	// Get scanner hook plugins
	scannerHooks := manager.GetScannerHookPlugins()
	fmt.Printf("\nScanner Hook Plugins: %d\n", len(scannerHooks))

	// Get metadata scraper plugins
	metadataScrapers := manager.GetMetadataScrapers()
	fmt.Printf("Metadata Scraper Plugins: %d\n", len(metadataScrapers))

	// Test scanner hook functionality (without starting the plugin)
	if len(scannerHooks) > 0 {
		fmt.Println("\nTesting scanner hook functionality...")
		for _, hook := range scannerHooks {
			fmt.Printf("Testing hook plugin: %s\n", hook.Info().ID)
			
			// Test OnMediaFileScanned
			if err := hook.OnMediaFileScanned(123, "/test/file.mp3", map[string]interface{}{
				"title":  "Test Song",
				"artist": "Test Artist",
				"album":  "Test Album",
			}); err != nil {
				fmt.Printf("❌ OnMediaFileScanned failed: %v\n", err)
			} else {
				fmt.Println("✅ OnMediaFileScanned test passed")
			}
		}
	}

	// Test metadata scraper functionality
	if len(metadataScrapers) > 0 {
		fmt.Println("\nTesting metadata scraper functionality...")
		for _, scraper := range metadataScrapers {
			fmt.Printf("Testing scraper plugin: %s\n", scraper.Info().ID)
			
			// Test CanHandle
			canHandle := scraper.CanHandle("/test/file.mp3", "audio/mpeg")
			fmt.Printf("Can handle MP3 files: %v\n", canHandle)
			
			// Test SupportedTypes
			supportedTypes := scraper.SupportedTypes()
			fmt.Printf("Supported types: %v\n", supportedTypes)
		}
	}

	// Shutdown (this should complete quickly now)
	fmt.Println("\nShutting down plugin manager...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	if err := manager.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("✅ Plugin system test completed successfully!")
} 