package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
)

// MetadataContext provides context for file processing plugins
type MetadataContext struct {
	DB        interface{}
	MediaFile interface{}
	LibraryID uint
	EventBus  interface{}
}

// FileHandlerPlugin defines the interface for file processing plugins
type FileHandlerPlugin interface {
	Match(path string, info fs.FileInfo) bool
	HandleFile(path string, ctx MetadataContext) error
	GetName() string
	GetSupportedExtensions() []string
}

// PluginRegistry manages file handler plugins
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins []FileHandlerPlugin
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make([]FileHandlerPlugin, 0),
	}
}

// Register adds a plugin to the registry
func (r *PluginRegistry) Register(plugin FileHandlerPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.plugins = append(r.plugins, plugin)
	logger.Debug("Plugin registered", "name", plugin.GetName())
}

// Unregister removes a plugin from the registry
func (r *PluginRegistry) Unregister(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for i, plugin := range r.plugins {
		if plugin.GetName() == pluginName {
			r.plugins = append(r.plugins[:i], r.plugins[i+1:]...)
			logger.Debug("Plugin unregistered", "name", pluginName)
			return
		}
	}
}

// Match finds the first plugin that can handle the given file
func (r *PluginRegistry) Match(path string, info fs.FileInfo) FileHandlerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, plugin := range r.plugins {
		if plugin.Match(path, info) {
			return plugin
		}
	}
	return nil
}

// GetRegisteredPlugins returns a list of all registered plugins
func (r *PluginRegistry) GetRegisteredPlugins() []FileHandlerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to prevent external modification
	plugins := make([]FileHandlerPlugin, len(r.plugins))
	copy(plugins, r.plugins)
	return plugins
}

// Helper function to get file extension in lowercase
func getFileExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if len(ext) > 0 && ext[0] == '.' {
		return ext[1:] // Remove the leading dot
	}
	return ext
}

// PluginRouter manages plugin integration for the scanner
type PluginRouter struct {
	pluginManager plugins.Manager
	mu            sync.RWMutex
	hooks         []ScannerPluginHook
	plugins       map[string]*plugins.Plugin
}

// ScannerPluginHook defines the interface for scanner plugin hooks
type ScannerPluginHook interface {
	OnScanStarted(jobID, libraryID uint, path string) error
	OnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) error
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
}

// NewPluginRouter creates a new plugin router
func NewPluginRouter(pluginManager plugins.Manager) *PluginRouter {
	return &PluginRouter{
		pluginManager: pluginManager,
		hooks:         make([]ScannerPluginHook, 0),
		plugins:       make(map[string]*plugins.Plugin),
	}
}

// RegisterHook adds a new scanner plugin hook
func (pr *PluginRouter) RegisterHook(hook ScannerPluginHook) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.hooks = append(pr.hooks, hook)
}

// UnregisterHook removes a scanner plugin hook
func (pr *PluginRouter) UnregisterHook(hook ScannerPluginHook) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	for i, h := range pr.hooks {
		if h == hook {
			pr.hooks = append(pr.hooks[:i], pr.hooks[i+1:]...)
			break
		}
	}
}

// CallOnScanStarted notifies all plugins that a scan has started
func (pr *PluginRouter) CallOnScanStarted(jobID, libraryID uint, path string) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	for _, hook := range pr.hooks {
		go func(h ScannerPluginHook) {
			if err := h.OnScanStarted(jobID, libraryID, path); err != nil {
				// Log error but don't fail the scan
				logger.Error("Plugin hook OnScanStarted failed", "error", err)
			}
		}(hook)
	}
}

// CallOnScanCompleted notifies all plugins that a scan has completed
func (pr *PluginRouter) CallOnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	for _, hook := range pr.hooks {
		go func(h ScannerPluginHook) {
			if err := h.OnScanCompleted(jobID, libraryID, stats); err != nil {
				// Log error but don't fail the scan
				logger.Error("Plugin hook OnScanCompleted failed", "error", err)
			}
		}(hook)
	}
}

// CallOnMediaFileScanned notifies all plugins that a media file has been scanned
func (pr *PluginRouter) CallOnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	for _, hook := range pr.hooks {
		go func(h ScannerPluginHook) {
			if err := h.OnMediaFileScanned(mediaFile, metadata); err != nil {
				// Log error but don't fail the scan
				logger.Error("Plugin hook OnMediaFileScanned failed", "error", err)
			}
		}(hook)
	}
}

func (pr *PluginRouter) RegisterPlugin(plugin *plugins.Plugin) {
	pr.plugins[plugin.ID] = plugin
	logger.Debug("Plugin registered", "name", plugin.Name)
}

func (pr *PluginRouter) UnregisterPlugin(pluginID string) {
	delete(pr.plugins, pluginID)
	logger.Debug("Plugin unregistered", "name", pluginID)
} 