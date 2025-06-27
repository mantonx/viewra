// Package services provides lazy loading capabilities for service dependencies
package services

import (
	"fmt"
	"sync"
	"time"
)

// LazyService provides lazy loading for a service
type LazyService struct {
	name       string
	loader     func() (interface{}, error)
	service    interface{}
	mu         sync.RWMutex
	loaded     bool
	retryDelay time.Duration
	maxRetries int
}

// NewLazyService creates a new lazy service wrapper
func NewLazyService(name string, loader func() (interface{}, error)) *LazyService {
	return &LazyService{
		name:       name,
		loader:     loader,
		retryDelay: 100 * time.Millisecond,
		maxRetries: 10,
	}
}

// Get retrieves the service, loading it if necessary
func (ls *LazyService) Get() (interface{}, error) {
	// Fast path: already loaded
	ls.mu.RLock()
	if ls.loaded && ls.service != nil {
		service := ls.service
		ls.mu.RUnlock()
		return service, nil
	}
	ls.mu.RUnlock()

	// Slow path: need to load
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Double-check after acquiring write lock
	if ls.loaded && ls.service != nil {
		return ls.service, nil
	}

	// Try to load with retries
	var lastErr error
	for i := 0; i < ls.maxRetries; i++ {
		service, err := ls.loader()
		if err == nil && service != nil {
			ls.service = service
			ls.loaded = true
			return service, nil
		}

		lastErr = err

		// Wait before retry (except on last attempt)
		if i < ls.maxRetries-1 {
			time.Sleep(ls.retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to load service %s after %d attempts: %w", ls.name, ls.maxRetries, lastErr)
}

// Reset clears the cached service
func (ls *LazyService) Reset() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.service = nil
	ls.loaded = false
}

// IsLoaded returns whether the service has been loaded
func (ls *LazyService) IsLoaded() bool {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	return ls.loaded
}

// LazyServiceRegistry provides lazy loading for all services
type LazyServiceRegistry struct {
	services map[string]*LazyService
	mu       sync.RWMutex
}

// Global lazy registry
var lazyRegistry = &LazyServiceRegistry{
	services: make(map[string]*LazyService),
}

// RegisterLazy registers a lazy-loaded service
func RegisterLazy(name string, loader func() (interface{}, error)) {
	lazyRegistry.mu.Lock()
	defer lazyRegistry.mu.Unlock()

	lazyRegistry.services[name] = NewLazyService(name, loader)
}

// GetLazy retrieves a service using lazy loading
func GetLazy(name string) (interface{}, error) {
	lazyRegistry.mu.RLock()
	lazy, exists := lazyRegistry.services[name]
	lazyRegistry.mu.RUnlock()

	if !exists {
		// Fall back to regular registry
		return Get(name)
	}

	return lazy.Get()
}

// ResetLazy resets a lazy service
func ResetLazy(name string) {
	lazyRegistry.mu.RLock()
	lazy, exists := lazyRegistry.services[name]
	lazyRegistry.mu.RUnlock()

	if exists {
		lazy.Reset()
	}
}

// GetPlaybackServiceLazy retrieves the playback service with lazy loading
func GetPlaybackServiceLazy() (PlaybackService, error) {
	service, err := GetLazy("playback")
	if err != nil {
		return nil, err
	}

	ps, ok := service.(PlaybackService)
	if !ok {
		return nil, fmt.Errorf("service playback does not implement PlaybackService interface")
	}

	return ps, nil
}

// GetTranscodingServiceLazy retrieves the transcoding service with lazy loading
func GetTranscodingServiceLazy() (TranscodingService, error) {
	service, err := GetLazy("transcoding")
	if err != nil {
		return nil, err
	}

	ts, ok := service.(TranscodingService)
	if !ok {
		return nil, fmt.Errorf("service transcoding does not implement TranscodingService interface")
	}

	return ts, nil
}

// GetPluginServiceLazy retrieves the plugin service with lazy loading
func GetPluginServiceLazy() (PluginService, error) {
	service, err := GetLazy("plugin")
	if err != nil {
		return nil, err
	}

	ps, ok := service.(PluginService)
	if !ok {
		return nil, fmt.Errorf("service plugin does not implement PluginService interface")
	}

	return ps, nil
}

// GetMediaServiceLazy retrieves the media service with lazy loading
func GetMediaServiceLazy() (MediaService, error) {
	service, err := GetLazy("media")
	if err != nil {
		return nil, err
	}

	ms, ok := service.(MediaService)
	if !ok {
		return nil, fmt.Errorf("service media does not implement MediaService interface")
	}

	return ms, nil
}

// GetScannerServiceLazy retrieves the scanner service with lazy loading
func GetScannerServiceLazy() (ScannerService, error) {
	service, err := GetLazy("scanner")
	if err != nil {
		return nil, err
	}

	ss, ok := service.(ScannerService)
	if !ok {
		return nil, fmt.Errorf("service scanner does not implement ScannerService interface")
	}

	return ss, nil
}

// GetAssetServiceLazy retrieves the asset service with lazy loading
func GetAssetServiceLazy() (AssetService, error) {
	service, err := GetLazy("asset")
	if err != nil {
		return nil, err
	}

	as, ok := service.(AssetService)
	if !ok {
		return nil, fmt.Errorf("service asset does not implement AssetService interface")
	}

	return as, nil
}

// GetEnrichmentServiceLazy retrieves the enrichment service with lazy loading
func GetEnrichmentServiceLazy() (EnrichmentService, error) {
	service, err := GetLazy("enrichment")
	if err != nil {
		return nil, err
	}

	es, ok := service.(EnrichmentService)
	if !ok {
		return nil, fmt.Errorf("service enrichment does not implement EnrichmentService interface")
	}

	return es, nil
}

// RegisterServiceLoaders registers lazy loaders for all services
// This allows modules to retrieve services even if they haven't been registered yet
func RegisterServiceLoaders() {
	// Playback service loader
	RegisterLazy("playback", func() (interface{}, error) {
		return Get("playback")
	})

	// Transcoding service loader
	RegisterLazy("transcoding", func() (interface{}, error) {
		return Get("transcoding")
	})

	// Plugin service loader
	RegisterLazy("plugin", func() (interface{}, error) {
		return Get("plugin")
	})

	// Media service loader
	RegisterLazy("media", func() (interface{}, error) {
		return Get("media")
	})

	// Scanner service loader
	RegisterLazy("scanner", func() (interface{}, error) {
		return Get("scanner")
	})

	// Asset service loader
	RegisterLazy("asset", func() (interface{}, error) {
		return Get("asset")
	})

	// Enrichment service loader
	RegisterLazy("enrichment", func() (interface{}, error) {
		return Get("enrichment")
	})
}

// WaitForService waits for a service to become available
func WaitForService(name string, timeout time.Duration) (interface{}, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		service, err := Get(name)
		if err == nil && service != nil {
			return service, nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	return nil, fmt.Errorf("service %s not available after %v", name, timeout)
}

// WaitForAllServices waits for all required services to become available
func WaitForAllServices(services []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	pending := make(map[string]bool)
	for _, name := range services {
		pending[name] = true
	}

	for len(pending) > 0 && time.Now().Before(deadline) {
		for name := range pending {
			if _, err := Get(name); err == nil {
				delete(pending, name)
			}
		}

		if len(pending) > 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	if len(pending) > 0 {
		names := make([]string, 0, len(pending))
		for name := range pending {
			names = append(names, name)
		}
		return fmt.Errorf("services not available after %v: %v", timeout, names)
	}

	return nil
}
