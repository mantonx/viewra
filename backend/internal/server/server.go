package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/scannermodule"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/enrichment"
	"github.com/mantonx/viewra/internal/plugins/ffmpeg"
	"github.com/mantonx/viewra/internal/server/handlers"

	// Import all modules to trigger their registration
	_ "github.com/mantonx/viewra/internal/modules/assetmodule"
	_ "github.com/mantonx/viewra/internal/modules/databasemodule"
	_ "github.com/mantonx/viewra/internal/modules/eventsmodule"
	_ "github.com/mantonx/viewra/internal/modules/mediamodule"
	_ "github.com/mantonx/viewra/internal/modules/scannermodule"
)

var pluginManager plugins.Manager
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
	
	// Register core API routes for discovery
	apiroutes.Register("/api", "GET", "Lists all available API endpoints.")
	apiroutes.Register("/api/v1/users", "GET, POST, PUT, DELETE", "Manages user accounts and authentication.") // Example methods
	apiroutes.Register("/api/v1/media", "GET, POST, PUT, DELETE", "Manages media items, libraries, and metadata.")
	apiroutes.Register("/api/v1/plugins", "GET, POST, PUT, DELETE", "Manages plugins, their configurations, and status.")
	apiroutes.Register("/swagger/index.html", "GET", "Serves API documentation (Swagger UI).")

	// Setup routes with event handlers
	setupRoutesWithEventHandlers(r, pluginManager)
	
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
	
	// Connect plugin manager to modules that need it
	if err := connectPluginManagerToModules(); err != nil {
		log.Printf("Warning: Failed to connect plugin manager to modules: %v", err)
	}
	
	// Start modules that need post-initialization setup
	if err := startModules(); err != nil {
		log.Printf("Warning: Failed to start some modules: %v", err)
	}
	
	moduleInitialized = true
	logModuleStatus()
	
	return nil
}

// connectPluginManagerToModules connects the plugin manager to modules that need it
func connectPluginManagerToModules() error {
	modules := modulemanager.ListModules()
	for _, module := range modules {
		// Connect to media module
		if module.ID() == "system.media" {
			// Plugin manager is already passed to metadata manager through constructor
			// No additional setup needed here
			log.Printf("✅ Media module already connected to plugin system")
		}
		
		// Connect to scanner module
		if module.ID() == "system.scanner" {
			if scannerModule, ok := module.(*scannermodule.Module); ok {
				if pluginManager != nil {
					scannerModule.SetPluginManager(pluginManager)
					log.Printf("✅ Connected plugin manager to scanner module")
				}
			}
		}
	}
	return nil
}

// startModules performs post-initialization startup for modules that need it
func startModules() error {
	modules := modulemanager.ListModules()
	for _, module := range modules {
		// Start scanner module and perform orphaned job recovery
		if module.ID() == "system.scanner" {
			if scannerModule, ok := module.(*scannermodule.Module); ok {
				log.Printf("🔄 Starting scanner module and performing orphaned job recovery...")
				if err := scannerModule.Start(); err != nil {
					log.Printf("❌ Failed to start scanner module: %v", err)
					return err
				}
				log.Printf("✅ Scanner module started successfully")
			}
		}
	}
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

// simpleLogger implements hclog.Logger for plugin manager
type simpleLogger struct{}

func (l *simpleLogger) GetLevel() hclog.Level { return hclog.Info }
func (l *simpleLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	log.Printf("[%s] %s %v", level, msg, args)
}
func (l *simpleLogger) Trace(msg string, args ...interface{}) { l.Log(hclog.Trace, msg, args...) }
func (l *simpleLogger) Info(msg string, args ...interface{})  { l.Log(hclog.Info, msg, args...) }
func (l *simpleLogger) Error(msg string, args ...interface{}) { l.Log(hclog.Error, msg, args...) }
func (l *simpleLogger) Warn(msg string, args ...interface{})  { l.Log(hclog.Warn, msg, args...) }
func (l *simpleLogger) Debug(msg string, args ...interface{}) { l.Log(hclog.Debug, msg, args...) }
func (l *simpleLogger) IsTrace() bool { return l.GetLevel() <= hclog.Trace }
func (l *simpleLogger) IsDebug() bool { return l.GetLevel() <= hclog.Debug }
func (l *simpleLogger) IsInfo() bool  { return l.GetLevel() <= hclog.Info }
func (l *simpleLogger) IsWarn() bool  { return l.GetLevel() <= hclog.Warn }
func (l *simpleLogger) IsError() bool { return l.GetLevel() <= hclog.Error }
func (l *simpleLogger) ImpliedArgs() []interface{} { return []interface{}{} }
func (l *simpleLogger) With(args ...interface{}) hclog.Logger { return l }
func (l *simpleLogger) Name() string { return "" }
func (l *simpleLogger) Named(name string) hclog.Logger { return l }
func (l *simpleLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}
func (l *simpleLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", log.LstdFlags)
}
func (l *simpleLogger) SetLevel(level hclog.Level) {}
func (l *simpleLogger) ResetNamed(name string) hclog.Logger { return l }

// initializePluginManager sets up the plugin system
func initializePluginManager() error {
	// Get plugin directory path
	pluginDir := GetPluginDirectory()
	
	// Ensure plugin directory exists
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}
	
	// Create plugin logger
	appLogger := &simpleLogger{}
	
	// Get database connection
	db := database.GetDB()
	
	// Create plugin manager
	pluginManager = plugins.NewManager(pluginDir, db, appLogger)
	
	// Initialize plugin manager
	ctx := context.Background()
	if err := pluginManager.Initialize(ctx); err != nil {
		return err
	}
	
	// Register core plugins
	if err := registerCorePlugins(); err != nil {
		log.Printf("WARNING: Failed to register core plugins: %v", err)
	}
	
	// Register plugin manager with handlers
	handlers.InitializePluginManager(pluginManager)
	
	log.Printf("✅ Plugin manager initialized with directory: %s", pluginDir)
	
	// Log plugin URLs for testing
	if len(pluginManager.ListPlugins()) > 0 {
		log.Printf("📋 Discovered plugins:")
		for _, info := range pluginManager.ListPlugins() {
			log.Printf("  - %s (v%s) [%s]", info.Name, info.Version, info.ID)

			// The Manifest field is gone. Admin pages would be discoverable via plugin services if needed here.
			// For simplicity, this detailed logging is removed for now.
			/*
			// Log admin page URLs if present
			if info.Manifest != nil && info.Manifest.UI != nil {
				for _, page := range info.Manifest.UI.AdminPages {
					log.Printf("    📄 Admin Page: %s - http://localhost:8080%s", page.Title, page.URL)
				}
			}
			*/
		}
	}
	
	return nil
}

// registerCorePlugins registers core plugins
func registerCorePlugins() error {
	// Register FFmpeg core plugin (for video files)
	ffmpegPlugin := ffmpeg.NewFFmpegCorePlugin()
	if err := pluginManager.RegisterCorePlugin(ffmpegPlugin); err != nil {
		return fmt.Errorf("failed to register FFmpeg core plugin: %w", err)
	}
	
	// Register enrichment core plugin (for music metadata and artwork extraction)
	enrichmentPlugin := enrichment.NewEnrichmentCorePlugin()
	if err := pluginManager.RegisterCorePlugin(enrichmentPlugin); err != nil {
		return fmt.Errorf("failed to register enrichment core plugin: %w", err)
	}
	
	log.Printf("✅ Registered core plugins: FFmpeg, Enrichment")
	return nil
}

// GetPluginManager returns the plugin manager instance
func GetPluginManager() plugins.Manager {
	return pluginManager
}

// GetEventBus returns the system event bus instance
func GetEventBus() events.EventBus {
	return systemEventBus
}

// ShutdownPluginManager gracefully shuts down the plugin manager
func ShutdownPluginManager() error {
	if pluginManager == nil {
		return nil
	}
	log.Println("INFO: Shutting down plugin manager...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pluginManager.Shutdown(ctx)
}

// ShutdownEventBus gracefully shuts down the event bus
func ShutdownEventBus() error {
	if systemEventBus == nil {
		return nil
	}
	log.Println("INFO: Shutting down event bus...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return systemEventBus.Stop(ctx)
}

// eventLogger implements the events.EventLogger interface
type eventLogger struct{}

func (l *eventLogger) Info(msg string, args ...interface{})  { log.Printf("[EVENT-INFO] "+msg, args...) }
func (l *eventLogger) Error(msg string, args ...interface{}) { log.Printf("[EVENT-ERROR] "+msg, args...) }
func (l *eventLogger) Warn(msg string, args ...interface{})  { log.Printf("[EVENT-WARN] "+msg, args...) }
func (l *eventLogger) Debug(msg string, args ...interface{}) { log.Printf("[EVENT-DEBUG] "+msg, args...) }

// GetPluginDirectory returns the configured plugin directory
func GetPluginDirectory() string {
	// Check environment variable first
	if pluginDir := os.Getenv("PLUGIN_DIR"); pluginDir != "" {
		return pluginDir
	}
	
	// TODO: Make this configurable via config file
	defaultDir := filepath.Join(".", "backend", "data", "plugins") // Adjusted default
	exePath, err := os.Executable()
	if err == nil {
		baseDir := filepath.Dir(exePath)
		// If running from a typical Go bin layout, adjust path relative to project root
		// This is a heuristic and might need refinement for different deployment scenarios
		if filepath.Base(baseDir) == "bin" || filepath.Base(baseDir) == "cmd" {
			projectRoot := filepath.Dir(filepath.Dir(baseDir)) // Go up two levels
            if filepath.Base(projectRoot) == "backend" { // If exe is in backend/cmd/xxx/main or backend/bin/xxx
                projectRoot = filepath.Dir(projectRoot) // Go up one more for viewra root
            }
			return filepath.Join(projectRoot, "backend", "data", "plugins")
		}
		// If not in a typical Go bin layout, assume running from project root or similar structure
		return filepath.Join(baseDir, "backend", "data", "plugins") 
	}
	return defaultDir
}

// initializeEventBus sets up the system-wide event bus
func initializeEventBus() error {
	config := events.DefaultEventBusConfig() // Use default config
	config.BufferSize = 1000 // Example capacity, can be tuned

	appEventLogger := &eventLogger{}
	db := database.GetDB() // Assuming database is initialized before event bus
	if db == nil {
		return fmt.Errorf("database not initialized before event bus")
	}
	storage := events.NewDatabaseEventStorage(db)
	metrics := events.NewBasicEventMetrics()

	systemEventBus = events.NewEventBus(config, appEventLogger, storage, metrics)

	// Start the event bus
	ctx := context.Background() // Define context for Start
	if err := systemEventBus.Start(ctx); err != nil { // Pass context to Start
		log.Printf("Failed to start event bus: %v", err)
		return err
	}

	log.Println("✅ System event bus initialized and started")
	return nil
}
