package server

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/server/handlers"
)

var pluginManager *plugins.Manager

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
	
	// Setup routes
	setupRoutes(r)
	
	// Initialize scanner manager
	handlers.InitializeScanner()
	
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

// ShutdownPluginManager gracefully shuts down the plugin manager
func ShutdownPluginManager() error {
	if pluginManager != nil {
		ctx := context.Background()
		return pluginManager.Shutdown(ctx)
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