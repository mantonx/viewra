package pluginmodule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"gorm.io/gorm"
)

// HotReloadManager manages hot reloading of plugins
type HotReloadManager struct {
	db              *gorm.DB
	logger          hclog.Logger
	externalManager *ExternalPluginManager

	// File watching
	watcher   *fsnotify.Watcher
	pluginDir string

	// State management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	// Configuration
	enabled       bool
	debounceDelay time.Duration

	// Reload tracking
	pendingReloads map[string]*time.Timer // pluginID -> timer
	reloadMutex    sync.Mutex

	// Plugin state preservation
	pluginStates map[string]*PluginState

	// Event callbacks
	onReloadStart   func(pluginID string)
	onReloadSuccess func(pluginID string, oldVersion, newVersion string)
	onReloadFailed  func(pluginID string, err error)
}

// PluginState represents the state of a plugin that needs to be preserved during reload
type PluginState struct {
	PluginID      string                 `json:"plugin_id"`
	Configuration map[string]interface{} `json:"configuration"`
	RuntimeData   map[string]interface{} `json:"runtime_data"`
	LastActivity  time.Time              `json:"last_activity"`
	Version       string                 `json:"version"`
	Status        string                 `json:"status"`
}

// HotReloadConfig configures the hot reload behavior
type HotReloadConfig struct {
	Enabled         bool          `json:"enabled"`
	DebounceDelay   time.Duration `json:"debounce_delay"`
	WatchPatterns   []string      `json:"watch_patterns"`
	ExcludePatterns []string      `json:"exclude_patterns"`
	PreserveState   bool          `json:"preserve_state"`
	MaxRetries      int           `json:"max_retries"`
	RetryDelay      time.Duration `json:"retry_delay"`
}

// DefaultHotReloadConfig returns the default hot reload configuration
func DefaultHotReloadConfig() *HotReloadConfig {
	return &HotReloadConfig{
		Enabled:         true,
		DebounceDelay:   500 * time.Millisecond,
		WatchPatterns:   []string{"*_transcoder", "*_enricher", "*_scanner"},
		ExcludePatterns: []string{"*.tmp", "*.log", "*.pid", ".git*", "*.swp", "*.swo", "go.mod", "go.sum", "*.go", "plugin.cue", "*.json"},
		PreserveState:   true,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
	}
}

// NewHotReloadConfigFromPluginConfig creates a HotReloadConfig from the global plugin config
func NewHotReloadConfigFromPluginConfig(cfg *config.PluginHotReloadConfig) *HotReloadConfig {
	if cfg == nil {
		return DefaultHotReloadConfig()
	}

	debounceDelay := time.Duration(cfg.DebounceDelayMs) * time.Millisecond
	if debounceDelay <= 0 {
		debounceDelay = 500 * time.Millisecond
	}

	retryDelay := time.Duration(cfg.RetryDelayMs) * time.Millisecond
	if retryDelay <= 0 {
		retryDelay = 1 * time.Second
	}

	watchPatterns := cfg.WatchPatterns
	if len(watchPatterns) == 0 {
		watchPatterns = []string{"*_transcoder", "*_enricher", "*_scanner"}
	}

	excludePatterns := cfg.ExcludePatterns
	if len(excludePatterns) == 0 {
		excludePatterns = []string{"*.tmp", "*.log", "*.pid", ".git*", "*.swp", "*.swo", "go.mod", "go.sum", "*.go", "plugin.cue", "*.json"}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &HotReloadConfig{
		Enabled:         cfg.Enabled,
		DebounceDelay:   debounceDelay,
		WatchPatterns:   watchPatterns,
		ExcludePatterns: excludePatterns,
		PreserveState:   cfg.PreserveState,
		MaxRetries:      maxRetries,
		RetryDelay:      retryDelay,
	}
}

// NewHotReloadManager creates a new hot reload manager
func NewHotReloadManager(db *gorm.DB, logger hclog.Logger, externalManager *ExternalPluginManager, pluginDir string, config *HotReloadConfig) (*HotReloadManager, error) {
	if config == nil {
		config = DefaultHotReloadConfig()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	hrm := &HotReloadManager{
		db:              db,
		logger:          logger.Named("hot-reload"),
		externalManager: externalManager,
		watcher:         watcher,
		pluginDir:       pluginDir,
		ctx:             ctx,
		cancel:          cancel,
		enabled:         config.Enabled,
		debounceDelay:   config.DebounceDelay,
		pendingReloads:  make(map[string]*time.Timer),
		pluginStates:    make(map[string]*PluginState),
	}

	return hrm, nil
}

// Start begins hot reload monitoring
func (hrm *HotReloadManager) Start() error {
	if !hrm.enabled {
		hrm.logger.Info("hot reload disabled by configuration")
		return nil
	}

	hrm.logger.Info("starting hot reload manager", "plugin_dir", hrm.pluginDir)

	// Ensure plugin directory exists
	if _, err := os.Stat(hrm.pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin directory does not exist: %s", hrm.pluginDir)
	}

	// Add watches for plugin directories
	if err := hrm.addWatches(); err != nil {
		return fmt.Errorf("failed to add file watches: %w", err)
	}

	// Start the file watcher event loop
	hrm.wg.Add(1)
	go hrm.watcherEventLoop()

	hrm.logger.Info("hot reload manager started successfully")
	return nil
}

// Stop gracefully stops the hot reload manager
func (hrm *HotReloadManager) Stop() error {
	hrm.logger.Info("stopping hot reload manager")

	// Cancel context
	hrm.cancel()

	// Close file watcher
	if hrm.watcher != nil {
		hrm.watcher.Close()
	}

	// Cancel pending reloads
	hrm.reloadMutex.Lock()
	for pluginID, timer := range hrm.pendingReloads {
		timer.Stop()
		hrm.logger.Debug("cancelled pending reload", "plugin_id", pluginID)
	}
	hrm.pendingReloads = make(map[string]*time.Timer)
	hrm.reloadMutex.Unlock()

	// Wait for goroutines to finish
	hrm.wg.Wait()

	hrm.logger.Info("hot reload manager stopped")
	return nil
}

// addWatches adds file system watches for plugin directories
func (hrm *HotReloadManager) addWatches() error {
	// Read plugin directory
	entries, err := os.ReadDir(hrm.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	watchedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if this directory matches our watch patterns
		if !hrm.shouldWatchPlugin(entry.Name()) {
			continue
		}

		pluginPath := filepath.Join(hrm.pluginDir, entry.Name())

		// Add watch for the plugin directory
		if err := hrm.watcher.Add(pluginPath); err != nil {
			hrm.logger.Error("failed to add watch for plugin directory", "path", pluginPath, "error", err)
			continue
		}

		watchedCount++
		hrm.logger.Debug("added watch for plugin directory", "path", pluginPath)
	}

	hrm.logger.Info("added file system watches", "watched_directories", watchedCount)
	return nil
}

// shouldWatchPlugin determines if a plugin directory should be watched
func (hrm *HotReloadManager) shouldWatchPlugin(dirName string) bool {
	// Check watch patterns (default: *_transcoder, *_enricher, *_scanner)
	watchPatterns := []string{"*_transcoder", "*_enricher", "*_scanner"}

	for _, pattern := range watchPatterns {
		if matched, _ := filepath.Match(pattern, dirName); matched {
			return true
		}
	}

	return false
}

// watcherEventLoop is the main event loop for file system events
func (hrm *HotReloadManager) watcherEventLoop() {
	defer hrm.wg.Done()

	hrm.logger.Info("hot reload watcher event loop started")

	for {
		select {
		case event, ok := <-hrm.watcher.Events:
			if !ok {
				hrm.logger.Info("watcher events channel closed")
				return
			}

			hrm.handleFileSystemEvent(event)

		case err, ok := <-hrm.watcher.Errors:
			if !ok {
				hrm.logger.Info("watcher errors channel closed")
				return
			}

			hrm.logger.Error("file watcher error", "error", err)

		case <-hrm.ctx.Done():
			hrm.logger.Info("hot reload watcher context cancelled")
			return
		}
	}
}

// handleFileSystemEvent processes a file system event
func (hrm *HotReloadManager) handleFileSystemEvent(event fsnotify.Event) {
	// Filter out unwanted events
	if !hrm.shouldProcessEvent(event) {
		return
	}

	pluginID := hrm.extractPluginIDFromPath(event.Name)
	if pluginID == "" {
		return
	}

	hrm.logger.Debug("file system event detected",
		"plugin_id", pluginID,
		"operation", event.Op,
		"path", event.Name)

	// Handle different event types
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		hrm.scheduleReload(pluginID, "binary updated")

	case event.Op&fsnotify.Create == fsnotify.Create:
		// Check if it's a new binary file
		if hrm.isBinaryFile(event.Name) {
			hrm.scheduleReload(pluginID, "new binary created")
		}

	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		// Binary permissions changed (could indicate new binary)
		if hrm.isBinaryFile(event.Name) {
			hrm.scheduleReload(pluginID, "binary permissions changed")
		}
	}
}

// shouldProcessEvent determines if an event should be processed
func (hrm *HotReloadManager) shouldProcessEvent(event fsnotify.Event) bool {
	filename := filepath.Base(event.Name)

	// Exclude temporary files, logs, etc.
	excludePatterns := []string{
		"*.tmp", "*.log", "*.pid", ".git*", "*.swp", "*.swo",
		"go.mod", "go.sum", "*.go", "plugin.cue", "*.json",
	}

	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return false
		}
	}

	return true
}

// extractPluginIDFromPath extracts the plugin ID from a file path
func (hrm *HotReloadManager) extractPluginIDFromPath(path string) string {
	// Get the plugin directory name
	relPath, err := filepath.Rel(hrm.pluginDir, path)
	if err != nil {
		return ""
	}

	// Split the path and get the first component (plugin directory)
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}

	pluginDir := parts[0]

	// Validate this is a plugin directory
	if !hrm.shouldWatchPlugin(pluginDir) {
		return ""
	}

	return pluginDir
}

// isBinaryFile checks if a file is a plugin binary
func (hrm *HotReloadManager) isBinaryFile(path string) bool {
	filename := filepath.Base(path)

	// Check if this could be a plugin binary
	// Plugin binaries are typically named after their directory
	pluginID := hrm.extractPluginIDFromPath(path)
	if pluginID == "" {
		return false
	}

	// Binary file should match the plugin directory name
	return filename == pluginID
}

// scheduleReload schedules a plugin reload with debouncing
func (hrm *HotReloadManager) scheduleReload(pluginID, reason string) {
	hrm.reloadMutex.Lock()
	defer hrm.reloadMutex.Unlock()

	// Cancel existing timer if one exists
	if timer, exists := hrm.pendingReloads[pluginID]; exists {
		timer.Stop()
		hrm.logger.Debug("cancelled previous reload timer", "plugin_id", pluginID)
	}

	// Schedule new reload with debounce delay
	timer := time.AfterFunc(hrm.debounceDelay, func() {
		hrm.performReload(pluginID, reason)

		// Clean up timer
		hrm.reloadMutex.Lock()
		delete(hrm.pendingReloads, pluginID)
		hrm.reloadMutex.Unlock()
	})

	hrm.pendingReloads[pluginID] = timer

	hrm.logger.Info("scheduled plugin reload",
		"plugin_id", pluginID,
		"reason", reason,
		"delay", hrm.debounceDelay)
}

// performReload performs the actual plugin reload
func (hrm *HotReloadManager) performReload(pluginID, reason string) {
	hrm.logger.Info("performing hot reload", "plugin_id", pluginID, "reason", reason)

	// Notify reload start
	if hrm.onReloadStart != nil {
		hrm.onReloadStart(pluginID)
	}

	// Step 1: Preserve plugin state
	oldState := hrm.preservePluginState(pluginID)

	// Step 2: Gracefully stop the old plugin
	oldVersion := ""
	if plugin, exists := hrm.externalManager.GetPlugin(pluginID); exists {
		oldVersion = plugin.Version
	}

	if err := hrm.gracefullyStopPlugin(pluginID); err != nil {
		hrm.logger.Error("failed to stop plugin gracefully", "plugin_id", pluginID, "error", err)
		if hrm.onReloadFailed != nil {
			hrm.onReloadFailed(pluginID, err)
		}
		return
	}

	// Step 3: Re-register the plugin (picks up new binary)
	if err := hrm.reregisterPlugin(pluginID); err != nil {
		hrm.logger.Error("failed to re-register plugin", "plugin_id", pluginID, "error", err)
		if hrm.onReloadFailed != nil {
			hrm.onReloadFailed(pluginID, err)
		}
		return
	}

	// Step 4: Start the new plugin
	if err := hrm.startNewPlugin(pluginID); err != nil {
		hrm.logger.Error("failed to start new plugin", "plugin_id", pluginID, "error", err)
		if hrm.onReloadFailed != nil {
			hrm.onReloadFailed(pluginID, err)
		}
		return
	}

	// Step 5: Restore plugin state (if supported)
	if oldState != nil {
		hrm.restorePluginState(pluginID, oldState)
	}

	// Get new version
	newVersion := ""
	if plugin, exists := hrm.externalManager.GetPlugin(pluginID); exists {
		newVersion = plugin.Version
	}

	hrm.logger.Info("hot reload completed successfully",
		"plugin_id", pluginID,
		"old_version", oldVersion,
		"new_version", newVersion)

	// Notify reload success
	if hrm.onReloadSuccess != nil {
		hrm.onReloadSuccess(pluginID, oldVersion, newVersion)
	}
}

// preservePluginState captures the current state of a plugin
func (hrm *HotReloadManager) preservePluginState(pluginID string) *PluginState {
	// Get current plugin info
	plugin, exists := hrm.externalManager.GetPlugin(pluginID)
	if !exists {
		return nil
	}

	state := &PluginState{
		PluginID:      pluginID,
		Configuration: make(map[string]interface{}),
		RuntimeData:   make(map[string]interface{}),
		LastActivity:  time.Now(),
		Version:       plugin.Version,
		Status:        "reloading",
	}

	// TODO: In the future, we could add hooks for plugins to export their state
	// For now, we just preserve basic information

	hrm.mu.Lock()
	hrm.pluginStates[pluginID] = state
	hrm.mu.Unlock()

	hrm.logger.Debug("preserved plugin state", "plugin_id", pluginID)
	return state
}

// gracefullyStopPlugin stops a plugin gracefully
func (hrm *HotReloadManager) gracefullyStopPlugin(pluginID string) error {
	hrm.logger.Debug("gracefully stopping plugin", "plugin_id", pluginID)

	// Use the external manager's unload method
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return hrm.externalManager.UnloadPlugin(ctx, pluginID)
}

// reregisterPlugin re-discovers and registers the plugin with new binary
func (hrm *HotReloadManager) reregisterPlugin(pluginID string) error {
	hrm.logger.Debug("re-registering plugin", "plugin_id", pluginID)

	// Refresh plugins to pick up new binary
	return hrm.externalManager.RefreshPlugins()
}

// startNewPlugin starts the newly registered plugin
func (hrm *HotReloadManager) startNewPlugin(pluginID string) error {
	hrm.logger.Debug("starting new plugin instance", "plugin_id", pluginID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return hrm.externalManager.LoadPlugin(ctx, pluginID)
}

// restorePluginState restores preserved state to the plugin
func (hrm *HotReloadManager) restorePluginState(pluginID string, state *PluginState) {
	hrm.logger.Debug("restoring plugin state", "plugin_id", pluginID)

	// TODO: Add hooks for plugins to import their state
	// For now, this is a placeholder

	// Clean up preserved state
	hrm.mu.Lock()
	delete(hrm.pluginStates, pluginID)
	hrm.mu.Unlock()
}

// SetReloadCallbacks sets callback functions for reload events
func (hrm *HotReloadManager) SetReloadCallbacks(
	onStart func(pluginID string),
	onSuccess func(pluginID string, oldVersion, newVersion string),
	onFailed func(pluginID string, err error),
) {
	hrm.onReloadStart = onStart
	hrm.onReloadSuccess = onSuccess
	hrm.onReloadFailed = onFailed
}

// TriggerManualReload manually triggers a reload for a specific plugin
func (hrm *HotReloadManager) TriggerManualReload(pluginID string) error {
	if !hrm.enabled {
		return fmt.Errorf("hot reload is disabled")
	}

	hrm.logger.Info("manual reload triggered", "plugin_id", pluginID)
	hrm.scheduleReload(pluginID, "manual trigger")
	return nil
}

// GetReloadStatus returns the current reload status
func (hrm *HotReloadManager) GetReloadStatus() map[string]interface{} {
	hrm.reloadMutex.Lock()
	defer hrm.reloadMutex.Unlock()

	pendingCount := len(hrm.pendingReloads)
	pendingPlugins := make([]string, 0, pendingCount)

	for pluginID := range hrm.pendingReloads {
		pendingPlugins = append(pendingPlugins, pluginID)
	}

	return map[string]interface{}{
		"enabled":         hrm.enabled,
		"pending_reloads": pendingCount,
		"pending_plugins": pendingPlugins,
		"debounce_delay":  hrm.debounceDelay.String(),
	}
}

// IsEnabled returns whether hot reload is enabled
func (hrm *HotReloadManager) IsEnabled() bool {
	return hrm.enabled
}

// SetEnabled enables or disables hot reload
func (hrm *HotReloadManager) SetEnabled(enabled bool) error {
	if hrm.enabled == enabled {
		return nil // No change
	}

	hrm.enabled = enabled

	if enabled {
		hrm.logger.Info("hot reload enabled")
		return hrm.Start()
	} else {
		hrm.logger.Info("hot reload disabled")
		return hrm.Stop()
	}
}
