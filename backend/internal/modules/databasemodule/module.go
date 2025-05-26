package databasemodule

import (
	"context"
	"fmt"
	"sync"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	// ModuleID is the unique identifier for the database module
	ModuleID = "system.database"
	
	// ModuleName is the display name for the database module
	ModuleName = "Database Manager"
)

// Module implements database functionality as a core module
type Module struct {
	db               *gorm.DB
	eventBus         events.EventBus
	connectionPool   *ConnectionPool
	migrationManager *MigrationManager
	transactionMgr   *TransactionManager
	modelRegistry    *ModelRegistry
	mu               sync.RWMutex
	initialized      bool
}



// NewModule creates a new database module
func NewModule(db *gorm.DB, eventBus events.EventBus) *Module {
	// Initialize migration manager
	migrationManager, err := NewMigrationManager(db)
	if err != nil {
		logger.Error("Failed to create migration manager: %v", err)
		// Use a nil migration manager and handle gracefully
	}
	
	return &Module{
		db:               db,
		eventBus:         eventBus,
		connectionPool:   NewConnectionPool(db),
		migrationManager: migrationManager,
		transactionMgr:   NewTransactionManager(db),
		modelRegistry:    NewModelRegistry(),
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
	return true // Database is a core module
}

// Migrate performs any necessary database migrations
func (m *Module) Migrate(db *gorm.DB) error {
	logger.Info("Migrating database module schema")
	
	// The database module itself doesn't need migrations as it manages the core database
	// But we ensure the system event table exists for database events
	if err := db.AutoMigrate(&database.SystemEvent{}); err != nil {
		return fmt.Errorf("failed to migrate system events table: %w", err)
	}
	
	return nil
}

// Init initializes the database module
func (m *Module) Init() error {
	logger.Info("Initializing database module")
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.initialized {
		return nil
	}
	
	// Initialize the module components if they're not already set
	if m.db == nil {
		m.db = database.GetDB()
		if m.db == nil {
			return fmt.Errorf("database connection is not available")
		}
	}
	
	if m.eventBus == nil {
		m.eventBus = events.GetGlobalEventBus()
	}
	
	// Initialize all sub-components now that we have a database connection
	if m.connectionPool == nil {
		m.connectionPool = NewConnectionPool(m.db)
	}
	
	if m.migrationManager == nil {
		var err error
		m.migrationManager, err = NewMigrationManager(m.db)
		if err != nil {
			return fmt.Errorf("failed to create migration manager: %w", err)
		}
	}
	
	if m.transactionMgr == nil {
		m.transactionMgr = NewTransactionManager(m.db)
	}
	
	if m.modelRegistry == nil {
		m.modelRegistry = NewModelRegistry()
	}
	
	// Initialize sub-components
	if err := m.connectionPool.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize connection pool: %w", err)
	}
	
	if err := m.migrationManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize migration manager: %w", err)
	}
	
	if err := m.transactionMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize transaction manager: %w", err)
	}
	
	if err := m.modelRegistry.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize model registry: %w", err)
	}
	
	// Publish database module initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			"database.module.initialized",
			"Database Module Initialized",
			"Database module has been successfully initialized",
		)
		m.eventBus.PublishAsync(initEvent)
	}
	
	m.initialized = true
	logger.Info("Database module initialized successfully")
	
	return nil
}

// GetConnectionPool returns the connection pool manager
func (m *Module) GetConnectionPool() *ConnectionPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionPool
}

// GetMigrationManager returns the migration manager
func (m *Module) GetMigrationManager() *MigrationManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.migrationManager
}

// GetTransactionManager returns the transaction manager
func (m *Module) GetTransactionManager() *TransactionManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transactionMgr
}

// GetModelRegistry returns the model registry
func (m *Module) GetModelRegistry() *ModelRegistry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.modelRegistry
}

// RegisterModel registers a model with the database module
func (m *Module) RegisterModel(model interface{}) error {
	return m.modelRegistry.RegisterModel(model)
}

// BeginTransaction starts a new database transaction
func (m *Module) BeginTransaction(ctx context.Context) (*TransactionContext, error) {
	return m.transactionMgr.BeginTransaction(ctx)
}

// Health performs a health check on the database module
func (m *Module) Health() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.initialized {
		return fmt.Errorf("database module not initialized")
	}
	
	// Test database connection
	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	// Check connection pool health
	if err := m.connectionPool.Health(); err != nil {
		return fmt.Errorf("connection pool health check failed: %w", err)
	}
	
	return nil
}

// Register registers this module with the module system
func Register() {
	// Create module without database connection - it will be initialized later
	module := &Module{}
	modulemanager.Register(module)
}
