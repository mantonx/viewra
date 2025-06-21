package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// ScannerPluginHook defines the interface for plugins that want to hook into scan events
type ScannerPluginHook interface {
	OnScanStarted(jobID, libraryID uint, path string) error
	OnFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
	OnScanCompleted(libraryID uint, stats ScanStats) error
	Name() string
}

// ScanStats represents scan completion statistics
type ScanStats struct {
	FilesProcessed int64
	FilesFound     int64
	FilesSkipped   int64
	BytesProcessed int64
	Duration       string
	ErrorCount     int64
}

// PluginMetadata represents metadata extracted by a plugin
type PluginMetadata struct {
	PluginName string                 `json:"plugin_name"`
	Version    string                 `json:"version"`
	Type       string                 `json:"type"`     // "core" or "external"
	Category   string                 `json:"category"` // "video", "audio", "image", etc.
	Data       map[string]interface{} `json:"data"`     // Arbitrary metadata
}

// FileProcessingResult represents the result of file processing
type FileProcessingResult struct {
	MediaFile      *database.MediaFile `json:"media_file"`
	Metadata       []PluginMetadata    `json:"metadata"`
	ProcessedBy    []string            `json:"processed_by"`
	Error          error               `json:"error,omitempty"`
	FilePath       string              `json:"file_path"`
	ProcessingTime float64             `json:"processing_time_ms"`
}

// PluginRouter manages plugin routing for file processing
type PluginRouter struct {
	pluginModule *pluginmodule.PluginModule
	mu           sync.RWMutex
	hooks        []ScannerPluginHook
}

// NewPluginRouter creates a new plugin router
func NewPluginRouter(pluginModule *pluginmodule.PluginModule) *PluginRouter {
	return &PluginRouter{
		pluginModule: pluginModule,
	}
}

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

	// Convert stats map to ScanStats struct
	scanStats := ScanStats{
		FilesProcessed: getInt64FromMap(stats, "files_processed"),
		FilesFound:     getInt64FromMap(stats, "files_found"),
		FilesSkipped:   getInt64FromMap(stats, "files_skipped"),
		BytesProcessed: getInt64FromMap(stats, "bytes_processed"),
		Duration:       getStringFromMap(stats, "duration"),
		ErrorCount:     getInt64FromMap(stats, "error_count"),
	}

	for _, hook := range pr.hooks {
		go func(h ScannerPluginHook) {
			if err := h.OnScanCompleted(libraryID, scanStats); err != nil {
				// Log error but don't fail the scan
				logger.Error("Plugin hook OnScanCompleted failed", "error", err)
			}
		}(hook)
	}
}

// Helper functions to safely extract values from map
func getInt64FromMap(m map[string]interface{}, key string) int64 {
	if val, ok := m[key]; ok {
		if i64, ok := val.(int64); ok {
			return i64
		}
		if i, ok := val.(int); ok {
			return int64(i)
		}
	}
	return 0
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
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

func (pr *PluginRouter) RegisterPlugin(plugin interface{}) {
	// TODO: Implement plugin registration for new system
	logger.Debug("Plugin registration not yet implemented in new system", "plugin", plugin)
}

func (pr *PluginRouter) UnregisterPlugin(pluginID string) {
	// TODO: Implement plugin unregistration for new system
	logger.Debug("Plugin unregistration not yet implemented in new system", "name", pluginID)
}
