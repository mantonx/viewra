package server

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/server/handlers"

	// Import all modules to trigger their registration
	_ "github.com/mantonx/viewra/internal/modules/scannermodule"
)

var pluginManager *plugins.Manager
var systemEventBus events.EventBus
var moduleInitialized bool
var disabledModules = make(map[string]bool)

// SetupRouter configures and returns the main router
func SetupRouter() *gin.Engine {
	// Set Gin to release mode in production
	// gin.SetMode(gin.ReleaseMode)
	
	r := gin.Default()
	
	// CORS middleware for development
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Initialize event bus system
	if err := initializeEventBus(); err != nil {
		log.Printf("Failed to initialize event bus: %v", err)
	}
	
	// Initialize plugin manager
	if err := initializePluginManager(); err != nil {
		log.Printf("Failed to initialize plugin manager: %v", err)
	}

	// Initialize module system
	if err := initializeModules(); err != nil {
		log.Printf("Failed to initialize modules: %v", err)
	}
	
	// Setup routes with event handlers
	setupRoutesWithEventHandlers(r)
	
	return r
}

// DisableModule disables a specific module (for development/testing only)
func DisableModule(moduleID string) {
	if moduleInitialized {
		logger.Warn("Attempting to disable module %s after modules have been initialized", moduleID)
		return
	}
	
	disabledModules[moduleID] = true
	modulemanager.DisableModule(moduleID)
	logger.Info("Module disabled for development: %s", moduleID)
}

// registerAllModules registers all available modules
func registerAllModules() {
	// Modules are now auto-registered when imported via init() functions
	// This function is kept for any future manual registration needs
}

// initializeModules sets up the module system and loads all modules
func initializeModules() error {
	if moduleInitialized {
		return nil
	}
	
	// Get database connection
	db := database.GetDB()
	
	// Register the event bus globally so modules can access it
	events.SetGlobalEventBus(systemEventBus)
	
	// Register all modules
	registerAllModules()
	
	// Load all modules
	if err := modulemanager.LoadAll(db); err != nil {
		return err
	}
	
	moduleInitialized = true
	logModuleStatus()
	
	return nil
}

// logModuleStatus logs the loaded modules 
func logModuleStatus() {
	modules := modulemanager.ListModules()
	
	log.Printf("✅ Module system initialized with %d modules", len(modules))
	
	// Log loaded modules with nice formatting
	log.Printf("┌────────────────────────────────────────────────────────────────┐")
	log.Printf("│ %-20s │ %-25s │ %-8s │", "MODULE NAME", "MODULE ID", "CORE")
	log.Printf("├────────────────────────────────────────────────────────────────┤")
	
	for _, module := range modules {
		coreStatus := "No"
		if module.Core() {
			coreStatus = "Yes"
		}
		log.Printf("│ %-20s │ %-25s │ %-8s │", 
			truncate(module.Name(), 20), 
			truncate(module.ID(), 25), 
			coreStatus)
	}
	
	log.Printf("└────────────────────────────────────────────────────────────────┘")
}

// truncate shortens a string to the given length, adding ... if needed
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// initializePluginManager sets up the plugin system
func initializePluginManager() error {
	// Get plugin directory path
	pluginDir := GetPluginDirectory()
	
	// Ensure plugin directory exists
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}
	
	// Create plugin logger
	logger := &simpleLogger{}
	
	// Create database wrapper
	db := &databaseWrapper{}
	
	// Create plugin manager
	pluginManager = plugins.NewManager(db, pluginDir, logger)
	
	// Initialize plugin manager
	ctx := context.Background()
	if err := pluginManager.Initialize(ctx); err != nil {
		return err
	}
	
	// Register plugin manager with handlers
	handlers.InitializePluginManager(pluginManager)
	
	log.Printf("✅ Plugin manager initialized with directory: %s", pluginDir)
	
	// Log plugin URLs for testing
	if len(pluginManager.ListPlugins()) > 0 {
		log.Printf("📋 Discovered plugins:")
		for _, info := range pluginManager.ListPlugins() {
			log.Printf("  - %s (v%s) [%s]", info.Name, info.Version, info.ID)
			
			// Log admin page URLs if present
			if info.Manifest != nil && info.Manifest.UI != nil {
				for _, page := range info.Manifest.UI.AdminPages {
					log.Printf("    📄 Admin Page: %s - http://localhost:8080%s", page.Title, page.URL)
				}
			}
		}
	}
	
	return nil
}

// GetPluginManager returns the global plugin manager instance
func GetPluginManager() *plugins.Manager {
	return pluginManager
}

// GetEventBus returns the global event bus instance
func GetEventBus() events.EventBus {
	return systemEventBus
}

// ShutdownPluginManager gracefully shuts down the plugin manager
func ShutdownPluginManager() error {
	if pluginManager != nil {
		ctx := context.Background()
		return pluginManager.Shutdown(ctx)
	}
	return nil
}

// ShutdownEventBus gracefully shuts down the event bus
func ShutdownEventBus() error {
	if systemEventBus != nil {
		ctx := context.Background()
		
		// Publish system shutdown event
		shutdownEvent := events.NewSystemEvent(
			events.EventSystemStopped,
			"System Stopped",
			"Viewra backend system is shutting down",
		)
		
		// Try to publish shutdown event (best effort)
		systemEventBus.PublishAsync(shutdownEvent)
		
		// Stop the event bus
		return systemEventBus.Stop(ctx)
	}
	return nil
}

// Simple logger implementation for plugins
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string, args ...interface{}) {
	log.Printf("[PLUGIN-INFO] "+msg, args...)
}

func (l *simpleLogger) Error(msg string, args ...interface{}) {
	log.Printf("[PLUGIN-ERROR] "+msg, args...)
}

func (l *simpleLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[PLUGIN-WARN] "+msg, args...)
}

func (l *simpleLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[PLUGIN-DEBUG] "+msg, args...)
}

// Database wrapper to implement the plugins.Database interface
type databaseWrapper struct{}

func (d *databaseWrapper) GetDB() interface{} {
	return database.GetDB()
}

// Event logger implementation for the event bus
type eventLogger struct{}

func (l *eventLogger) Info(msg string, args ...interface{}) {
	log.Printf("[EVENT-INFO] "+msg, args...)
}

func (l *eventLogger) Error(msg string, args ...interface{}) {
	log.Printf("[EVENT-ERROR] "+msg, args...)
}

func (l *eventLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[EVENT-WARN] "+msg, args...)
}

func (l *eventLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[EVENT-DEBUG] "+msg, args...)
}

// GetPluginDirectory returns the plugin directory path
func GetPluginDirectory() string {
	dir := os.Getenv("PLUGIN_DIR")
	if dir == "" {
		dir = filepath.Join(".", "data", "plugins")
	}
	return dir
}

// initializeEventBus sets up the system-wide event bus
func initializeEventBus() error {
	// Create event bus configuration
	config := events.DefaultEventBusConfig()
	
	// Create event logger
	logger := &eventLogger{}
	
	// Create database storage
	db := database.GetDB()
	storage := events.NewDatabaseEventStorage(db)
	
	// Create metrics
	metrics := events.NewBasicEventMetrics()
	
	// Create event bus
	systemEventBus = events.NewEventBus(config, logger, storage, metrics)
	
	// Start event bus
	ctx := context.Background()
	if err := systemEventBus.Start(ctx); err != nil {
		return err
	}
	
	// Connect event bus to plugin manager
	if pluginManager != nil {
		pluginManager.SetEventBus(systemEventBus)
	}
	
	log.Printf("✅ Event bus initialized and started")
	
	// Publish system startup event
	startupEvent := events.NewSystemEvent(
		events.EventSystemStarted,
		"System Started",
		"Viewra backend system has started successfully",
	)
	
	startupEvent.Data = map[string]interface{}{
		"version": "1.0.0", // TODO: Get from build info
	}
	
	if err := systemEventBus.PublishAsync(startupEvent); err != nil {
		log.Printf("Failed to publish startup event: %v", err)
	}
	
	return nil
}
