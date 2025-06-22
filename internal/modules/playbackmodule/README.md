# Playback Module - Clean Architecture

## Overview

The Playback Module has been completely refactored to follow a clean architecture pattern, providing dynamic discovery and use of transcoding plugins. This provides a scalable, extensible architecture for video transcoding and streaming.

## Architecture

### Module Structure

The playback module follows a clean separation of concerns:

1. **module.go** - Module lifecycle management (Init, Migrate, RegisterRoutes, Shutdown)
2. **manager.go** - Business logic and service management
3. **api_handler.go** - HTTP request handling
4. **routes.go** - Route registration
5. **transcode_manager.go** - Transcoding session management
6. **planner.go** - Playback decision logic
7. **types.go** - Shared type definitions
8. **plugin_adapter.go** - Plugin system integration

### Core Components

1. **Manager** - Central business logic coordinator
   - Owns the TranscodeManager and Planner
   - Manages background services (cleanup)
   - Handles configuration

2. **PlaybackPlanner** - Analyzes media and device capabilities to decide between direct play and transcoding

3. **TranscodeManager** - Plugin-aware transcoding session manager

4. **APIHandler** - HTTP endpoint handlers

### Plugin Integration Flow

```
[Client Request] 
    ↓
[APIHandler] 
    ↓
[Manager] 
    ↓
[TranscodeManager] 
    ↓
[Plugin Discovery] → [Available Transcoding Plugins]
    ↓
[Best Plugin Selection] 
    ↓
[Transcoding Session] → [FFmpeg/Other Transcoder]
    ↓
[Streaming Response]
```

## Key Features

### 🔍 **Automatic Plugin Discovery**
- Scans running plugins for transcoding services
- Dynamically registers available transcoders
- Supports hot-reloading of plugins

### 🎯 **Intelligent Backend Selection**
- Evaluates plugins based on:
  - Codec support (H.264, HEVC, VP8, VP9, AV1)
  - Resolution capabilities
  - Container format support
  - Priority and current load
- Automatically selects the best available transcoder

### 📊 **Comprehensive Session Management**
- Real-time session tracking
- Concurrent session limits
- Automatic cleanup of expired sessions
- Detailed statistics and monitoring

### 🔄 **Streaming Optimization**
- Direct streaming from transcoder output
- Real-time progress monitoring
- Efficient memory usage with streaming pipes

## API Endpoints

### Playback Decision
```http
POST /api/playback/decide
```
**Request:**
```json
{
  "media_path": "/path/to/video.mkv",
  "device_profile": {
    "user_agent": "Mozilla/5.0...",
    "supported_codecs": ["h264", "aac"],
    "max_resolution": "1080p",
    "max_bitrate": 6000,
    "supports_hevc": false,
    "client_ip": "192.168.1.100"
  }
}
```

**Response:**
```json
{
  "should_transcode": true,
  "transcode_params": {
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000
  },
  "reason": "Transcoding required: container change: mkv -> mp4"
}
```

### Start Transcoding
```http
POST /api/playback/start
```
**Request:**
```json
{
  "input_path": "/path/to/video.mkv",
  "target_codec": "h264",
  "target_container": "mp4",
  "resolution": "1080p",
  "bitrate": 3000,
  "audio_codec": "aac",
  "quality": 23,
  "preset": "fast"
}
```

**Response:**
```json
{
  "id": "uuid-session-id",
  "status": "running",
  "start_time": "2024-01-01T12:00:00Z",
  "backend": "ffmpeg_transcoder",
  "progress": 0.0
}
```

### Stream Transcoded Video
```http
GET /api/playback/stream/:sessionId
```
**Response:** Direct video stream with appropriate headers

### Session Management
```http
GET    /api/playback/session/:sessionId      # Get session info
DELETE /api/playback/session/:sessionId      # Stop session
GET    /api/playback/sessions                # List active sessions
GET    /api/playback/stats                   # Get statistics
```

### Cleanup Management
```http
POST   /api/playback/cleanup/run             # Run manual cleanup
GET    /api/playback/cleanup/stats           # Get cleanup statistics
```

## Configuration

### Module Configuration
```go
type Config struct {
    MaxConcurrentSessions    int
    SessionTimeoutMinutes    int
    EnableHardwareAccel      bool
    DefaultQuality           int
    DefaultPreset            string
    TranscodingTimeout       time.Duration
    BufferSize               int
    CleanupIntervalMinutes   int
    CleanupRetentionMinutes  int
    EnableDebugLogging       bool
}
```

Configuration can be set via environment variables:
- `PLAYBACK_MAX_CONCURRENT_SESSIONS` (default: 3)
- `PLAYBACK_SESSION_TIMEOUT_MINUTES` (default: 120)
- `PLAYBACK_CLEANUP_INTERVAL_MINUTES` (default: 30)
- etc.

### Plugin Configuration (FFmpeg Example)
Located in `backend/data/plugins/ffmpeg_transcoder/plugin.cue`:

```cue
plugin: {
    id:          "ffmpeg_transcoder"
    name:        "FFmpeg Transcoder"
    type:        "transcoder"
    priority:    50
}

ffmpeg: {
    path:    "ffmpeg"
    preset:  "fast"
    threads: 0
}

quality: {
    crf_h264: 23.0
    crf_hevc: 28.0
}

performance: {
    max_concurrent_jobs: 3
    timeout_seconds:     1800
}
```

## Integration Example

### Basic Setup
```go
package main

import (
    "github.com/mantonx/viewra/internal/database"
    "github.com/mantonx/viewra/internal/events"
    "github.com/mantonx/viewra/internal/modules/playbackmodule"
    "github.com/mantonx/viewra/internal/modules/pluginmodule"
    "gorm.io/gorm"
)

func setupPlaybackModule(db *gorm.DB) error {
    // 1. Create external plugin manager
    logger := hclog.NewNullLogger()
    externalPluginManager := pluginmodule.NewExternalPluginManager(db, logger)
    
    // 2. Initialize with plugin directory
    pluginDir := "/path/to/plugins"
    hostServices := &pluginmodule.HostServices{}
    
    err := externalPluginManager.Initialize(ctx, pluginDir, hostServices)
    if err != nil {
        return err
    }

    // 3. Create adapter and module
    adapter := playbackmodule.NewExternalPluginManagerAdapter(externalPluginManager)
    module := playbackmodule.NewModule(db, nil, adapter)
    
    // 4. Initialize module
    return module.Init()
}
```

### Usage Example
```go
// Get the manager from the module
manager := module.GetManager()
if manager == nil {
    return fmt.Errorf("playback manager not available")
}

// Get playback decision
deviceProfile := &playbackmodule.DeviceProfile{
    UserAgent:       "Chrome/Browser",
    SupportedCodecs: []string{"h264", "aac"},
    MaxResolution:   "1080p",
    MaxBitrate:      6000,
}

planner := manager.GetPlanner()
decision, err := planner.DecidePlayback(mediaPath, deviceProfile)
if err != nil {
    return err
}

// Start transcoding if needed
if decision.ShouldTranscode {
    transcodeManager := manager.GetTranscodeManager()
    session, err := transcodeManager.StartTranscode(decision.TranscodeParams)
    if err != nil {
        return err
    }
    
    // Get the transcoding stream
    stream, err := transcodeManager.GetStream(session.ID)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    // Use stream for HTTP response...
}
```

## Plugin Development

### Implementing a Transcoding Plugin

1. **Implement the `plugins.Implementation` interface**
2. **Provide `TranscodingProvider()` method**
3. **Implement `plugins.TranscodingProvider` interface**

```go
type MyTranscoderPlugin struct {
    // Plugin fields
}

func (p *MyTranscoderPlugin) TranscodingProvider() plugins.TranscodingProvider {
    return p.transcodingProvider
}

func (p *MyTranscoderPlugin) Initialize(ctx *plugins.PluginContext) error {
    // Initialize transcoding provider
    return nil
}
```

### Transcoding Provider Implementation

```go
func (p *MyProvider) GetInfo() plugins.ProviderInfo {
    return plugins.ProviderInfo{
        ID:          "my-transcoder",
        Name:        "My Transcoder",
        Description: "Custom transcoding provider",
        Version:     "1.0.0",
        Author:      "My Company",
        Priority:    75, // Higher = preferred
    }
}

func (p *MyProvider) GetSupportedFormats() []plugins.ContainerFormat {
    return []plugins.ContainerFormat{
        {Format: "mp4", MimeType: "video/mp4", Extensions: []string{".mp4"}},
        {Format: "webm", MimeType: "video/webm", Extensions: []string{".webm"}},
    }
}

func (p *MyProvider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
    // Implement transcoding logic
}
```

## Monitoring and Statistics

### Real-time Statistics
```go
manager := module.GetManager()
stats, err := manager.GetTranscodeManager().GetStats()

// Access backend information
for backendID, backend := range stats.Backends {
    fmt.Printf("Backend: %s (Priority: %d, Active: %d)\n", 
        backend.Name, backend.Priority, backend.ActiveSessions)
}

// Access recent sessions
for _, session := range stats.RecentSessions {
    fmt.Printf("Session %s: %s (%s)\n", 
        session.ID, session.Status, session.Provider)
}
```

### Health Monitoring
- Automatic session cleanup
- Plugin health checks
- Performance metrics
- Error tracking and recovery

## Benefits

### 🚀 **Scalability**
- Add new transcoding backends without code changes
- Horizontal scaling with multiple plugin instances
- Load balancing across available transcoders

### 🔧 **Flexibility**
- Support for multiple transcoding engines (FFmpeg, hardware encoders, cloud services)
- Runtime plugin discovery and registration
- Configurable quality and performance settings

### 📈 **Performance**
- Intelligent backend selection
- Real-time streaming without temporary files
- Concurrent session management
- Resource optimization

### 🛠 **Maintainability**
- Clean separation of concerns
- Plugin-based architecture
- Comprehensive error handling
- Extensive logging and monitoring

## Current Plugin Support

### FFmpeg Transcoder Plugin
- **Location**: `backend/data/plugins/ffmpeg_transcoder/`
- **Codecs**: H.264, HEVC, VP8, VP9, AV1
- **Features**: Subtitle burn-in, multi-audio, streaming output
- **Priority**: 50 (configurable)
- **Status**: ✅ Complete implementation

### Future Plugin Opportunities
- **Hardware Transcoders**: NVENC, VAAPI, QuickSync
- **Cloud Transcoders**: AWS Elemental, Google Transcoder API
- **Specialized Encoders**: x265, SVT-AV1, rav1e

 // Updated: Sat Jun 21 07:45:54 PM EDT 2025
