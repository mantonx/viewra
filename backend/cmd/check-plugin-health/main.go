package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

func main() {
	fmt.Println("=== Plugin Health Check Tool ===")

	// Initialize configuration
	if err := config.Load(""); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	database.Initialize()
	db := database.GetDB()

	// Initialize plugin module
	pluginConfig := &pluginmodule.PluginModuleConfig{
		PluginDir:       "./data/plugins",
		EnabledCore:     []string{},
		EnabledExternal: []string{},
		LibraryConfigs:  make(map[string]pluginmodule.LibraryPluginSettings),
	}

	pluginModule := pluginmodule.NewPluginModule(db, pluginConfig)
	ctx := context.Background()

	fmt.Println("🔍 Initializing plugin system...")
	if err := pluginModule.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize plugin module: %v", err)
	}
	defer pluginModule.Shutdown(ctx)

	fmt.Println("✅ Plugin system initialized")

	// Get external plugin manager
	externalManager := pluginModule.GetExternalManager()

	// Check current plugin status from database
	fmt.Println("\n📊 Database Plugin Status:")
	var dbPlugins []database.Plugin
	if err := db.Where("type = ?", "external").Find(&dbPlugins).Error; err != nil {
		log.Fatalf("Failed to query database plugins: %v", err)
	}

	for _, plugin := range dbPlugins {
		enabledAt := "never"
		if plugin.EnabledAt != nil {
			enabledAt = plugin.EnabledAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("  📦 %s (%s)\n", plugin.Name, plugin.PluginID)
		fmt.Printf("     Status: %s | Enabled At: %s\n", plugin.Status, enabledAt)
		fmt.Printf("     Version: %s | Path: %s\n", plugin.Version, plugin.InstallPath)
	}

	// List discovered plugins
	fmt.Println("\n🔍 Discovered Plugins:")
	allPlugins := externalManager.ListPlugins()
	for _, plugin := range allPlugins {
		fmt.Printf("  📦 %s (%s) - Running: %v\n", plugin.Name, plugin.ID, plugin.Enabled)
	}

	// Perform health checks
	fmt.Println("\n🏥 Plugin Health Checks:")
	healthResults, err := externalManager.CheckAllPluginsHealth()
	if err != nil {
		log.Fatalf("Failed to perform health checks: %v", err)
	}

	healthyCount := 0
	for _, result := range healthResults {
		status := "❌ UNHEALTHY"
		if result.Healthy {
			status = "✅ HEALTHY"
			healthyCount++
		}

		fmt.Printf("  %s %s (%s)\n", status, result.Name, result.PluginID)
		fmt.Printf("     Status: %s | Uptime: %s\n", result.Status, result.Uptime)
		if result.Error != "" {
			fmt.Printf("     Error: %s\n", result.Error)
		}
	}

	// Summary
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("  Total plugins: %d\n", len(dbPlugins))
	fmt.Printf("  Running plugins: %d\n", len(externalManager.GetRunningPlugins()))
	fmt.Printf("  Healthy plugins: %d\n", healthyCount)

	// Check for common issues
	fmt.Printf("\n🔧 Issue Analysis:\n")
	
	enabledButNotRunning := 0
	missingBinaries := 0
	
	for _, dbPlugin := range dbPlugins {
		if dbPlugin.Status == "enabled" {
			// Check if it's actually running
			if plugin, exists := externalManager.GetPlugin(dbPlugin.PluginID); exists {
				if !plugin.Running {
					enabledButNotRunning++
				}
			}
		}
		
		// Check if binary exists
		if plugin, exists := externalManager.GetPlugin(dbPlugin.PluginID); exists {
			if _, err := os.Stat(plugin.Path); os.IsNotExist(err) {
				missingBinaries++
			}
		}
	}

	if enabledButNotRunning > 0 {
		fmt.Printf("  ⚠️  %d plugins are enabled but not running\n", enabledButNotRunning)
		fmt.Printf("     Fix: Run 'docker-compose restart backend' to reload plugins\n")
	}

	if missingBinaries > 0 {
		fmt.Printf("  ⚠️  %d plugins have missing binaries\n", missingBinaries)
		fmt.Printf("     Fix: Check plugin installation and binary paths\n")
	}

	if healthyCount == len(dbPlugins) && len(dbPlugins) > 0 {
		fmt.Printf("  🎉 All plugins are healthy!\n")
	}

	// Test plugin auto-loading
	fmt.Printf("\n🔄 Testing Plugin Auto-Loading:\n")
	if err := externalManager.LoadAllEnabledPlugins(ctx); err != nil {
		fmt.Printf("  ❌ Auto-loading failed: %v\n", err)
	} else {
		fmt.Printf("  ✅ Auto-loading completed successfully\n")
	}

	// Final verification
	time.Sleep(2 * time.Second)
	finalHealthResults, _ := externalManager.CheckAllPluginsHealth()
	finalHealthyCount := 0
	for _, result := range finalHealthResults {
		if result.Healthy {
			finalHealthyCount++
		}
	}

	fmt.Printf("\n🎯 Final Status: %d/%d plugins healthy\n", finalHealthyCount, len(dbPlugins))

	// Output JSON for automation
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		status, _ := externalManager.GetPluginStatus()
		jsonOutput, _ := json.MarshalIndent(status, "", "  ")
		fmt.Printf("\n📄 JSON Output:\n%s\n", string(jsonOutput))
	}

	fmt.Println("\n=== Health Check Complete ===")
} 