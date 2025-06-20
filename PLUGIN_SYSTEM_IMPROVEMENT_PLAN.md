# Plugin System Improvement Plan

## Current Issues

1. **CUE Parsing Issues**
   - Type field showing constraint syntax instead of simple string
   - Plugin discovery failing due to type mismatch
   - Plugin info API returning corrupted data

2. **Build Process Issues**
   - Binary permissions not set correctly
   - Architecture mismatches (host vs Docker)
   - Build mode confusion (plugin vs executable)

3. **Discovery Issues**
   - Provider discovery not finding running plugins
   - Interface casting failures
   - Plugin registration timing problems

4. **Hot Reload Issues**
   - Inconsistent hot reload behavior
   - Process monitoring gaps
   - State synchronization problems

## Solution Overview

### 1. Enhanced Plugin Development Script

The new `scripts/plugin-dev.sh` provides:
- Automated CUE file validation and fixing
- Docker-based builds for architecture compatibility
- Automatic permission setting
- Integrated testing workflow
- Status monitoring and logs

### 2. Key Improvements

#### CUE File Handling
- Automatically fix constraint syntax in type fields
- Validate all required fields
- Ensure simple string values for core fields

#### Build Process
- Always build inside Docker container
- Set execute permissions automatically
- Use correct build flags (no -buildmode=plugin)
- Verify binary after build

#### Discovery Process
- Add refresh endpoint for manual plugin discovery
- Enhanced logging for debugging
- Better error messages
- Automatic retry logic

#### Hot Reload
- Automatic detection of binary changes
- Proper process cleanup
- State synchronization

## Usage Guide

### Quick Start

```bash
# List all plugins
./scripts/plugin-dev.sh list

# Build and enable a plugin
./scripts/plugin-dev.sh workflow ffmpeg_transcoder

# Check plugin status
./scripts/plugin-dev.sh status ffmpeg_transcoder

# View logs
./scripts/plugin-dev.sh logs ffmpeg_transcoder 100

# Test transcoding
./scripts/plugin-dev.sh test-transcode
```

### Development Workflow

1. **Make Code Changes**
   ```bash
   vim backend/data/plugins/ffmpeg_transcoder/main.go
   ```

2. **Build Plugin**
   ```bash
   ./scripts/plugin-dev.sh build ffmpeg_transcoder
   ```

3. **Hot Reload (if already running)**
   ```bash
   ./scripts/plugin-dev.sh reload ffmpeg_transcoder
   ```

4. **Test Plugin**
   ```bash
   ./scripts/plugin-dev.sh test-transcode
   ```

5. **Check Logs**
   ```bash
   ./scripts/plugin-dev.sh logs ffmpeg_transcoder
   ```

### Troubleshooting

#### Plugin Not Found in Discovery
1. Check CUE file has correct type field
2. Ensure plugin is enabled
3. Refresh plugin discovery
4. Check logs for errors

#### Binary Not Found
1. Ensure plugin was built
2. Check Docker container path
3. Verify permissions

#### Transcoding Fails
1. Check FFmpeg is available in plugin
2. Verify configuration paths
3. Check provider discovery
4. Review session logs

### Best Practices

1. **Always use Docker builds** - ensures Alpine Linux compatibility
2. **Monitor logs during development** - helps catch issues early
3. **Use workflow command** - automates the entire process
4. **Keep CUE files simple** - avoid complex constraint syntax
5. **Test after changes** - verify functionality works end-to-end

## Implementation Details

### CUE Parser Fix

The CUE parser now handles constraint syntax properly:
```go
// Before: type: "transcoder" | "metadata_scraper" | *"transcoder"
// After: type: "transcoder"
```

### Plugin Discovery Enhancement

Added detailed logging:
```go
tm.logger.Info("examining plugin",
    "plugin_id", pluginInfo.ID,
    "name", pluginInfo.Name,
    "type", pluginInfo.Type,
    "version", pluginInfo.Version,
)
```

### API Endpoints

- `POST /api/playback/plugins/refresh` - Refresh transcoding plugin discovery
- `GET /api/v1/plugins/` - List all plugins with correct info
- `POST /api/plugin-manager/external/{plugin}/enable` - Enable plugin

## Future Improvements

1. **Automatic CUE Migration**
   - Tool to update old CUE files
   - Validation on startup
   - Schema versioning

2. **Plugin Templates**
   - Generate new plugins from templates
   - Include best practices
   - Example implementations

3. **Better Error Recovery**
   - Automatic restart on crash
   - Circuit breaker pattern
   - Fallback mechanisms

4. **Development UI**
   - Web-based plugin manager
   - Real-time log viewer
   - Configuration editor

5. **Testing Framework**
   - Unit test helpers
   - Integration test suite
   - Performance benchmarks 