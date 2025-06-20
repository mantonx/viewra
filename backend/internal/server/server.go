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
	"github.com/mantonx/viewra/internal/modules/enrichmentmodule"
	"github.com/mantonx/viewra/internal/modules/mediamodule"
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
	var pluginModule *pluginmodule.PluginModule
	var enrichmentModule *enrichmentmodule.Module

	// Get database reference
	db := database.GetDB()

	modules := modulemanager.ListModules()

	// Find existing enrichment and plugin modules
	for _, module := range modules {
		if module.ID() == "system.enrichment" {
			if em, ok := module.(*enrichmentmodule.Module); ok {
				enrichmentModule = em
			}
		}
		if module.ID() == "system.plugins" {
			if pm, ok := module.(*pluginmodule.PluginModule); ok {
				pluginModule = pm
				log.Printf("🔍 DEBUG: Found existing plugin module from registry")
			}
		}
	}

	// If we have both modules, connect them
	if enrichmentModule != nil && pluginModule != nil {
		log.Printf("🔍 DEBUG: Using existing plugin module instead of creating new one")

		// NOTE: Don't call Initialize() here - the module manager already initialized it
		// The plugin module is already initialized by the module manager's automatic initialization

		// Connect external manager to enrichment module
		extMgr := pluginModule.GetExternalManager()
		log.Printf("🔍 DEBUG: External manager from existing plugin module: %v", extMgr != nil)

		if extMgr != nil {
			enrichmentModule.SetExternalPluginManager(extMgr)
			log.Printf("✅ Connected external plugin manager to enrichment module")
		} else {
			log.Printf("⚠️  WARNING: GetExternalManager() returned nil from existing plugin module")
		}

		// Initialize plugin handlers with the existing plugin module
		handlers.InitializePluginManager(pluginModule)
		log.Printf("✅ Plugin handlers initialized with existing plugin module")

		log.Printf("✅ Plugin system connected to enrichment module using existing plugin module")
	} else if enrichmentModule != nil {
		log.Printf("🔍 DEBUG: No existing plugin module found, creating new one")
		// Only create a new plugin module if one doesn't exist
		// Get plugin directory and create config
		pluginDir := GetPluginDirectory()

		// Ensure plugin directory exists
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			log.Printf("WARNING: Failed to create plugin directory: %v", err)
		}

		// Create plugin module config
		pluginConfig := &pluginmodule.PluginModuleConfig{
			PluginDir:       pluginDir,
			EnabledCore:     []string{"ffmpeg", "enrichment", "tv_structure", "movie_structure"},
			EnabledExternal: []string{},
			LibraryConfigs:  make(map[string]pluginmodule.LibraryPluginSettings),
			EnableHotReload: true, // Enable hot reload by default for development
			HotReload: pluginmodule.PluginHotReloadConfig{
				Enabled:         true,
				DebounceDelayMs: 500,
				WatchPatterns:   []string{"*_transcoder", "*_enricher", "*_scanner"},
				ExcludePatterns: []string{"*.tmp", "*.log", "*.pid", ".git*", "*.swp", "*.swo", "go.mod", "go.sum", "*.go", "plugin.cue", "*.json"},
				PreserveState:   true,
				MaxRetries:      3,
				RetryDelayMs:    1000,
			},
		}

		// Create and initialize plugin module
		log.Printf("🔍 DEBUG: Creating plugin module with config: %+v", pluginConfig)
		pluginModule = pluginmodule.NewPluginModule(db, pluginConfig)
		log.Printf("🔍 DEBUG: Plugin module created: %v", pluginModule != nil)

		ctx := context.Background()
		log.Printf("🔍 DEBUG: About to initialize plugin module...")
		if err := pluginModule.Initialize(ctx, db); err != nil {
			log.Printf("WARNING: Failed to initialize plugin module: %v", err)
		} else {
			log.Printf("✅ Plugin module initialized successfully")
		}

		// Debug external manager before calling GetExternalManager
		log.Printf("🔍 DEBUG: About to call GetExternalManager()...")
		extMgr := pluginModule.GetExternalManager()
		log.Printf("🔍 DEBUG: External manager from GetExternalManager(): %v", extMgr)
		log.Printf("🔍 DEBUG: External manager is nil: %v", extMgr == nil)

		if extMgr != nil {
			log.Printf("🔍 DEBUG: External manager type: %T", extMgr)
			enrichmentModule.SetExternalPluginManager(extMgr)
			log.Printf("✅ Connected external plugin manager to enrichment module")
		} else {
			log.Printf("⚠️  WARNING: GetExternalManager() returned nil - external plugin manager not connected!")
		}

		// Initialize plugin handlers with the plugin module
		handlers.InitializePluginManager(pluginModule)
		log.Printf("✅ Plugin handlers initialized with plugin module")

		log.Printf("✅ Plugin system connected to enrichment module")
	}

	for _, module := range modules {
		// Connect scanner module to plugin system
		if module.ID() == "system.scanner" {
			if scannerModule, ok := module.(*scannermodule.Module); ok {
				if pluginModule != nil {
					// Set the plugin module on the scanner
					scannerModule.SetPluginModule(pluginModule)
					// Get enabled file handlers for scanning
					fileHandlers := pluginModule.GetEnabledFileHandlers()
					log.Printf("✅ Connected plugin module to scanner (%d handlers available) - scanner: %v", len(fileHandlers), scannerModule != nil)
				}
			}
		}

		// Connect media module to plugin system
		if module.ID() == "system.media" {
			if mediaModule, ok := module.(*mediamodule.Module); ok {
				if pluginModule != nil {
					mediaModule.SetPluginModule(pluginModule)
					log.Printf("✅ Connected plugin module to media module")
				}
			}
		}

		// Connect enrichment module to scanner
		if module.ID() == "system.enrichment" {
			if enrichmentModule, ok := module.(*enrichmentmodule.Module); ok {
				// Find scanner module and register enrichment module as hook
				for _, scannerMod := range modules {
					if scannerMod.ID() == "system.scanner" {
						if scannerModule, ok := scannerMod.(*scannermodule.Module); ok {
							manager := scannerModule.GetScannerManager()
							if manager != nil {
								manager.RegisterEnrichmentHook(enrichmentModule)
								log.Printf("✅ Registered enrichment module as scanner hook")
							}
						}
						break
					}
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
