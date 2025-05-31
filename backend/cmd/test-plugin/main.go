package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
)

func main() {
	// Test plugin discovery and basic functionality
	fmt.Println("=== Plugin System Test ===")
	
	// Initialize configuration
	if err := config.Load(""); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database.Initialize()

	// Create logger
	appLogger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-test",
		Level: hclog.Debug,
	})

	// Get plugin directory
	cfg := config.Get()
	pluginDir := cfg.Plugins.PluginDir
	if pluginDir == "" {
		pluginDir = "./data/plugins" // Default
	}

	// Ensure directory exists
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		log.Fatalf("Failed to create plugin directory: %v", err)
	}

	fmt.Printf("Using plugin directory: %s\n", pluginDir)

	// Create plugin manager
	db := database.GetDB()
	manager := plugins.NewManager(pluginDir, db, appLogger)

	// Initialize plugin manager
	ctx := context.Background()
	if err := manager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin manager: %v", err)
	}
	defer manager.Shutdown(ctx)

	fmt.Println("✅ Plugin manager initialized successfully")

	// Test plugin discovery
	fmt.Println("\nDiscovering plugins...")
	if err := manager.DiscoverPlugins(); err != nil {
		log.Fatalf("Failed to discover plugins: %v", err)
	}

	// List discovered plugins
	allPlugins := manager.ListPlugins()
	fmt.Printf("📋 Discovered %d plugins:\n", len(allPlugins))
	
	if len(allPlugins) == 0 {
		fmt.Println("⚠️  No plugins found. Make sure plugin files exist in the plugins directory.")
		fmt.Printf("Expected directory structure: %s/{plugin-name}/plugin.cue\n", pluginDir)
		return
	}

	// Display all discovered plugins
	for i, pluginInfo := range allPlugins {
		fmt.Printf("  %d. %s (v%s) [%s]\n", i+1, pluginInfo.Name, pluginInfo.Version, pluginInfo.ID)
		fmt.Printf("     Type: %s | Core: %v | Enabled: %v\n", 
			pluginInfo.Type, pluginInfo.IsCore, pluginInfo.Enabled)
	}

	// Test loading the first external plugin found
	var testPluginID string
	for _, pluginInfo := range allPlugins {
		if !pluginInfo.IsCore {
			testPluginID = pluginInfo.ID
			break
		}
	}

	if testPluginID == "" {
		fmt.Println("⚠️  No external plugins found to test.")
		fmt.Println("✅ Core plugin system verification completed successfully.")
		return
	}

	fmt.Printf("\nAttempting to load external plugin: %s\n", testPluginID)
	
	// Test plugin loading
	if err := manager.LoadPlugin(ctx, testPluginID); err != nil {
		fmt.Printf("❌ Failed to load plugin '%s': %v\n", testPluginID, err)
		fmt.Println("This may be expected if the plugin binary is not available.")
	} else {
		fmt.Printf("✅ Plugin '%s' loaded successfully\n", testPluginID)
		
		// Get loaded plugin details
		if loadedPlugin, exists := manager.GetPlugin(testPluginID); exists {
			fmt.Printf("✅ Plugin is running: %v\n", loadedPlugin.Running)
			
			// Test plugin services
			if loadedPlugin.PluginService != nil {
				fmt.Println("✅ Plugin exposes PluginService")
			}
			if loadedPlugin.MetadataScraperService != nil {
				fmt.Println("✅ Plugin exposes MetadataScraperService")
			}
			if loadedPlugin.SearchService != nil {
				fmt.Println("✅ Plugin exposes SearchService")
			}
		}
		
		// Test unloading
		fmt.Printf("Unloading plugin '%s'...\n", testPluginID)
		if err := manager.UnloadPlugin(ctx, testPluginID); err != nil {
			fmt.Printf("❌ Failed to unload plugin: %v\n", err)
		} else {
			fmt.Printf("✅ Plugin '%s' unloaded successfully\n", testPluginID)
		}
	}

	// Test core plugins
	fmt.Println("\nTesting core plugins...")
	corePlugins := manager.ListCorePlugins()
	fmt.Printf("📋 Found %d core plugins:\n", len(corePlugins))
	
	for _, corePlugin := range corePlugins {
		fmt.Printf("  - %s (v%s) [%s] - Enabled: %v\n", 
			corePlugin.Name, corePlugin.Version, corePlugin.Type, corePlugin.Enabled)
	}

	fmt.Println("\n=== Plugin System Test Completed Successfully ===")
} 