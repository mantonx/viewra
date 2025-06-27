// Package modulemanager provides interfaces for the module system
package modulemanager

import (
	"context"
	"time"
)

// ServiceInjector is an optional interface for modules that need services injected
type ServiceInjector interface {
	// InjectServices is called after the dependency graph is built but before Init()
	// The services map contains all available services keyed by service name
	InjectServices(services map[string]interface{}) error
}

// ServiceRegistrar is an optional interface for modules that register services early
type ServiceRegistrar interface {
	// RegisterServices is called after construction but before any Init() calls
	// This allows modules to register services that other modules depend on
	RegisterServices() error
}

// HealthChecker is an optional interface for modules that can report health status
type HealthChecker interface {
	// HealthCheck returns the current health status of the module
	HealthCheck(ctx context.Context) HealthStatus
}

// HealthStatus represents the health of a module
type HealthStatus struct {
	Status      HealthState               `json:"status"`
	Message     string                    `json:"message,omitempty"`
	LastChecked time.Time                 `json:"last_checked"`
	Details     map[string]interface{}    `json:"details,omitempty"`
	Dependencies map[string]HealthState   `json:"dependencies,omitempty"`
}

// HealthState represents the state of a module's health
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateDegraded  HealthState = "degraded"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateUnknown   HealthState = "unknown"
)

// LifecycleHooks is an optional interface for modules that need lifecycle callbacks
type LifecycleHooks interface {
	// PreInit is called before Init for any early setup
	PreInit() error
	
	// PostInit is called after all modules have been initialized
	PostInit() error
	
	// PreShutdown is called before shutdown begins
	PreShutdown(ctx context.Context) error
	
	// PostShutdown is called after shutdown completes
	PostShutdown(ctx context.Context) error
}

// Restartable is an optional interface for modules that can be restarted
type Restartable interface {
	// CanRestart returns true if the module supports restart
	CanRestart() bool
	
	// Restart attempts to restart the module
	Restart(ctx context.Context) error
}

// ConfigReloadable is an optional interface for modules that can reload configuration
type ConfigReloadable interface {
	// ReloadConfig reloads the module's configuration without restart
	ReloadConfig() error
}