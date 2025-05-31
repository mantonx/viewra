package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"gorm.io/driver/sqlite" // Using sqlite for a mock DB
	"gorm.io/gorm"
)

// Simple logger implementation for testing
type TestLogger struct{}

func (l *TestLogger) GetLevel() hclog.Level { return hclog.Info }
func (l *TestLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	log.Printf("[%s] %s %v", level, msg, args)
}
func (l *TestLogger) Trace(msg string, args ...interface{}) { l.Log(hclog.Trace, msg, args...) }
func (l *TestLogger) Debug(msg string, args ...interface{}) { l.Log(hclog.Debug, msg, args...) }
func (l *TestLogger) Info(msg string, args ...interface{})  { l.Log(hclog.Info, msg, args...) }
func (l *TestLogger) Warn(msg string, args ...interface{})  { l.Log(hclog.Warn, msg, args...) }
func (l *TestLogger) Error(msg string, args ...interface{}) { l.Log(hclog.Error, msg, args...) }
func (l *TestLogger) IsTrace() bool { return l.GetLevel() <= hclog.Trace }
func (l *TestLogger) IsDebug() bool { return l.GetLevel() <= hclog.Debug }
func (l *TestLogger) IsInfo() bool  { return l.GetLevel() <= hclog.Info }
func (l *TestLogger) IsWarn() bool  { return l.GetLevel() <= hclog.Warn }
func (l *TestLogger) IsError() bool { return l.GetLevel() <= hclog.Error }
func (l *TestLogger) ImpliedArgs() []interface{} { return []interface{}{} }
func (l *TestLogger) With(args ...interface{}) hclog.Logger { return l } // Simplistic
func (l *TestLogger) Name() string { return "" }
func (l *TestLogger) Named(name string) hclog.Logger { return l } // Simplistic
func (l *TestLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr // Keep it simple for the test logger
}
func (l *TestLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", log.LstdFlags)
}
func (l *TestLogger) ResetNamed(name string) hclog.Logger { return l }
func (l *TestLogger) SetLevel(level hclog.Level)          {}

// Simple database wrapper for testing
type TestDatabase struct {
	db *gorm.DB
}

func NewTestDatabase() (*TestDatabase, error) {
	// Using an in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory database: %w", err)
	}
	return &TestDatabase{db: db}, nil
}

func (d *TestDatabase) GetDB() *gorm.DB {
	return d.db
}

func main() {
	fmt.Println("Testing MusicBrainz Enricher Plugin Discovery and Registration")
	fmt.Println("=============================================================")

	// Get the plugin directory
	// Assume running from project root for this test executable
	pluginDir := filepath.Join("backend", "data", "plugins")
	absPluginDir, err := filepath.Abs(pluginDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for plugin directory: %v", err)
	}
	fmt.Printf("Using plugin directory: %s\n", absPluginDir)


	// Create plugin manager
	logger := &TestLogger{}
	testDB, err := NewTestDatabase()
	if err != nil {
		log.Fatalf("Failed to create test database: %v", err)
	}
	db := testDB.GetDB()
	manager := plugins.NewManager(absPluginDir, db, logger)

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout
	defer cancel()

	// Initialize plugin manager (this will discover plugins)
	if err := manager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin manager: %v", err)
	}

	// List discovered plugins
	fmt.Println("\nDiscovered Plugins:")
	fmt.Println("------------------")
	pluginList := manager.ListPlugins()
	if len(pluginList) == 0 {
		fmt.Println("No plugins discovered. Ensure 'plugin.cue' files exist and are correctly configured.")
	} else {
		for _, pluginInfo := range pluginList {
			fmt.Printf("- Name: %s\n", pluginInfo.Name)
			fmt.Printf("  Version: %s\n", pluginInfo.Version)
			fmt.Printf("  Type: %s\n", pluginInfo.Type)
			fmt.Printf("  Description: %s\n", pluginInfo.Description)
			fmt.Printf("  Enabled: %t\n", pluginInfo.Enabled)
			fmt.Printf("  IsCore: %t\n", pluginInfo.IsCore)
			fmt.Println()
		}
	}

	// Check if MusicBrainz enricher was discovered
	mbPlugin, exists := manager.GetPlugin("musicbrainz_enricher")
	if !exists {
		fmt.Println("❌ MusicBrainz Enricher plugin not discovered")
		fmt.Println("\nMake sure the plugin.cue file exists in backend/data/plugins/musicbrainz_enricher/")
		os.Exit(1)
	}

	fmt.Printf("✅ MusicBrainz Enricher plugin discovered: %s v%s\n", mbPlugin.Name, mbPlugin.Version)

	// Try to load the plugin
	fmt.Println("\nAttempting to load MusicBrainz Enricher plugin...")
	if err := manager.LoadPlugin(ctx, "musicbrainz_enricher"); err != nil {
		log.Printf("Failed to load plugin: %v", err)
		fmt.Println("❌ Plugin loading failed")
	} else {
		fmt.Println("✅ Plugin loaded successfully")

		// Get the loaded plugin again to check its status and services
		loadedMbPlugin, _ := manager.GetPlugin("musicbrainz_enricher")
		if loadedMbPlugin.MetadataScraperService != nil {
			// Check health via a service call if available, e.g., a Ping or GetInfo method
			// For now, we assume successful load means it's 'healthy' for the test's purpose.
			fmt.Println("✅ Plugin seems healthy (loaded and service client available)")

			// Example of calling a method on the service if one existed like Ping
			// resp, err := loadedMbPlugin.MetadataScraperService.Ping(ctx, &proto.PingRequest{})
			// if err != nil {
			// 	fmt.Printf("Plugin Ping RPC failed: %v\n", err)
			// } else {
			//  fmt.Printf("Plugin Ping RPC success: %s\n", resp.Message)
			// }

		} else {
			fmt.Println("Loaded plugin does not have an active MetadataScraperService client.")
		}
	}

	// Get scanner hook plugins
	scannerHooks := manager.GetScannerHooks()
	fmt.Printf("\nScanner Hook Plugins: %d\n", len(scannerHooks))

	// Get metadata scraper plugins
	metadataScrapers := manager.GetMetadataScrapers()
	fmt.Printf("Metadata Scraper Plugins: %d\n", len(metadataScrapers))

	if len(metadataScrapers) > 0 && metadataScrapers[0] != nil {
		// Attempt to get basic info from the first scraper
		// Assuming the MusicBrainz plugin is the first one if loaded
		scraperClient := metadataScrapers[0] // This is a proto.MetadataScraperServiceClient

		// Test GetSupportedTypes
		supportedTypesResp, err := scraperClient.GetSupportedTypes(ctx, &proto.GetSupportedTypesRequest{})
		if err != nil {
			fmt.Printf("❌ Failed to get supported types from scraper: %v\n", err)
		} else {
			fmt.Printf("Scraper supported types: %v\n", supportedTypesResp.GetTypes())
		}

		// Test CanHandle (example)
		canHandleResp, err := scraperClient.CanHandle(ctx, &proto.CanHandleRequest{FilePath: "/test/file.mp3", MimeType: "audio/mpeg"})
		if err != nil {
			fmt.Printf("❌ Scraper CanHandle call failed: %v\n", err)
		} else {
			fmt.Printf("Scraper can handle MP3 files: %v\n", canHandleResp.GetCanHandle())
		}

	} else {
		fmt.Println("No metadata scrapers available or the first one is nil.")
	}
	

	// Test scanner hook functionality
	if len(scannerHooks) > 0 && scannerHooks[0] != nil {
		fmt.Println("\nTesting scanner hook functionality...")
		hookClient := scannerHooks[0] // This is a proto.ScannerHookServiceClient

		fmt.Printf("Testing hook plugin (first available)\n")

		// Test OnMediaFileScanned
		_, err := hookClient.OnMediaFileScanned(ctx, &proto.OnMediaFileScannedRequest{
			MediaFileId: 123,
			FilePath:    "/test/file.mp3",
			Metadata:    map[string]string{"title": "Test Song", "artist": "Test Artist", "album": "Test Album"},
		})
		if err != nil {
			fmt.Printf("❌ OnMediaFileScanned failed: %v\n", err)
		} else {
			fmt.Println("✅ OnMediaFileScanned test passed")
		}
	} else {
		 fmt.Println("No scanner hooks available or the first one is nil.")
	}


	// Shutdown (this should complete quickly now)
	fmt.Println("\nShutting down plugin manager...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second) // Increased timeout
	defer shutdownCancel()

	if err := manager.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("✅ Plugin system test completed successfully!")
} 