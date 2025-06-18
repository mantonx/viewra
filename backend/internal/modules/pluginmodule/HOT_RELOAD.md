# 🔥 Plugin Hot Reload System

The Viewra Plugin Hot Reload System enables rapid plugin development by automatically detecting and reloading plugins when their binaries change, eliminating the need to restart the backend during development.

## 🚀 Quick Start

1. **Start the backend with hot reload enabled (default):**
   ```bash
   docker-compose up -d backend
   ```

2. **Develop your plugin and build it:**
   ```bash
   ./backend/scripts/build-plugin.sh ffmpeg_transcoder
   ```

3. **The plugin automatically reloads!** No backend restart needed.

4. **Use the demo script for interactive development:**
   ```bash
   ./scripts/hot-reload-demo.sh interactive
   ```

## 🏗️ Architecture

### Core Components

#### 1. HotReloadManager
- **File Watching**: Uses `fsnotify` to monitor plugin directory for changes
- **Debouncing**: Prevents excessive reloads with configurable delay (default: 500ms)
- **State Preservation**: Maintains plugin configuration and runtime data across reloads
- **Graceful Shutdown**: Properly stops old plugin instances before starting new ones

#### 2. Configuration System
- **Module-level Config**: Integrated into `PluginModuleConfig`
- **Runtime Control**: Enable/disable hot reload without restart
- **Flexible Patterns**: Configurable watch and exclude patterns

#### 3. HTTP API
- **Status Monitoring**: `GET /api/plugin-manager/hot-reload/status`
- **Control**: `POST /api/plugin-manager/hot-reload/enable|disable`
- **Manual Triggers**: `POST /api/plugin-manager/hot-reload/trigger/{plugin_id}`

### File Structure

```
backend/internal/modules/pluginmodule/
├── hot_reload.go           # Core hot reload implementation
├── module.go              # Integration with plugin module
├── types.go               # Configuration types
└── HOT_RELOAD.md         # This documentation
```

## ⚙️ Configuration

### Default Configuration
```go
HotReload: PluginHotReloadConfig{
    Enabled:         true,
    DebounceDelayMs: 500,
    WatchPatterns:   []string{"*_transcoder", "*_enricher", "*_scanner"},
    ExcludePatterns: []string{"*.tmp", "*.log", "*.pid", ".git*", 
                              "*.swp", "*.swo", "go.mod", "go.sum", 
                              "*.go", "plugin.cue", "*.json"},
    PreserveState:   true,
    MaxRetries:      3,
    RetryDelayMs:    1000,
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `Enabled` | Enable/disable hot reload | `true` |
| `DebounceDelayMs` | Delay before reload after file change | `500` |
| `WatchPatterns` | Plugin directory patterns to watch | `["*_transcoder", "*_enricher", "*_scanner"]` |
| `ExcludePatterns` | File patterns to ignore | Source files, temp files, logs |
| `PreserveState` | Maintain plugin state across reloads | `true` |
| `MaxRetries` | Max retry attempts for failed reloads | `3` |
| `RetryDelayMs` | Delay between retry attempts | `1000` |

### Environment Variables
- `VIEWRA_HOT_RELOAD_ENABLED`: Override hot reload enable/disable
- `VIEWRA_HOT_RELOAD_DEBOUNCE_MS`: Override debounce delay
- `VIEWRA_PLUGIN_HOT_RELOAD`: Global hot reload toggle

## 🛠️ Development Workflow

### 1. Standard Development
```bash
# 1. Make changes to plugin source
vim backend/data/plugins/ffmpeg_transcoder/main.go

# 2. Build the plugin
./backend/scripts/build-plugin.sh ffmpeg_transcoder

# 3. Plugin automatically reloads!
# Check logs: docker-compose logs -f backend | grep reload
```

### 2. Interactive Development
```bash
# Start interactive mode
./scripts/hot-reload-demo.sh interactive

# Available commands:
# s) Show status
# l) List plugins  
# b) Build plugin
# r) Manual reload
# w) Watch logs
# e) Enable hot reload
# d) Disable hot reload
```

### 3. Monitoring and Debugging
```bash
# Watch hot reload events
docker-compose logs -f backend | grep -E "(hot.*reload|🔄|✅.*reload|❌.*reload)"

# Check status via API
curl http://localhost:8080/api/plugin-manager/hot-reload/status | jq

# List plugins
curl http://localhost:8080/api/plugin-manager/external | jq

# Manual trigger
curl -X POST http://localhost:8080/api/plugin-manager/hot-reload/trigger/ffmpeg_transcoder
```

## 🔄 Hot Reload Process

### 1. Change Detection
- File system events trigger via `fsnotify`
- Changes filtered by watch/exclude patterns
- Events debounced to prevent multiple reloads

### 2. Plugin State Preservation
```go
type PluginState struct {
    PluginID      string                 `json:"plugin_id"`
    Configuration map[string]interface{} `json:"configuration"`
    RuntimeData   map[string]interface{} `json:"runtime_data"`
    LastActivity  time.Time              `json:"last_activity"`
    Version       string                 `json:"version"`
    Status        string                 `json:"status"`
}
```

### 3. Graceful Reload Sequence
1. **Preserve State**: Save current plugin configuration and runtime data
2. **Graceful Stop**: Send shutdown signal to current plugin instance
3. **Cleanup**: Remove old plugin from registry
4. **Register New**: Load and register new plugin binary
5. **Restore State**: Apply preserved configuration to new instance
6. **Verify**: Confirm new plugin is running correctly

### 4. Error Handling
- **Retry Logic**: Failed reloads are retried with exponential backoff
- **State Recovery**: If reload fails, attempt to restore previous state
- **Logging**: Comprehensive logging for debugging reload issues

## 📊 Monitoring and Metrics

### Reload Status Response
```json
{
  "enabled": true,
  "watching_directories": 3,
  "last_reload": "2024-01-15T10:30:45Z",
  "reload_count": 12,
  "failed_reloads": 1,
  "active_sessions": 2,
  "debounce_delay_ms": 500,
  "watch_patterns": ["*_transcoder", "*_enricher", "*_scanner"],
  "exclude_patterns": ["*.tmp", "*.log", "*.go"]
}
```

### Log Messages
- `🔄 Hot reload started` - Reload initiated
- `✅ Hot reload completed successfully` - Successful reload
- `❌ Hot reload failed` - Failed reload with error details
- `📁 Added watch for plugin directory` - Directory monitoring started

## 🚨 Troubleshooting

### Common Issues

#### 1. Plugin Not Reloading
**Problem**: Changes made but plugin doesn't reload
**Solutions**:
- Check if hot reload is enabled: `curl .../hot-reload/status`
- Verify plugin binary was actually rebuilt
- Check file patterns match plugin directory name
- Review logs for error messages

#### 2. Reload Failures
**Problem**: Hot reload triggers but fails to load new plugin
**Solutions**:
- Verify plugin binary is executable
- Check plugin dependencies are available
- Review plugin logs for startup errors
- Try manual reload with specific plugin ID

#### 3. State Loss
**Problem**: Plugin configuration lost after reload
**Solutions**:
- Ensure `PreserveState` is enabled in config
- Check plugin implements state serialization correctly
- Verify no conflicting plugin instances

#### 4. Performance Issues
**Problem**: Too many reload events or slow reloads
**Solutions**:
- Increase debounce delay
- Refine exclude patterns to filter more aggressively
- Check for file system permission issues
- Monitor system resources during reload

### Debug Commands
```bash
# Enable debug logging
export VIEWRA_LOG_LEVEL=debug

# Force manual reload
curl -X POST http://localhost:8080/api/plugin-manager/hot-reload/trigger/plugin_id

# Check plugin binary
ls -la backend/data/plugins/*/plugin_binary

# Verify file watching
inotifywait -m backend/data/plugins/
```

## 🔧 API Reference

### GET /api/plugin-manager/hot-reload/status
Get current hot reload status and statistics.

**Response:**
```json
{
  "status": "success",
  "hot_reload": {
    "enabled": true,
    "watching_directories": 3,
    "reload_count": 5,
    "last_reload": "2024-01-15T10:30:45Z"
  }
}
```

### POST /api/plugin-manager/hot-reload/enable
Enable hot reload functionality.

### POST /api/plugin-manager/hot-reload/disable  
Disable hot reload functionality.

### POST /api/plugin-manager/hot-reload/trigger/{plugin_id}
Manually trigger hot reload for specific plugin.

**Parameters:**
- `plugin_id`: ID of plugin to reload

## 🎯 Best Practices

### 1. Development Environment
- Always enable hot reload in development
- Use the demo script for initial testing
- Monitor logs during development for issues

### 2. Plugin Design
- Implement clean startup/shutdown in plugins
- Support state serialization for preservation
- Handle graceful termination signals

### 3. Build Process
- Use the official build script: `./backend/scripts/build-plugin.sh`
- Ensure plugins are built for correct architecture
- Test plugin binary before relying on hot reload

### 4. Monitoring
- Watch reload logs regularly during development
- Use status API to monitor reload health
- Set up alerts for repeated reload failures in production

### 5. Performance
- Optimize exclude patterns for your use case
- Adjust debounce delay based on development speed
- Disable hot reload in production environments

## 🚀 Advanced Features

### Custom Reload Callbacks
```go
// Set custom callbacks for reload events
hotReloadManager.SetReloadCallbacks(
    func(pluginID string) {
        // On reload start
        log.Info("Starting reload", "plugin", pluginID)
    },
    func(pluginID string, oldVersion, newVersion string) {
        // On reload success
        metrics.IncrementCounter("plugin_reloads_success")
    },
    func(pluginID string, err error) {
        // On reload failure
        metrics.IncrementCounter("plugin_reloads_failed")
        alerting.SendAlert("Plugin reload failed", err)
    },
)
```

### Dynamic Configuration
```go
// Enable/disable at runtime
pluginModule.SetHotReloadEnabled(true)

// Get current status
status := pluginModule.GetHotReloadStatus()

// Manual trigger
err := pluginModule.TriggerPluginReload("ffmpeg_transcoder")
```

### Integration with CI/CD
```bash
# In your build pipeline
./scripts/build-plugin.sh $PLUGIN_NAME

# Plugin automatically reloads in development environment
# No manual intervention needed!
```

---

**🎉 Happy Plugin Development!** 

The hot reload system eliminates the tedious restart cycle, letting you focus on building amazing plugins for the Viewra media system. 
