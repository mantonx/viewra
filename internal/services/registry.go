package services

import (
	"fmt"
	"sync"
)

// ServiceRegistry provides a clean architectural pattern for inter-module communication
//
// This pattern should be used by all modules to expose their functionality:
//
// 1. Define a clean interface for your module's public API
// 2. Register your service implementation during module initialization  
// 3. Other modules access your service through the registry, not direct imports
//
// Benefits:
// - Clean separation of concerns
// - Testable with interface mocking
// - No circular dependencies
// - Clear API boundaries
// - Consistent pattern across all modules
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]interface{}
}

var globalRegistry = &ServiceRegistry{
	services: make(map[string]interface{}),
}

// RegisterService registers a service with the given name
func RegisterService[T any](name string, service T) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	
	globalRegistry.services[name] = service
}

// GetService retrieves a service by name with type safety
func GetService[T any](name string) (T, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	
	var zero T
	
	service, exists := globalRegistry.services[name]
	if !exists {
		return zero, fmt.Errorf("service '%s' not found", name)
	}
	
	typedService, ok := service.(T)
	if !ok {
		return zero, fmt.Errorf("service '%s' has wrong type", name)
	}
	
	return typedService, nil
}

// MustGetService retrieves a service and panics if not found (for initialization)
func MustGetService[T any](name string) T {
	service, err := GetService[T](name)
	if err != nil {
		panic(fmt.Sprintf("Required service not available: %v", err))
	}
	return service
}

// ListServices returns all registered service names
func ListServices() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	
	names := make([]string, 0, len(globalRegistry.services))
	for name := range globalRegistry.services {
		names = append(names, name)
	}
	return names
}