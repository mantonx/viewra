// Package services provides a service registry for decoupled module communication
package services

import (
	"fmt"
	"sync"
)

// Registry manages service registrations and lookups
type Registry struct {
	services map[string]interface{}
	mu       sync.RWMutex
}

// Global registry instance
var globalRegistry = &Registry{
	services: make(map[string]interface{}),
}

// Register registers a service with the given name
func Register(name string, service interface{}) error {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.services[name]; exists {
		return fmt.Errorf("service %s already registered", name)
	}

	globalRegistry.services[name] = service
	return nil
}

// Get retrieves a service by name
func Get(name string) (interface{}, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	service, exists := globalRegistry.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}

	return service, nil
}

// GetPlaybackService retrieves the playback service
func GetPlaybackService() (PlaybackService, error) {
	service, err := Get("playback")
	if err != nil {
		return nil, err
	}

	ps, ok := service.(PlaybackService)
	if !ok {
		return nil, fmt.Errorf("service playback does not implement PlaybackService interface")
	}

	return ps, nil
}

// GetTranscodingService retrieves the transcoding service
func GetTranscodingService() (TranscodingService, error) {
	service, err := Get("transcoding")
	if err != nil {
		return nil, err
	}

	ts, ok := service.(TranscodingService)
	if !ok {
		return nil, fmt.Errorf("service transcoding does not implement TranscodingService interface")
	}

	return ts, nil
}

// GetPluginService retrieves the plugin service
func GetPluginService() (PluginService, error) {
	service, err := Get("plugin")
	if err != nil {
		return nil, err
	}

	ps, ok := service.(PluginService)
	if !ok {
		return nil, fmt.Errorf("service plugin does not implement PluginService interface")
	}

	return ps, nil
}

// GetMediaService retrieves the media service
func GetMediaService() (MediaService, error) {
	service, err := Get("media")
	if err != nil {
		return nil, err
	}

	ms, ok := service.(MediaService)
	if !ok {
		return nil, fmt.Errorf("service media does not implement MediaService interface")
	}

	return ms, nil
}

// GetScannerService retrieves the scanner service
func GetScannerService() (ScannerService, error) {
	service, err := Get("scanner")
	if err != nil {
		return nil, err
	}

	ss, ok := service.(ScannerService)
	if !ok {
		return nil, fmt.Errorf("service scanner does not implement ScannerService interface")
	}

	return ss, nil
}

// GetAssetService retrieves the asset service
func GetAssetService() (AssetService, error) {
	service, err := Get("asset")
	if err != nil {
		return nil, err
	}

	as, ok := service.(AssetService)
	if !ok {
		return nil, fmt.Errorf("service asset does not implement AssetService interface")
	}

	return as, nil
}

// GetEnrichmentService retrieves the enrichment service
func GetEnrichmentService() (EnrichmentService, error) {
	service, err := Get("enrichment")
	if err != nil {
		return nil, err
	}

	es, ok := service.(EnrichmentService)
	if !ok {
		return nil, fmt.Errorf("service enrichment does not implement EnrichmentService interface")
	}

	return es, nil
}

// List returns all registered service names
func List() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	names := make([]string, 0, len(globalRegistry.services))
	for name := range globalRegistry.services {
		names = append(names, name)
	}

	return names
}

// Clear removes all registered services (mainly for testing)
func Clear() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.services = make(map[string]interface{})
}
