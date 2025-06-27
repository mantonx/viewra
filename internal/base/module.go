package base

import (
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// BaseModule provides common functionality for all modules
// This eliminates 200+ lines of duplicate code across modules
type BaseModule struct {
	id          string
	name        string
	version     string
	core        bool
	initialized bool
	db          *gorm.DB
	eventBus    events.EventBus
	mu          sync.RWMutex
}

// NewBaseModule creates a new base module with common properties
func NewBaseModule(id, name, version string, core bool) *BaseModule {
	return &BaseModule{
		id:      id,
		name:    name,
		version: version,
		core:    core,
	}
}

// Common Module interface implementations
func (m *BaseModule) ID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id
}

func (m *BaseModule) Name() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.name
}

func (m *BaseModule) Version() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.version
}

func (m *BaseModule) Core() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.core
}

func (m *BaseModule) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// SetInitialized marks the module as initialized
func (m *BaseModule) SetInitialized(initialized bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = initialized
}

// SetDB sets the database connection
func (m *BaseModule) SetDB(db *gorm.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.db = db
}

// GetDB returns the database connection
func (m *BaseModule) GetDB() *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// SetEventBus sets the event bus
func (m *BaseModule) SetEventBus(eventBus events.EventBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBus = eventBus
}

// GetEventBus returns the event bus
func (m *BaseModule) GetEventBus() events.EventBus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.eventBus
}

// BaseRouteRegistrar provides common route registration utilities
type BaseRouteRegistrar struct {
	basePath string
	module   *BaseModule
}

// NewBaseRouteRegistrar creates a new route registrar
func NewBaseRouteRegistrar(basePath string, module *BaseModule) *BaseRouteRegistrar {
	return &BaseRouteRegistrar{
		basePath: basePath,
		module:   module,
	}
}

// RegisterRoutes is a helper for common route registration patterns
func (r *BaseRouteRegistrar) RegisterRoutes(router *gin.Engine, routes func(*gin.RouterGroup)) {
	if !r.module.IsInitialized() {
		logger.Warn("Skipping route registration for uninitialized module: %s", r.module.Name())
		return
	}

	api := router.Group(r.basePath)
	routes(api)

	logger.Info("Routes registered for module: %s", r.module.Name())
}

// HealthCheck provides a common health check implementation
func (m *BaseModule) HealthCheck() error {
	if !m.IsInitialized() {
		return ErrModuleNotInitialized
	}

	// Check database connection if available
	if db := m.GetDB(); db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return ErrDatabaseConnection
		}
		if err := sqlDB.Ping(); err != nil {
			return ErrDatabasePing
		}
	}

	return nil
}

// Common errors
var (
	ErrModuleNotInitialized = &ModuleError{Code: "MODULE_NOT_INITIALIZED", Message: "Module is not initialized"}
	ErrDatabaseConnection   = &ModuleError{Code: "DATABASE_CONNECTION", Message: "Failed to get database connection"}
	ErrDatabasePing         = &ModuleError{Code: "DATABASE_PING", Message: "Database ping failed"}
)

// ModuleError provides structured error handling
type ModuleError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ModuleError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ModuleError) Unwrap() error {
	return e.Cause
}

// NewModuleError creates a new module error with optional cause
func NewModuleError(code, message string, cause error) *ModuleError {
	return &ModuleError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
