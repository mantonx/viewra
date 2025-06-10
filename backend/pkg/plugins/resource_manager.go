package plugins

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ResourceManager provides centralized resource management for plugins
// to prevent memory leaks and ensure proper cleanup
type ResourceManager struct {
	mutex     sync.RWMutex
	pluginID  string
	resources map[string]ManagedResource
	ctx       context.Context
	cancel    context.CancelFunc

	// Resource limits
	maxMemoryBytes     int64
	maxGoroutines      int
	maxFileDescriptors int

	// Monitoring
	startTime      time.Time
	lastCleanup    time.Time
	cleanupCounter int64
}

// ManagedResource represents a resource that can be cleaned up
type ManagedResource interface {
	// Cleanup should release all resources held by this resource
	Cleanup() error

	// ResourceType returns a string describing the resource type
	ResourceType() string

	// EstimatedMemoryUsage returns the estimated memory usage in bytes
	EstimatedMemoryUsage() int64

	// IsActive returns whether the resource is currently active
	IsActive() bool
}

// ResourceManagerConfig configures the resource manager
type ResourceManagerConfig struct {
	PluginID           string        `json:"plugin_id"`
	MaxMemoryBytes     int64         `json:"max_memory_bytes"`     // 256MB default
	MaxGoroutines      int           `json:"max_goroutines"`       // 100 default
	MaxFileDescriptors int           `json:"max_file_descriptors"` // 50 default
	CleanupInterval    time.Duration `json:"cleanup_interval"`     // 5 minutes default
	ForceCleanupAfter  time.Duration `json:"force_cleanup_after"`  // 30 minutes default
}

// NewResourceManager creates a new resource manager for a plugin
func NewResourceManager(config ResourceManagerConfig) *ResourceManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Set defaults
	if config.MaxMemoryBytes == 0 {
		config.MaxMemoryBytes = 256 * 1024 * 1024 // 256MB
	}
	if config.MaxGoroutines == 0 {
		config.MaxGoroutines = 100
	}
	if config.MaxFileDescriptors == 0 {
		config.MaxFileDescriptors = 50
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rm := &ResourceManager{
		pluginID:           config.PluginID,
		resources:          make(map[string]ManagedResource),
		ctx:                ctx,
		cancel:             cancel,
		maxMemoryBytes:     config.MaxMemoryBytes,
		maxGoroutines:      config.MaxGoroutines,
		maxFileDescriptors: config.MaxFileDescriptors,
		startTime:          time.Now(),
		lastCleanup:        time.Now(),
	}

	// Start background cleanup routine
	go rm.cleanupRoutine(config.CleanupInterval, config.ForceCleanupAfter)

	return rm
}

// RegisterResource registers a resource for management
func (rm *ResourceManager) RegisterResource(name string, resource ManagedResource) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.resources[name]; exists {
		return fmt.Errorf("resource %s already registered", name)
	}

	rm.resources[name] = resource
	return nil
}

// UnregisterResource unregisters and cleans up a resource
func (rm *ResourceManager) UnregisterResource(name string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	resource, exists := rm.resources[name]
	if !exists {
		return fmt.Errorf("resource %s not found", name)
	}

	// Cleanup the resource
	if err := resource.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup resource %s: %w", name, err)
	}

	delete(rm.resources, name)
	return nil
}

// GetResourceUsage returns current resource usage statistics
func (rm *ResourceManager) GetResourceUsage() *ResourceUsage {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var totalMemory int64
	activeResources := 0
	resourceTypes := make(map[string]int)

	for _, resource := range rm.resources {
		if resource.IsActive() {
			activeResources++
			totalMemory += resource.EstimatedMemoryUsage()
		}
		resourceTypes[resource.ResourceType()]++
	}

	// Get current goroutine count
	goroutineCount := runtime.NumGoroutine()

	return &ResourceUsage{
		PluginID:        rm.pluginID,
		TotalResources:  len(rm.resources),
		ActiveResources: activeResources,
		MemoryUsage:     totalMemory,
		GoroutineCount:  goroutineCount,
		ResourceTypes:   resourceTypes,
		Uptime:          time.Since(rm.startTime),
		LastCleanup:     rm.lastCleanup,
		CleanupCount:    rm.cleanupCounter,
	}
}

// ForceCleanup forces cleanup of all inactive resources
func (rm *ResourceManager) ForceCleanup() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	var errors []error
	var cleanedCount int

	for name, resource := range rm.resources {
		if !resource.IsActive() {
			if err := resource.Cleanup(); err != nil {
				errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", name, err))
			} else {
				delete(rm.resources, name)
				cleanedCount++
			}
		}
	}

	rm.lastCleanup = time.Now()
	rm.cleanupCounter++

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors (cleaned %d resources)", len(errors), cleanedCount)
	}

	return nil
}

// IsWithinLimits checks if current resource usage is within configured limits
func (rm *ResourceManager) IsWithinLimits() (bool, []string) {
	usage := rm.GetResourceUsage()
	var violations []string

	if usage.MemoryUsage > rm.maxMemoryBytes {
		violations = append(violations, fmt.Sprintf("memory usage %d exceeds limit %d",
			usage.MemoryUsage, rm.maxMemoryBytes))
	}

	if usage.GoroutineCount > rm.maxGoroutines {
		violations = append(violations, fmt.Sprintf("goroutine count %d exceeds limit %d",
			usage.GoroutineCount, rm.maxGoroutines))
	}

	return len(violations) == 0, violations
}

// Shutdown gracefully shuts down the resource manager
func (rm *ResourceManager) Shutdown() error {
	rm.cancel() // Stop cleanup routine

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	var errors []error

	// Cleanup all remaining resources
	for name, resource := range rm.resources {
		if err := resource.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s during shutdown: %w", name, err))
		}
	}

	rm.resources = make(map[string]ManagedResource) // Clear all resources

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors", len(errors))
	}

	return nil
}

// cleanupRoutine runs periodic cleanup of inactive resources
func (rm *ResourceManager) cleanupRoutine(interval, forceAfter time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.performRoutineCleanup(forceAfter)
		}
	}
}

// performRoutineCleanup performs routine cleanup of resources
func (rm *ResourceManager) performRoutineCleanup(forceAfter time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	now := time.Now()
	var cleanedCount int

	for name, resource := range rm.resources {
		shouldCleanup := !resource.IsActive()

		// Force cleanup of resources that have been inactive for too long
		if !shouldCleanup && now.Sub(rm.lastCleanup) > forceAfter {
			shouldCleanup = true
		}

		if shouldCleanup {
			if err := resource.Cleanup(); err != nil {
				// Log error but continue with other resources
				continue
			}
			delete(rm.resources, name)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		rm.lastCleanup = now
		rm.cleanupCounter++
	}

	// Force GC if memory usage is high
	usage := rm.getCurrentMemoryUsage()
	if usage > rm.maxMemoryBytes*80/100 { // 80% threshold
		runtime.GC()
	}
}

// getCurrentMemoryUsage calculates current memory usage
func (rm *ResourceManager) getCurrentMemoryUsage() int64 {
	var total int64
	for _, resource := range rm.resources {
		total += resource.EstimatedMemoryUsage()
	}
	return total
}

// ResourceUsage represents current resource usage statistics
type ResourceUsage struct {
	PluginID        string         `json:"plugin_id"`
	TotalResources  int            `json:"total_resources"`
	ActiveResources int            `json:"active_resources"`
	MemoryUsage     int64          `json:"memory_usage"`
	GoroutineCount  int            `json:"goroutine_count"`
	ResourceTypes   map[string]int `json:"resource_types"`
	Uptime          time.Duration  `json:"uptime"`
	LastCleanup     time.Time      `json:"last_cleanup"`
	CleanupCount    int64          `json:"cleanup_count"`
}

// CommonResource types that plugins can use

// CacheResource represents a managed cache resource
type CacheResource struct {
	name        string
	cache       map[string]interface{}
	mutex       sync.RWMutex
	lastAccess  time.Time
	estimatedMB int64
}

// NewCacheResource creates a new cache resource
func NewCacheResource(name string, estimatedMB int64) *CacheResource {
	return &CacheResource{
		name:        name,
		cache:       make(map[string]interface{}),
		lastAccess:  time.Now(),
		estimatedMB: estimatedMB,
	}
}

func (cr *CacheResource) Cleanup() error {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	cr.cache = make(map[string]interface{})
	return nil
}

func (cr *CacheResource) ResourceType() string {
	return "cache"
}

func (cr *CacheResource) EstimatedMemoryUsage() int64 {
	return cr.estimatedMB * 1024 * 1024
}

func (cr *CacheResource) IsActive() bool {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	// Consider active if accessed within last hour
	return time.Since(cr.lastAccess) < time.Hour
}

// HTTPClientResource represents a managed HTTP client resource
type HTTPClientResource struct {
	name        string
	client      interface{} // http.Client or similar
	active      bool
	lastUsed    time.Time
	connections int
}

func NewHTTPClientResource(name string, client interface{}) *HTTPClientResource {
	return &HTTPClientResource{
		name:     name,
		client:   client,
		active:   true,
		lastUsed: time.Now(),
	}
}

func (hcr *HTTPClientResource) Cleanup() error {
	hcr.active = false
	hcr.client = nil
	return nil
}

func (hcr *HTTPClientResource) ResourceType() string {
	return "http_client"
}

func (hcr *HTTPClientResource) EstimatedMemoryUsage() int64 {
	// Estimate based on connection pool size
	return int64(hcr.connections * 4096) // 4KB per connection estimate
}

func (hcr *HTTPClientResource) IsActive() bool {
	return hcr.active && time.Since(hcr.lastUsed) < 30*time.Minute
}
