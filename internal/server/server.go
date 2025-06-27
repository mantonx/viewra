package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"github.com/mantonx/viewra/internal/modules/scannermodule"
	"github.com/mantonx/viewra/internal/server/handlers"

	// Import all modules to trigger their registration
	_ "github.com/mantonx/viewra/internal/modules/assetmodule"
	_ "github.com/mantonx/viewra/internal/modules/databasemodule"
	_ "github.com/mantonx/viewra/internal/modules/enrichmentmodule"
	_ "github.com/mantonx/viewra/internal/modules/eventsmodule"
	_ "github.com/mantonx/viewra/internal/modules/mediamodule"
	_ "github.com/mantonx/viewra/internal/modules/playbackmodule"
	_ "github.com/mantonx/viewra/internal/modules/scannermodule"
	_ "github.com/mantonx/viewra/internal/modules/transcodingmodule"

	// Bootstrap core plugins
	_ "github.com/mantonx/viewra/internal/plugins/bootstrap"
)

// Global instances
var (
	systemEventBus events.EventBus
	pluginModule   *pluginmodule.PluginModule
)

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
	setupRoutesWithEventHandlers(r, pluginModule)

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

	// Modules now use service discovery - no manual wiring needed

	// Start modules that need post-initialization setup
	if err := startModules(); err != nil {
		log.Printf("Warning: Failed to start some modules: %v", err)
	}

	moduleInitialized = true
	logModuleStatus()

	return nil
}



// startModules performs post-initialization startup for modules that need it
func startModules() error {
	modules := modulemanager.ListModules()
	for _, module := range modules {
		switch module.ID() {
		case "system.scanner":
			// Start scanner module and perform orphaned job recovery
			if scannerModule, ok := module.(*scannermodule.Module); ok {
				log.Printf("🔄 Starting scanner module and performing orphaned job recovery...")
				if err := scannerModule.Start(); err != nil {
					log.Printf("❌ Failed to start scanner module: %v", err)
					return err
				}
				log.Printf("✅ Scanner module started successfully")
			}
		case "system.plugins":
			// Initialize plugin handlers for legacy compatibility
			if pm, ok := module.(*pluginmodule.PluginModule); ok {
				setGlobalPluginModule(pm)
				handlers.InitializePluginManager(pm)
				log.Printf("✅ Plugin handlers initialized")
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
func (l *simpleLogger) IsTrace() bool                         { return l.GetLevel() <= hclog.Trace }
func (l *simpleLogger) IsDebug() bool                         { return l.GetLevel() <= hclog.Debug }
func (l *simpleLogger) IsInfo() bool                          { return l.GetLevel() <= hclog.Info }
func (l *simpleLogger) IsWarn() bool                          { return l.GetLevel() <= hclog.Warn }
func (l *simpleLogger) IsError() bool                         { return l.GetLevel() <= hclog.Error }
func (l *simpleLogger) ImpliedArgs() []interface{}            { return []interface{}{} }
func (l *simpleLogger) With(args ...interface{}) hclog.Logger { return l }
func (l *simpleLogger) Name() string                          { return "" }
func (l *simpleLogger) Named(name string) hclog.Logger        { return l }
func (l *simpleLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}
func (l *simpleLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", log.LstdFlags)
}
func (l *simpleLogger) SetLevel(level hclog.Level)          {}
func (l *simpleLogger) ResetNamed(name string) hclog.Logger { return l }

// initializeEventBus sets up the system-wide event bus
func initializeEventBus() error {
	config := events.DefaultEventBusConfig() // Use default config
	config.BufferSize = 1000                 // Example capacity, can be tuned

	appEventLogger := &eventLogger{}
	db := database.GetDB() // Assuming database is initialized before event bus
	if db == nil {
		return fmt.Errorf("database not initialized before event bus")
	}
	storage := events.NewDatabaseEventStorage(db)
	metrics := events.NewBasicEventMetrics()

	systemEventBus = events.NewEventBus(config, appEventLogger, storage, metrics)

	// Start the event bus
	ctx := context.Background()                       // Define context for Start
	if err := systemEventBus.Start(ctx); err != nil { // Pass context to Start
		log.Printf("Failed to start event bus: %v", err)
		return err
	}

	log.Println("✅ System event bus initialized and started")
	return nil
}

// eventLogger implements the events.EventLogger interface
type eventLogger struct{}

func (l *eventLogger) Info(msg string, args ...interface{}) { log.Printf("[EVENT-INFO] "+msg, args...) }
func (l *eventLogger) Error(msg string, args ...interface{}) {
	log.Printf("[EVENT-ERROR] "+msg, args...)
}
func (l *eventLogger) Warn(msg string, args ...interface{}) { log.Printf("[EVENT-WARN] "+msg, args...) }
func (l *eventLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[EVENT-DEBUG] "+msg, args...)
}

// GetPluginDirectory returns the configured plugin directory
func GetPluginDirectory() string {
	// Use centralized configuration system
	cfg := config.Get()
	return cfg.Plugins.PluginDir
}

// setGlobalPluginModule sets the global plugin module instance
func setGlobalPluginModule(pm *pluginmodule.PluginModule) {
	pluginModule = pm
}

// GetPluginModule returns the plugin module instance
func GetPluginModule() *pluginmodule.PluginModule {
	return pluginModule
}

// GetEventBus returns the system event bus instance
func GetEventBus() events.EventBus {
	return systemEventBus
}

// ShutdownPluginManager gracefully shuts down the plugin module
func ShutdownPluginManager() error {
	if pluginModule == nil {
		return nil
	}
	log.Println("INFO: Shutting down plugin module...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pluginModule.Shutdown(ctx)
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


