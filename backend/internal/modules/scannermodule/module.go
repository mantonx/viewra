package scannermodule

import (
	"fmt"
	"runtime"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	// ModuleID is the unique identifier for the scanner module
	ModuleID = "system.scanner"

	// ModuleName is the display name for the scanner module
	ModuleName = "Media Scanner"
)

// Module implements the scanner functionality as a module
type Module struct {
	scannerManager *scanner.Manager
	db             *gorm.DB
	eventBus       events.EventBus
	pluginModule   *pluginmodule.PluginModule
}

// NewModule creates a new scanner module
func NewModule(db *gorm.DB, eventBus events.EventBus, pluginModule *pluginmodule.PluginModule) *Module {
	return &Module{
		db:           db,
		eventBus:     eventBus,
		pluginModule: pluginModule,
	}
}

// ID returns the unique module identifier
func (m *Module) ID() string {
	return ModuleID
}

// Name returns the module display name
func (m *Module) Name() string {
	return ModuleName
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return true // Scanner is a core module
}

// Migrate performs any necessary database migrations
func (m *Module) Migrate(db *gorm.DB) error {
	logger.Info("Migrating scanner database schema")

	// Migrate scan job model
	if err := db.AutoMigrate(&database.ScanJob{}); err != nil {
		return err
	}

	// Add any other scanner-related models here

	return nil
}

// Init initializes the scanner module
func (m *Module) Init() error {
	logger.Info("Initializing scanner module")

	if m.db == nil {
		logger.Error("Scanner module db is nil")
		m.db = database.GetDB()
	}

	if m.eventBus == nil {
		logger.Error("Scanner module eventBus is nil")
		m.eventBus = events.GetGlobalEventBus()
	}

	// Create scanner manager with plugin module
	m.scannerManager = scanner.NewManager(m.db, m.eventBus, m.pluginModule, &scanner.ManagerOptions{
		Workers:      runtime.NumCPU(),
		CleanupHours: 24,
	})

	if m.scannerManager == nil {
		logger.Error("Failed to initialize scanner manager")
		return fmt.Errorf("failed to initialize scanner manager")
	}

	logger.Info("Scanner module initialized successfully with manager: %v", m.scannerManager)

	return nil
}

// Start starts the scanner module services and performs recovery
func (m *Module) Start() error {
	logger.Info("Starting scanner module")

	if m.scannerManager == nil {
		return fmt.Errorf("scanner manager not initialized - call Init() first")
	}

	// Recover orphaned jobs from previous backend instances
	logger.Info("Recovering orphaned scan jobs...")
	if err := m.scannerManager.RecoverOrphanedJobs(); err != nil {
		logger.Error("Failed to recover orphaned jobs: %v", err)
		// Don't fail startup, just log the error
	}

	// Start enhanced safeguards system
	logger.Info("Starting enhanced safeguards system...")
	if err := m.scannerManager.StartSafeguards(); err != nil {
		logger.Error("Failed to start safeguards system: %v", err)
		// Don't fail startup, but this is concerning
	} else {
		logger.Info("Enhanced safeguards system started successfully")
	}

	// Start background state synchronizer to prevent future inconsistencies
	logger.Info("Starting background state synchronizer...")
	m.scannerManager.StartStateSynchronizer()

	// Start file monitoring service
	logger.Info("Starting file monitoring service...")
	if err := m.scannerManager.StartFileMonitoring(); err != nil {
		logger.Error("Failed to start file monitoring: %v", err)
		// Don't fail startup, just log the error
	} else {
		logger.Info("File monitoring service started successfully")
	}

	logger.Info("Scanner module started successfully")
	return nil
}

// Stop gracefully shuts down the scanner module
func (m *Module) Stop() error {
	logger.Info("Stopping scanner module")

	if m.scannerManager == nil {
		return nil
	}

	// Stop safeguards system first
	logger.Info("Stopping safeguards system...")
	if err := m.scannerManager.StopSafeguards(); err != nil {
		logger.Error("Error stopping safeguards system: %v", err)
	}

	// Stop scanner manager
	if err := m.scannerManager.Shutdown(); err != nil {
		logger.Error("Error shutting down scanner manager: %v", err)
		return err
	}

	logger.Info("Scanner module stopped successfully")
	return nil
}

// GetScannerManager returns the underlying scanner manager
func (m *Module) GetScannerManager() *scanner.Manager {
	if m.scannerManager == nil {
		logger.Error("ScannerManager is nil in GetScannerManager()")

		// Try to re-initialize
		if m.db == nil {
			logger.Error("Module database is nil, getting from global database")
			m.db = database.GetDB()
		}

		if m.eventBus == nil {
			logger.Error("Module eventBus is nil, getting from global event bus")
			m.eventBus = events.GetGlobalEventBus()
		}

		if m.db == nil {
			logger.Error("CRITICAL: Cannot create scanner manager - database connection is nil")
			return nil
		}

		m.scannerManager = scanner.NewManager(m.db, m.eventBus, m.pluginModule, &scanner.ManagerOptions{
			Workers:      runtime.NumCPU(),
			CleanupHours: 24,
		})
		logger.Info("Re-initialized scanner manager: %v", m.scannerManager)

		// Double-check that the manager was created properly
		if m.scannerManager == nil {
			logger.Error("CRITICAL: Failed to create scanner manager")
			return nil
		}
	}

	return m.scannerManager
}

// SetPluginModule sets the plugin module for the scanner
func (m *Module) SetPluginModule(pluginModule *pluginmodule.PluginModule) {
	logger.Info("Setting plugin module for scanner")
	m.pluginModule = pluginModule

	// Update scanner manager if it exists
	if m.scannerManager != nil {
		m.scannerManager.SetPluginModule(pluginModule)
	}
}

// Register registers the scanner module with the module system
func Register() {
	scannerModule := &Module{
		db:       nil, // Will be set during Init
		eventBus: nil, // Will be set during Init
	}
	modulemanager.Register(scannerModule)
}
