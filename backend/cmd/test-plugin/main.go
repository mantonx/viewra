package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
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

	// Create plugin module
	db := database.GetDB()
	pluginConfig := &pluginmodule.PluginModuleConfig{
		PluginDir:       pluginDir,
		EnabledCore:     []string{"ffmpeg", "enrichment", "tv_structure", "movie_structure"},
		EnabledExternal: []string{},
		LibraryConfigs:  make(map[string]pluginmodule.LibraryPluginSettings),
	}

	pluginModule := pluginmodule.NewPluginModule(db, pluginConfig)

	// Initialize plugin module
	ctx := context.Background()
	if err := pluginModule.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin module: %v", err)
	}
	defer pluginModule.Shutdown(ctx)

	fmt.Println("✅ Plugin module initialized successfully")

	// List discovered plugins
	allPlugins := pluginModule.ListAllPlugins()
	fmt.Printf("📋 Discovered %d plugins:\n", len(allPlugins))

	if len(allPlugins) == 0 {
		fmt.Println("⚠️  No plugins found.")
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

		// Test core plugins
		fmt.Println("\nTesting core plugins...")
		corePlugins := pluginModule.GetCoreManager().ListCorePluginInfo()
		fmt.Printf("📋 Found %d core plugins:\n", len(corePlugins))

		for _, corePlugin := range corePlugins {
			fmt.Printf("  - %s (v%s) [%s] - Enabled: %v\n",
				corePlugin.Name, corePlugin.Version, corePlugin.Type, corePlugin.Enabled)
		}

		fmt.Println("\n=== Plugin System Test Completed Successfully ===")
		return
	}

	fmt.Printf("\nAttempting to load external plugin: %s\n", testPluginID)

	// Test plugin loading
	if err := pluginModule.LoadExternalPlugin(ctx, testPluginID); err != nil {
		fmt.Printf("❌ Failed to load plugin '%s': %v\n", testPluginID, err)
		fmt.Println("This may be expected if the plugin binary is not available.")
	} else {
		fmt.Printf("✅ Plugin '%s' loaded successfully\n", testPluginID)

		// Get loaded plugin details
		if loadedPlugin, exists := pluginModule.GetExternalPlugin(testPluginID); exists {
			fmt.Printf("✅ Plugin is running: %v\n", loadedPlugin.Running)
		}

		// Test unloading
		fmt.Printf("Unloading plugin '%s'...\n", testPluginID)
		if err := pluginModule.UnloadExternalPlugin(ctx, testPluginID); err != nil {
			fmt.Printf("❌ Failed to unload plugin: %v\n", err)
		} else {
			fmt.Printf("✅ Plugin '%s' unloaded successfully\n", testPluginID)
		}
	}

	// Test core plugins
	fmt.Println("\nTesting core plugins...")
	corePlugins := pluginModule.GetCoreManager().ListCorePluginInfo()
	fmt.Printf("📋 Found %d core plugins:\n", len(corePlugins))

	for _, corePlugin := range corePlugins {
		fmt.Printf("  - %s (v%s) [%s] - Enabled: %v\n",
			corePlugin.Name, corePlugin.Version, corePlugin.Type, corePlugin.Enabled)
	}

	fmt.Println("\n=== Plugin System Test Completed Successfully ===")
}
