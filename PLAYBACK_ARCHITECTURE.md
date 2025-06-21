# Video Playback and Transcoding Architecture

This document provides a comprehensive overview of the video playback and transcoding system architecture in Viewra.

## Overview

The Viewra playback system implements a clean, modular architecture that supports adaptive streaming (DASH/HLS) and progressive downloads through an extensible plugin-based transcoding system.

## Architecture Diagram

```
Frontend Request
      ↓
┌─────────────────┐
│   API Routes    │ → /api/playback/start
│   (routes.go)   │
└─────────────────┘
      ↓
┌─────────────────┐
│   API Handler   │ → HandleStartTranscode()
│ (api_handler.go)│
└─────────────────┘
      ↓
┌─────────────────┐
│    Manager      │ → StartTranscode()
│  (manager.go)   │
└─────────────────┘
      ↓
┌─────────────────┐
│TranscodeService │ → providerManager.SelectProvider()
│(transcode_*.go) │
└─────────────────┘
      ↓
┌─────────────────┐
│ProviderManager  │ → Smart provider selection
│(provider_*.go)  │
└─────────────────┘
      ↓
┌─────────────────┐
│ Transcoding     │ → FFmpeg Software/Hardware
│   Plugins       │   VAAPI, QSV, NVIDIA, etc.
│ (plugins/*.go)  │
└─────────────────┘
```

## Key Components

### 1. API Layer (`/api/playback/*`)

**File**: `internal/modules/playbackmodule/routes.go`

Routes registered:
- `POST /api/playback/start` - Start transcoding session
- `GET /api/playback/stream/{id}/*` - Stream content
- `GET /api/playback/sessions` - List active sessions
- `DELETE /api/playback/sessions/{id}` - Stop session

**API Handler**: `internal/modules/playbackmodule/api_handler.go`
- Validates JSON requests
- Delegates to Manager layer
- Returns session information with manifest URLs

### 2. Service Layer (Clean Architecture)

**Service Registry**: `internal/services/registry.go`
- Provides dependency injection for inter-module communication
- Type-safe service retrieval: `services.GetService[PlaybackService]("playback")`

**Service Interface**: `internal/services/interfaces.go`
```go
type PlaybackService interface {
    DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error)
    StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error)
    GetSession(sessionID string) (*database.TranscodeSession, error)
    StopSession(sessionID string) error
    GetStats() (*types.TranscodingStats, error)
}
```

**Service Implementation**: `internal/modules/playbackmodule/service_impl.go`
- Bridges external service interface to internal manager
- Handles type conversions between external/internal types

### 3. Manager Layer

**Playback Manager**: `internal/modules/playbackmodule/manager.go`
- Orchestrates transcoding workflow
- Manages plugin discovery: `discoverTranscodingPlugins()` (lines 317-364)
- Delegates to core services

**Core Services**:
- `TranscodeService` - Main transcoding orchestration
- `ProviderManager` - Provider selection and load balancing  
- `SessionStore` - Session lifecycle management
- `FileManager` - Output file management
- `CleanupService` - Background cleanup operations

### 4. Provider Management

**Provider Manager**: `internal/modules/playbackmodule/core/provider_manager.go`

**Provider Selection Algorithm**:
```go
func (pm *ProviderManager) SelectProvider(ctx, req) (provider, error) {
    candidates := pm.getCapableProviders(req)        // Format compatibility
    optimal := pm.selectOptimalProvider(candidates)   // Smart scoring
    return optimal, nil
}
```

**Scoring Factors**:
1. **Format Support**: Must support requested container format
2. **Hardware Preference**: Bonus for hardware acceleration when requested
3. **Load Balancing**: Penalty for providers with high active session count
4. **Priority**: Provider-configured priority scores

### 5. Plugin Architecture

**Plugin Discovery**: 
- Plugins register with `type: "transcoder"` in `plugin.cue`
- Manager discovers via `GetRunningPlugins()` and filters by type
- Extracts `TranscodingProvider` interface for registration

**Plugin Structure**:
```
plugins/ffmpeg_software/
├── plugin.cue          # Configuration and metadata
├── main.go            # Plugin implementation  
├── Dockerfile         # Container build
└── README.md         # Documentation
```

**Plugin Interface**: Plugins must implement:
```go
type TranscodingProvider interface {
    GetInfo() ProviderInfo
    GetSupportedFormats() []ContainerFormat
    GetQualityPresets() []QualityPreset
    StartTranscode(ctx, req) (*TranscodeHandle, error)
    // ... additional methods
}
```

**Available Providers**:
- `ffmpeg_software` - CPU-based transcoding (universal)
- `ffmpeg_vaapi` - VAAPI hardware acceleration (Intel/AMD)
- `ffmpeg_qsv` - Intel Quick Sync Video
- `ffmpeg_nvidia` - NVIDIA NVENC acceleration

### 6. Session Management

**Session Store**: `internal/modules/playbackmodule/core/session_store.go`
- Database-backed session persistence
- Active session tracking and limits
- Session lifecycle events

**Session States**:
- `pending` - Initial state, not yet started
- `active` - Transcoding in progress
- `completed` - Successfully finished
- `failed` - Error occurred
- `stopped` - Manually terminated

### 7. Format Support

**Adaptive Streaming**:
- **DASH** (Dynamic Adaptive Streaming over HTTP)
  - Manifest: `/api/playback/stream/{id}/manifest.mpd`
  - Segments: `/api/playback/stream/{id}/segment_{n}.m4s`
- **HLS** (HTTP Live Streaming)  
  - Playlist: `/api/playback/stream/{id}/playlist.m3u8`
  - Segments: `/api/playback/stream/{id}/segment_{n}.ts`

**Progressive Streaming**:
- **MP4** - Direct progressive download
- **WebM** - Web-optimized container
- **MKV** - Matroska container support

## Data Flow Examples

### 1. Starting a Transcoding Session

```
1. POST /api/playback/start
   {
     "input_path": "/media/video.mkv",
     "container": "dash",
     "video_codec": "h264",
     "quality": 70
   }

2. API Handler validates and calls Manager.StartTranscode()

3. Manager calls TranscodeService.StartTranscode()

4. TranscodeService calls ProviderManager.SelectProvider()

5. ProviderManager:
   - Filters providers by format support (DASH)
   - Scores candidates by hardware/load/priority
   - Returns best provider (e.g., ffmpeg_software)

6. TranscodeService calls provider.StartTranscode()

7. Provider starts FFmpeg process and returns handle

8. Session stored in database with status "active"

9. API returns:
   {
     "id": "session_123",
     "status": "active", 
     "manifest_url": "/api/playback/stream/session_123/manifest.mpd",
     "provider": "ffmpeg_software"
   }
```

### 2. Plugin Registration Flow

```
1. Application startup → server.go → initializeModules()

2. Plugin module discovers external plugins

3. Playback module gets plugin manager via SetPluginModule()

4. Manager calls discoverTranscodingPlugins():
   - Gets running plugins from plugin manager
   - Filters by type: "transcoder"  
   - Extracts TranscodingProvider interface
   - Registers with TranscodeService.RegisterProvider()

5. Providers are now available for selection
```

## Inter-Module Communication

### Service Registry Pattern

**Registration** (Playback Module):
```go
// During module initialization
playbackService := NewPlaybackServiceImpl(m.manager)
services.RegisterService("playback", playbackService)
```

**Usage** (Media Module):
```go
// Clean service access
if playbackService, err := services.GetService[services.PlaybackService]("playback"); err == nil {
    m.playbackIntegration = NewPlaybackIntegration(m.db, playbackService)
}
```

**Benefits**:
- No circular dependencies
- Clean API boundaries  
- Testable with interface mocking
- Consistent pattern across modules

## Configuration

### Provider Configuration
Each plugin defines supported formats in `plugin.cue`:
```cue
plugin_name: "ffmpeg_software"
plugin_type: "transcoding"
enabled: true

capabilities: {
    formats: ["mp4", "dash", "hls", "webm"]
    codecs: ["h264", "h265", "vp9"]
    hardware_acceleration: "none"
}
```

### System Configuration
```go
type TranscodeConfig struct {
    MaxSessions      int
    OutputDir        string
    CleanupInterval  time.Duration
    SessionTimeout   time.Duration
}
```

## Error Handling

**Common Error Scenarios**:

1. **No Capable Providers**: `"no capable providers found for format: dash"`
   - Cause: No plugins support requested format
   - Solution: Install compatible plugin or change format

2. **Session Limit Reached**: `"maximum number of sessions reached: 10"`
   - Cause: Too many concurrent transcoding sessions
   - Solution: Wait for sessions to complete or increase limit

3. **Provider Error**: `"failed to start transcoding: FFmpeg not found"`
   - Cause: Plugin dependency missing
   - Solution: Install required dependencies in plugin container

## Performance Considerations

### Provider Selection Optimization
- **Format Pre-filtering**: Only considers compatible providers
- **Load Balancing**: Distributes load across available providers
- **Hardware Acceleration**: Prioritizes when requested and available

### Session Management
- **Background Cleanup**: Removes expired sessions and temporary files
- **Session Limits**: Prevents resource exhaustion
- **Progress Tracking**: Monitors transcoding progress and health

### Caching Strategy
- **Completed Sessions**: Cache manifest files for repeat access
- **Provider Capabilities**: Cache format support for faster selection
- **Session Metadata**: Database persistence for restart recovery

## Monitoring and Observability

### Logging Levels
- **TRACE**: Provider selection details, session lifecycle
- **DEBUG**: Request processing, plugin discovery
- **INFO**: Session start/stop, provider registration
- **WARN**: Non-critical errors, fallback scenarios  
- **ERROR**: Failed requests, plugin errors

### Metrics Available
- Active session count per provider
- Provider selection frequency
- Session success/failure rates
- Average transcoding times
- Resource utilization per provider

## Testing Strategy

### Unit Tests
- Provider selection algorithm
- Session lifecycle management
- Plugin interface compliance
- Service registry functionality

### Integration Tests  
- End-to-end transcoding workflows
- Plugin discovery and registration
- Multi-provider load balancing
- Error handling scenarios

### Performance Tests
- Concurrent session handling
- Provider selection under load
- Memory usage during transcoding
- Cleanup operation efficiency

## Future Enhancements

### Planned Features
1. **Dynamic Quality Adjustment**: Real-time quality adaptation based on network conditions
2. **Distributed Transcoding**: Multi-node transcoding cluster support
3. **GPU Pool Management**: Shared GPU resource scheduling
4. **Advanced Analytics**: Detailed performance metrics and insights
5. **CDN Integration**: Direct streaming to content delivery networks

### Architectural Improvements
1. **Repository Pattern**: Standardize database access across modules
2. **Command Pattern**: Implement undo/redo for transcoding operations  
3. **Event Sourcing**: Audit trail for all transcoding activities
4. **Circuit Breaker**: Resilience patterns for plugin failures

## Troubleshooting Guide

### Common Issues

**1. "No capable providers found"**
```bash
# Check available plugins
curl http://localhost:8080/api/plugins/list

# Verify plugin status  
curl http://localhost:8080/api/plugins/ffmpeg_software/status

# Check plugin logs
docker logs viewra-backend | grep -i plugin
```

**2. "Session creation failed"**
```bash
# Check active sessions
curl http://localhost:8080/api/playback/sessions

# Verify resource limits
curl http://localhost:8080/api/playback/stats

# Check available disk space
df -h /tmp/viewra-transcoding
```

**3. "Provider selection slow"**
```bash
# Enable trace logging for provider selection
# Add to environment: LOG_LEVEL=TRACE

# Monitor provider performance
curl http://localhost:8080/api/playback/providers/stats
```

This architecture provides a robust, scalable foundation for video transcoding with clear separation of concerns, extensible plugin support, and maintainable service boundaries.