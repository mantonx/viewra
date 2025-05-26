package modulemanager

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// Module defines the interface that all modules must implement
type Module interface {
	ID() string        // Unique identifier for the module
	Name() string      // Display name for the module
	Core() bool        // Whether this is a core module (cannot be disabled)
	Migrate(db *gorm.DB) error // Run database migrations
	Init() error       // Initialize the module
}

// RouteRegistrar is an optional interface for modules that need to register routes
type RouteRegistrar interface {
	RegisterRoutes(router *gin.Engine)
}

// ModuleRegistry manages module registration and initialization
type ModuleRegistry struct {
	modules        map[string]Module
	disabledModules map[string]bool
	mu             sync.RWMutex
	initialized    bool
}

// Registry is the global module registry
var Registry = &ModuleRegistry{
	modules:        make(map[string]Module),
	disabledModules: make(map[string]bool),
}

// Register adds a module to the registry
func Register(m Module) {
	Registry.Register(m)
}

// Register adds a module to the registry
func (r *ModuleRegistry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		logger.Warn("Module %s (%s) registered after initialization", m.Name(), m.ID())
	}

	r.modules[m.ID()] = m
	logger.Info("📦 Module registered: %s (%s)", m.Name(), m.ID())
}

// LoadAll initializes all registered modules
func LoadAll(db *gorm.DB) error {
	return Registry.LoadAll(db)
}

// LoadAll initializes all registered modules
func (r *ModuleRegistry) LoadAll(db *gorm.DB) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		logger.Warn("Module system already initialized")
		return nil
	}

	// Load configuration
	configPath := GetDefaultConfigPath()
	config, err := LoadConfig(configPath)
	if err != nil {
		logger.Warn("Failed to load module config, using defaults: %v", err)
		config = &ModuleConfig{}
	}
	
	// Apply configuration - disable modules listed in config
	for _, moduleID := range config.Modules.Disabled {
		r.disabledModules[moduleID] = true
		logger.Info("Module disabled via configuration: %s", moduleID)
	}

	logger.Info("🔄 Loading %d modules...", len(r.modules))
	
	for id, module := range r.modules {
		// Skip disabled modules
		if r.isDisabled(id) {
			if module.Core() {
				return fmt.Errorf("attempted to disable core module: %s", id)
			}
			logger.Warn("⚠️ Skipping module %s (disabled)", module.Name())
			continue
		}

		logger.Info("📋 Initializing module: %s", module.Name())
		
		// Migrate module database schemas
		if err := module.Migrate(db); err != nil {
			return fmt.Errorf("failed to migrate %s: %w", module.Name(), err)
		}
		
		// Initialize the module
		if err := module.Init(); err != nil {
			return fmt.Errorf("failed to initialize %s: %w", module.Name(), err)
		}
		
		logger.Info("✅ Module loaded: %s", module.Name())
	}
	
	r.initialized = true
	return nil
}

// DisableModule marks a module as disabled (for development/testing only)
func DisableModule(id string) {
	Registry.DisableModule(id)
}

// DisableModule marks a module as disabled
func (r *ModuleRegistry) DisableModule(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	module, exists := r.modules[id]
	if !exists {
		logger.Warn("Attempted to disable non-existent module: %s", id)
		return
	}

	if module.Core() {
		logger.Error("Cannot disable core module: %s", id)
		return
	}

	r.disabledModules[id] = true
	logger.Info("Module disabled: %s", id)
}

// EnableModule enables a previously disabled module
func EnableModule(id string) {
	Registry.EnableModule(id)
}

// EnableModule enables a previously disabled module
func (r *ModuleRegistry) EnableModule(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.disabledModules, id)
	logger.Info("Module enabled: %s", id)
}

// isDisabled checks if a module is disabled
func (r *ModuleRegistry) isDisabled(id string) bool {
	return r.disabledModules[id]
}

// GetModule returns a module by ID
func GetModule(id string) (Module, bool) {
	return Registry.GetModule(id)
}

// GetModule returns a module by ID
func (r *ModuleRegistry) GetModule(id string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, exists := r.modules[id]
	return module, exists
}

// ListModules returns all registered modules
func ListModules() []Module {
	return Registry.ListModules()
}

// ListModules returns all registered modules
func (r *ModuleRegistry) ListModules() []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	modules := make([]Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

// ListCoreModules returns all core modules
func ListCoreModules() []Module {
	return Registry.ListCoreModules()
}

// ListCoreModules returns all core modules
func (r *ModuleRegistry) ListCoreModules() []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var coreModules []Module
	
	for _, module := range r.modules {
		if module.Core() {
			coreModules = append(coreModules, module)
		}
	}
	
	return coreModules
}

// RegisterRoutes registers routes for all modules that implement RouteRegistrar
func RegisterRoutes(router *gin.Engine) {
	Registry.RegisterRoutes(router)
}

// RegisterRoutes registers routes for all modules that implement RouteRegistrar
func (r *ModuleRegistry) RegisterRoutes(router *gin.Engine) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, module := range r.modules {
		if routeRegistrar, ok := module.(RouteRegistrar); ok {
			logger.Info("Registering routes for module: " + module.Name())
			routeRegistrar.RegisterRoutes(router)
		}
	}
}
