package server

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/server/handlers"
)

var pluginManager *plugins.Manager
var systemEventBus events.EventBus

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
	
	// Initialize plugin manager
	if err := initializePluginManager(); err != nil {
		log.Printf("Failed to initialize plugin manager: %v", err)
	}
	
	// Initialize event bus system
	if err := initializeEventBus(); err != nil {
		log.Printf("Failed to initialize event bus: %v", err)
	}
	
	// Setup routes with event handlers
	setupRoutesWithEventHandlers(r)
	
	// Initialize scanner manager with event bus
	handlers.InitializeScanner(systemEventBus)
	
	return r
}

// initializePluginManager sets up the plugin system
func initializePluginManager() error {
	// Get plugin directory from environment or use default
	pluginDir := os.Getenv("PLUGIN_DIR")
	if pluginDir == "" {
		// Try to find a local path for development
		if _, err := os.Stat("./data/plugins"); err == nil {
			pluginDir = "./data/plugins"
		} else {
			pluginDir = "/app/data/plugins"
		}
	}
	
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
	startupEvent.Data["version"] = "1.0.0" // TODO: Get from build info
	
	if err := systemEventBus.PublishAsync(startupEvent); err != nil {
		log.Printf("Failed to publish startup event: %v", err)
	}
	
	return nil
}