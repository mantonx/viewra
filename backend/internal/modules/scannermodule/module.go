package scannermodule

import (
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
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
}

// NewModule creates a new scanner module
func NewModule(db *gorm.DB, eventBus events.EventBus) *Module {
	return &Module{
		db:       db,
		eventBus: eventBus,
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
	m.scannerManager = scanner.NewManager(m.db, m.eventBus)
	
	if m.scannerManager == nil {
		logger.Error("Failed to initialize scanner manager")
		return fmt.Errorf("failed to initialize scanner manager")
	}
	
	logger.Info("Scanner module initialized successfully with manager: %v", m.scannerManager)
	
	return nil
}

// GetScannerManager returns the underlying scanner manager
func (m *Module) GetScannerManager() *scanner.Manager {
	if m.scannerManager == nil {
		logger.Error("ScannerManager is nil in GetScannerManager()")
		
		// Try to re-initialize
		if m.db == nil {
			m.db = database.GetDB()
		}
		
		if m.eventBus == nil {
			m.eventBus = events.GetGlobalEventBus()
		}
		
		m.scannerManager = scanner.NewManager(m.db, m.eventBus)
		logger.Info("Re-initialized scanner manager: %v", m.scannerManager)
	}
	
	return m.scannerManager
}

// Register registers this module with the module system
func Register() {
	// Get database connection
	db := database.GetDB()
	
	// Get event bus (possibly from global event bus)
	bus := events.GetGlobalEventBus()
	
	// Create and register module
	module := NewModule(db, bus)
	modulemanager.Register(module)
}
