package scannermodule

import (
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
	"github.com/mantonx/viewra/internal/plugins"
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
	pluginManager  plugins.Manager
}

// NewModule creates a new scanner module
func NewModule(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager) *Module {
	return &Module{
		db:            db,
		eventBus:      eventBus,
		pluginManager: pluginManager,
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
	
	// Create scanner manager
	m.scannerManager = scanner.NewManager(m.db, m.eventBus, m.pluginManager)
	
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
		
		m.scannerManager = scanner.NewManager(m.db, m.eventBus, m.pluginManager)
		logger.Info("Re-initialized scanner manager: %v", m.scannerManager)
		
		// Double-check that the manager was created properly
		if m.scannerManager == nil {
			logger.Error("CRITICAL: Failed to create scanner manager")
			return nil
		}
	}
	
	// Additional safety checks to ensure the manager is properly initialized
	if m.scannerManager != nil {
		// Use reflection or check internal state if possible, but for now just validate it's not obviously broken
		// We can't access private fields directly, but we can try a basic operation
		if m.scannerManager.GetActiveScanCount() < 0 {
			// This should never be negative, indicates a problem
			logger.Error("Scanner manager appears to be corrupted, reinitializing...")
			m.scannerManager = scanner.NewManager(m.db, m.eventBus, m.pluginManager)
		}
	}
	
	return m.scannerManager
}

// SetPluginManager sets the plugin manager for the scanner module
func (m *Module) SetPluginManager(pluginMgr plugins.Manager) {
	m.pluginManager = pluginMgr
	
	// If scanner manager already exists, we need to recreate it with the plugin manager
	if m.scannerManager != nil {
		logger.Info("Recreating scanner manager with plugin manager")
		// Update plugin manager in existing scanner manager
		m.scannerManager.SetPluginManager(pluginMgr)
	}
}

// Register registers this module with the module system
func Register() {
	// Create module without dependencies - they will be initialized later
	module := &Module{
		db:            nil, // Will be set during Init()
		eventBus:      nil, // Will be set during Init()
		pluginManager: nil, // Will be set later via SetPluginManager()
	}
	modulemanager.Register(module)
}
