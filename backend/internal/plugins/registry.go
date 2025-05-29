package plugins

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/mantonx/viewra/internal/database"
)

// MediaPluginRegistry manages media handler plugins with thread-safe operations
type MediaPluginRegistry struct {
	mu            sync.RWMutex
	plugins       []MediaHandlerPlugin
	corePlugins   []CoreMediaPlugin
	enabledOnly   bool
	hooks         []MediaScannerHook
}

// NewMediaPluginRegistry creates a new media plugin registry
func NewMediaPluginRegistry() *MediaPluginRegistry {
	return &MediaPluginRegistry{
		plugins:     make([]MediaHandlerPlugin, 0),
		corePlugins: make([]CoreMediaPlugin, 0),
		hooks:       make([]MediaScannerHook, 0),
		enabledOnly: true, // Only match enabled plugins by default
	}
}

// RegisterCorePlugin registers a core media plugin that's always available
func (r *MediaPluginRegistry) RegisterCorePlugin(plugin CoreMediaPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Initialize the plugin
	if err := plugin.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize core plugin %s: %w", plugin.GetName(), err)
	}
	
	// Add to both core and general plugin lists
	r.corePlugins = append(r.corePlugins, plugin)
	r.plugins = append(r.plugins, plugin)
	
	fmt.Printf("DEBUG: Registered core media plugin: %s (%s)\n", 
		plugin.GetName(), plugin.GetMediaType())
	return nil
}

// RegisterPlugin registers a regular media handler plugin
func (r *MediaPluginRegistry) RegisterPlugin(plugin MediaHandlerPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.plugins = append(r.plugins, plugin)
	fmt.Printf("DEBUG: Registered media plugin: %s (%s)\n", 
		plugin.GetName(), plugin.GetMediaType())
}

// UnregisterPlugin removes a plugin from the registry
func (r *MediaPluginRegistry) UnregisterPlugin(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Remove from plugins list
	for i, plugin := range r.plugins {
		if plugin.GetName() == pluginName {
			r.plugins = append(r.plugins[:i], r.plugins[i+1:]...)
			break
		}
	}
	
	// Remove from core plugins list if it's a core plugin
	for i, plugin := range r.corePlugins {
		if plugin.GetName() == pluginName {
			// Shutdown the core plugin
			if err := plugin.Shutdown(); err != nil {
				fmt.Printf("WARNING: Error shutting down core plugin %s: %v\n", pluginName, err)
			}
			r.corePlugins = append(r.corePlugins[:i], r.corePlugins[i+1:]...)
			break
		}
	}
	
	fmt.Printf("DEBUG: Unregistered media plugin: %s\n", pluginName)
}

// Match finds the first plugin that can handle the given file
func (r *MediaPluginRegistry) Match(path string, info fs.FileInfo) MediaHandlerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, plugin := range r.plugins {
		// Check if we should only consider enabled plugins
		if r.enabledOnly {
			if corePlugin, ok := plugin.(CoreMediaPlugin); ok {
				if !corePlugin.IsEnabled() {
					continue
				}
			}
		}
		
		if plugin.Match(path, info) {
			return plugin
		}
	}
	return nil
}

// GetPlugins returns all registered plugins
func (r *MediaPluginRegistry) GetPlugins() []MediaHandlerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent external modification
	plugins := make([]MediaHandlerPlugin, len(r.plugins))
	copy(plugins, r.plugins)
	return plugins
}

// GetCorePlugins returns all registered core plugins
func (r *MediaPluginRegistry) GetCorePlugins() []CoreMediaPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent external modification
	plugins := make([]CoreMediaPlugin, len(r.corePlugins))
	copy(plugins, r.corePlugins)
	return plugins
}

// GetPluginsByType returns plugins that handle a specific media type
func (r *MediaPluginRegistry) GetPluginsByType(mediaType string) []MediaHandlerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var matchedPlugins []MediaHandlerPlugin
	for _, plugin := range r.plugins {
		if plugin.GetMediaType() == mediaType {
			// Check if we should only consider enabled plugins
			if r.enabledOnly {
				if corePlugin, ok := plugin.(CoreMediaPlugin); ok {
					if !corePlugin.IsEnabled() {
						continue
					}
				}
			}
			matchedPlugins = append(matchedPlugins, plugin)
		}
	}
	return matchedPlugins
}

// GetPluginInfo returns information about all registered plugins
func (r *MediaPluginRegistry) GetPluginInfo() []MediaPluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var pluginInfos []MediaPluginInfo
	for _, plugin := range r.plugins {
		info := MediaPluginInfo{
			Name:          plugin.GetName(),
			MediaType:     plugin.GetMediaType(),
			SupportedExts: plugin.GetSupportedExtensions(),
			Description:   fmt.Sprintf("Handles %s files", plugin.GetMediaType()),
		}
		
		// Check if it's a core plugin
		if corePlugin, ok := plugin.(CoreMediaPlugin); ok {
			info.IsCore = true
			info.Enabled = corePlugin.IsEnabled()
		} else {
			info.IsCore = false
			info.Enabled = true // Regular plugins are always enabled
		}
		
		pluginInfos = append(pluginInfos, info)
	}
	return pluginInfos
}

// SetEnabledOnly configures whether to only match enabled plugins
func (r *MediaPluginRegistry) SetEnabledOnly(enabledOnly bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabledOnly = enabledOnly
}

// RegisterHook adds a media scanner hook
func (r *MediaPluginRegistry) RegisterHook(hook MediaScannerHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, hook)
}

// CallOnMediaFileScanned notifies all hooks that a media file has been scanned
func (r *MediaPluginRegistry) CallOnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, hook := range r.hooks {
		go func(h MediaScannerHook) {
			if err := h.OnMediaFileScanned(mediaFile, metadata); err != nil {
				fmt.Printf("WARNING: Media scanner hook failed: %v\n", err)
			}
		}(hook)
	}
}

// Shutdown gracefully shuts down all core plugins
func (r *MediaPluginRegistry) Shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for _, plugin := range r.corePlugins {
		if err := plugin.Shutdown(); err != nil {
			fmt.Printf("WARNING: Error shutting down core plugin %s: %v\n", plugin.GetName(), err)
		}
	}
	return nil
} 