package databasemodule

import (
	"fmt"
	"sync"

	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// ModelRegistry manages model registration from other modules
type ModelRegistry struct {
	models    []interface{}
	callbacks map[string][]func(interface{})
	mu        sync.RWMutex
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:    make([]interface{}, 0),
		callbacks: make(map[string][]func(interface{})),
	}
}

// Initialize sets up the model registry
func (mr *ModelRegistry) Initialize() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	logger.Info("Initializing model registry")
	
	// Model registry doesn't need specific initialization
	// but we keep this for consistency
	
	logger.Info("Model registry initialized successfully")
	return nil
}

// RegisterModel registers a model with the registry
func (mr *ModelRegistry) RegisterModel(model interface{}) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	// Check if model is already registered
	for _, existing := range mr.models {
		if existing == model {
			return fmt.Errorf("model already registered")
		}
	}
	
	// Add model to registry
	mr.models = append(mr.models, model)
	
	// Trigger callbacks
	mr.triggerCallbacks("register", model)
	
	logger.Info("Registered model: %T", model)
	return nil
}

// UnregisterModel removes a model from the registry
func (mr *ModelRegistry) UnregisterModel(model interface{}) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	// Find and remove the model
	for i, existing := range mr.models {
		if existing == model {
			// Remove from slice
			mr.models = append(mr.models[:i], mr.models[i+1:]...)
			
			// Trigger callbacks
			mr.triggerCallbacks("unregister", model)
			
			logger.Info("Unregistered model: %T", model)
			return nil
		}
	}
	
	return fmt.Errorf("model not found in registry")
}

// GetModels returns all registered models
func (mr *ModelRegistry) GetModels() []interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	// Return a copy to prevent external modification
	models := make([]interface{}, len(mr.models))
	copy(models, mr.models)
	return models
}

// GetModelCount returns the number of registered models
func (mr *ModelRegistry) GetModelCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return len(mr.models)
}

// AutoMigrateAll performs auto-migration for all registered models
func (mr *ModelRegistry) AutoMigrateAll(db *gorm.DB) error {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	if len(mr.models) == 0 {
		logger.Info("No models registered for auto-migration")
		return nil
	}
	
	logger.Info("Auto-migrating %d registered models", len(mr.models))
	
	// Perform auto-migration for all models
	if err := db.AutoMigrate(mr.models...); err != nil {
		return fmt.Errorf("failed to auto-migrate models: %w", err)
	}
	
	logger.Info("Successfully auto-migrated all registered models")
	return nil
}

// RegisterCallback registers a callback for model events
func (mr *ModelRegistry) RegisterCallback(event string, callback func(interface{})) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	if mr.callbacks[event] == nil {
		mr.callbacks[event] = make([]func(interface{}), 0)
	}
	
	mr.callbacks[event] = append(mr.callbacks[event], callback)
	logger.Debug("Registered callback for event: %s", event)
}

// triggerCallbacks triggers all callbacks for a specific event
func (mr *ModelRegistry) triggerCallbacks(event string, model interface{}) {
	callbacks := mr.callbacks[event]
	for _, callback := range callbacks {
		go func(cb func(interface{})) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Callback panic for event %s: %v", event, r)
				}
			}()
			cb(model)
		}(callback)
	}
}

// ValidateModel validates a model structure
func (mr *ModelRegistry) ValidateModel(model interface{}) error {
	// Basic validation - ensure model is not nil
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}
	
	// TODO: Add more sophisticated validation
	// - Check for required GORM tags
	// - Validate field types
	// - Check for circular references
	
	return nil
}

// GetModelInfo returns information about registered models
func (mr *ModelRegistry) GetModelInfo() []map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	info := make([]map[string]interface{}, 0, len(mr.models))
	
	for _, model := range mr.models {
		modelInfo := map[string]interface{}{
			"type": fmt.Sprintf("%T", model),
			"name": getModelName(model),
		}
		info = append(info, modelInfo)
	}
	
	return info
}

// getModelName extracts the model name from the type
func getModelName(model interface{}) string {
	// Simple implementation - just use the type name
	// TODO: Could be enhanced to use struct tags or other metadata
	return fmt.Sprintf("%T", model)
}

// Clear removes all registered models (useful for testing)
func (mr *ModelRegistry) Clear() {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	
	count := len(mr.models)
	mr.models = mr.models[:0] // Clear slice but keep capacity
	
	logger.Info("Cleared %d models from registry", count)
}

// HasModel checks if a specific model is registered
func (mr *ModelRegistry) HasModel(model interface{}) bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	for _, existing := range mr.models {
		if existing == model {
			return true
		}
	}
	
	return false
}

// GetStats returns model registry statistics
func (mr *ModelRegistry) GetStats() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_models":      len(mr.models),
		"registered_models": mr.GetModelInfo(),
		"callback_events":   len(mr.callbacks),
	}
	
	// Count callbacks per event
	callbackCounts := make(map[string]int)
	for event, callbacks := range mr.callbacks {
		callbackCounts[event] = len(callbacks)
	}
	stats["callbacks_per_event"] = callbackCounts
	
	return stats
}
