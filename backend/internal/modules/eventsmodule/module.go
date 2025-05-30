package eventsmodule

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/server/handlers"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	ModuleID   = "system.events"
	ModuleName = "Event Management"
)

// Module handles advanced event management functionality
type Module struct {
	id          string
	name        string
	version     string
	core        bool
	db          *gorm.DB
	eventBus    events.EventBus
	initialized bool
	
	// Event handler for API routes
	eventsHandler   *handlers.EventsHandler
}

// Register registers this module with the module system
func Register() {
	eventsModule := &Module{
		id:      ModuleID,
		name:    ModuleName,
		version: "1.0.0",
		core:    true,
	}
	modulemanager.Register(eventsModule)
}

// ID returns the module ID
func (m *Module) ID() string {
	return m.id
}

// Name returns the module name
func (m *Module) Name() string {
	return m.name
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return m.core
}

// Migrate handles database schema migrations for events
func (m *Module) Migrate(db *gorm.DB) error {
	// Events module doesn't need additional migrations
	// Core event storage is handled by the events package
	return nil
}

// Init initializes the events module
func (m *Module) Init() error {
	if m.initialized {
		return nil
	}
	
	// Get dependencies
	m.eventBus = events.GetGlobalEventBus()
	if m.eventBus == nil {
		return fmt.Errorf("global event bus not initialized")
	}
	
	// Initialize event handler
	m.eventsHandler = handlers.NewEventsHandler(m.eventBus)
	
	m.initialized = true
	return nil
}

// RegisterRoutes registers API routes for event management
func (m *Module) RegisterRoutes(router *gin.Engine) {
	if !m.initialized {
		return
	}
	
	api := router.Group("/api/v1/events")
	{
		// Event querying
		api.GET("/", m.eventsHandler.GetEvents)
		api.GET("/range", m.eventsHandler.GetEventsByTimeRange)
		api.GET("/types", m.eventsHandler.GetEventTypes)
		
		// Event publishing (admin)
		api.POST("/", m.eventsHandler.PublishEvent)
		
		// Event management  
		api.DELETE("/:id", m.eventsHandler.DeleteEvent)
		api.POST("/clear", m.eventsHandler.ClearEvents)
		
		// Health check
		api.GET("/health", m.getEventHealth)
	}
}

// getEventHealth provides health check for the event bus
func (m *Module) getEventHealth(c *gin.Context) {
	health := m.eventBus.Health()
	if health != nil {
		c.JSON(500, gin.H{"healthy": false, "error": health.Error()})
		return
	}
	c.JSON(200, gin.H{"healthy": true})
} 