package plugins

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// WarmPool manages plugin lifecycle to optimize startup latency.
// It keeps frequently-used plugins running and shuts down idle plugins.
type WarmPool struct {
	manager *Manager
	logger  *slog.Logger

	// lastAccess tracks when each plugin was last used.
	lastAccess map[string]time.Time

	// keepWarm lists plugins that should never be shut down.
	keepWarm map[string]bool

	// warmTimeout is how long a plugin stays warm after last use.
	warmTimeout time.Duration

	// checkInterval is how often to check for idle plugins.
	checkInterval time.Duration

	mu     sync.RWMutex
	stopCh chan struct{}
}

// WarmPoolConfig configures the warm pool behavior.
type WarmPoolConfig struct {
	// WarmTimeout is how long a plugin stays warm after last use.
	// Default: 30 minutes.
	WarmTimeout time.Duration

	// CheckInterval is how often to check for idle plugins.
	// Default: 1 minute.
	CheckInterval time.Duration

	// KeepWarm lists plugin IDs that should always stay running.
	KeepWarm []string
}

// NewWarmPool creates a new warm pool for managing plugin lifecycle.
func NewWarmPool(manager *Manager, cfg WarmPoolConfig, logger *slog.Logger) *WarmPool {
	if cfg.WarmTimeout == 0 {
		cfg.WarmTimeout = 30 * time.Minute
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 1 * time.Minute
	}

	keepWarm := make(map[string]bool)
	for _, id := range cfg.KeepWarm {
		keepWarm[id] = true
	}

	return &WarmPool{
		manager:       manager,
		logger:        logger,
		lastAccess:    make(map[string]time.Time),
		keepWarm:      keepWarm,
		warmTimeout:   cfg.WarmTimeout,
		checkInterval: cfg.CheckInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background goroutine that manages plugin lifecycle.
func (p *WarmPool) Start(ctx context.Context) {
	ticker := time.NewTicker(p.checkInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				p.checkIdlePlugins(ctx)
			case <-p.stopCh:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the warm pool management.
func (p *WarmPool) Stop() {
	close(p.stopCh)
}

// Touch marks a plugin as recently used.
// Call this whenever a plugin is accessed.
func (p *WarmPool) Touch(pluginID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAccess[pluginID] = time.Now()
}

// Prewarm starts plugins that will be needed for a scan.
// Call this before starting a library scan.
func (p *WarmPool) Prewarm(ctx context.Context, pluginIDs []string) error {
	p.logger.Info("prewarming plugins", "count", len(pluginIDs))

	var wg sync.WaitGroup
	errCh := make(chan error, len(pluginIDs))

	for _, id := range pluginIDs {
		wg.Add(1)
		go func(pluginID string) {
			defer wg.Done()

			// Check if already loaded
			if _, ok := p.manager.GetPlugin(pluginID); ok {
				p.Touch(pluginID)
				return
			}

			// Try to load from discovered plugins
			paths, err := p.manager.DiscoverPlugins()
			if err != nil {
				p.logger.Warn("failed to discover plugins", "error", err)
				return
			}

			for _, path := range paths {
				// Extract plugin ID from path (last component)
				// This is a simple heuristic - might need adjustment
				if containsPluginID(path, pluginID) {
					if _, err := p.manager.LoadPlugin(ctx, path); err != nil {
						errCh <- err
						return
					}
					p.Touch(pluginID)
					p.logger.Info("prewarmed plugin", "plugin", pluginID)
					return
				}
			}
		}(id)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors (non-fatal)
	for err := range errCh {
		p.logger.Warn("prewarm error", "error", err)
	}

	return nil
}

// SetKeepWarm marks a plugin to always stay running.
func (p *WarmPool) SetKeepWarm(pluginID string, keepWarm bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if keepWarm {
		p.keepWarm[pluginID] = true
	} else {
		delete(p.keepWarm, pluginID)
	}
}

// IsWarm returns true if the plugin is currently loaded and warm.
func (p *WarmPool) IsWarm(pluginID string) bool {
	_, ok := p.manager.GetPlugin(pluginID)
	return ok
}

// GetWarmPlugins returns a list of currently warm plugin IDs.
func (p *WarmPool) GetWarmPlugins() []string {
	plugins := p.manager.ListPlugins()
	ids := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		ids = append(ids, plugin.ID)
	}
	return ids
}

// checkIdlePlugins shuts down plugins that have been idle too long.
func (p *WarmPool) checkIdlePlugins(ctx context.Context) {
	now := time.Now()
	var toUnload []string

	plugins := p.manager.ListPlugins()

	p.mu.Lock()
	for _, plugin := range plugins {
		// Skip keep-warm plugins
		if p.keepWarm[plugin.ID] {
			continue
		}

		// Check last access time
		lastAccess, ok := p.lastAccess[plugin.ID]
		if !ok {
			// Never accessed, use load time (give it warmTimeout to be used)
			lastAccess = now.Add(-p.warmTimeout / 2)
			p.lastAccess[plugin.ID] = lastAccess
		}

		if now.Sub(lastAccess) > p.warmTimeout {
			toUnload = append(toUnload, plugin.ID)
		}
	}
	p.mu.Unlock()

	// Unload idle plugins
	for _, id := range toUnload {
		p.logger.Info("unloading idle plugin", "plugin", id)
		if err := p.manager.UnloadPlugin(ctx, id); err != nil {
			p.logger.Warn("failed to unload idle plugin", "plugin", id, "error", err)
		} else {
			p.mu.Lock()
			delete(p.lastAccess, id)
			p.mu.Unlock()
		}
	}
}

// GetOrLoadEnricher returns an enricher plugin, loading it if necessary.
// This is the primary method for getting an enricher with warm pool management.
func (p *WarmPool) GetOrLoadEnricher(ctx context.Context, pluginID string) (*PluginInstance, error) {
	// Check if already loaded
	if instance, ok := p.manager.GetPlugin(pluginID); ok {
		p.Touch(pluginID)
		return instance, nil
	}

	// Try to load
	paths, err := p.manager.DiscoverPlugins()
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		if containsPluginID(path, pluginID) {
			instance, err := p.manager.LoadPlugin(ctx, path)
			if err != nil {
				return nil, err
			}
			p.Touch(pluginID)
			return instance, nil
		}
	}

	return nil, &PluginNotFoundError{PluginID: pluginID}
}

// PluginNotFoundError indicates a plugin could not be found.
type PluginNotFoundError struct {
	PluginID string
}

func (e *PluginNotFoundError) Error() string {
	return "plugin not found: " + e.PluginID
}

// containsPluginID checks if a path likely contains the given plugin ID.
// This is a simple heuristic based on path components.
func containsPluginID(path, pluginID string) bool {
	// Simple check: the path should end with the plugin ID
	// e.g., /data/plugins/tmdb/tmdb or /data/plugins/tmdb
	return len(path) >= len(pluginID) &&
		(path[len(path)-len(pluginID):] == pluginID ||
			(len(path) > len(pluginID) && path[len(path)-len(pluginID)-1:] == "/"+pluginID))
}

// WarmPoolStats contains statistics about the warm pool.
type WarmPoolStats struct {
	WarmCount     int
	KeepWarmCount int
	IdleCount     int
}

// Stats returns current warm pool statistics.
func (p *WarmPool) Stats() WarmPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	plugins := p.manager.ListPlugins()
	now := time.Now()

	stats := WarmPoolStats{
		WarmCount:     len(plugins),
		KeepWarmCount: len(p.keepWarm),
	}

	for _, plugin := range plugins {
		if p.keepWarm[plugin.ID] {
			continue
		}
		lastAccess, ok := p.lastAccess[plugin.ID]
		if !ok || now.Sub(lastAccess) > p.warmTimeout/2 {
			stats.IdleCount++
		}
	}

	return stats
}
