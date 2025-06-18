# Playback Module - Plugin Integration

## Overview

The Playback Module has been completely refactored to integrate with the Viewra plugin system, enabling dynamic discovery and use of transcoding plugins. This provides a scalable, extensible architecture for video transcoding and streaming.

## Architecture

### Core Components

1. **PlaybackPlanner** - Analyzes media and device capabilities to decide between direct play and transcoding
2. **PluginTranscodeManager** - Plugin-aware transcoding session manager
3. **ExternalPluginManagerAdapter** - Bridges the external plugin manager with playback module interfaces
4. **HTTP API Module** - REST endpoints for playback decisions and transcoding control

### Plugin Integration Flow

```
[Client Request] 
    ↓
[PlaybackModule] 
    ↓
[PluginTranscodeManager] 
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
POST /api/playback/transcode/start
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
GET /api/playback/transcode/:sessionId/stream
```
**Response:** Direct video stream with appropriate headers

### Session Management
```http
GET    /api/playback/transcode/:sessionId      # Get session info
DELETE /api/playback/transcode/:sessionId      # Stop session
GET    /api/playback/transcode/sessions        # List active sessions
GET    /api/playback/stats                     # Get statistics
```

## Configuration

### Module Configuration
```go
type PlaybackModuleConfig struct {
    Enabled           bool              `json:"enabled"`
    TranscodingConfig TranscodingConfig `json:"transcoding"`
    StreamingConfig   StreamingConfig   `json:"streaming"`
}

type TranscodingConfig struct {
    MaxConcurrentSessions int      `json:"max_concurrent_sessions"`
    SessionTimeoutMinutes int      `json:"session_timeout_minutes"`
    AutoDiscoverPlugins   bool     `json:"auto_discover_plugins"`
    PreferredCodecs       []string `json:"preferred_codecs"`
    DefaultQuality        int      `json:"default_quality"`
    DefaultPreset         string   `json:"default_preset"`
}
```

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
    "github.com/hashicorp/go-hclog"
    "github.com/mantonx/viewra/internal/modules/playbackmodule"
    "github.com/mantonx/viewra/internal/modules/pluginmodule"
    "gorm.io/gorm"
)

func setupPlaybackModule(db *gorm.DB, logger hclog.Logger) error {
    // 1. Create external plugin manager
    externalPluginManager := pluginmodule.NewExternalPluginManager(db, logger)
    
    // 2. Initialize with plugin directory
    pluginDir := "/path/to/plugins"
    hostServices := &pluginmodule.HostServices{}
    
    err := externalPluginManager.Initialize(ctx, pluginDir, hostServices)
    if err != nil {
        return err
    }

    // 3. Create adapter and playback module
    adapter := playbackmodule.NewExternalPluginManagerAdapter(externalPluginManager)
    playbackModule := playbackmodule.NewPlaybackModule(logger, adapter)
    
    // 4. Initialize (discovers transcoding plugins)
    return playbackModule.Initialize()
}
```

### Usage Example
```go
// Get playback decision
deviceProfile := &plugins.DeviceProfile{
    UserAgent:       "Chrome/Browser",
    SupportedCodecs: []string{"h264", "aac"},
    MaxResolution:   "1080p",
    MaxBitrate:      6000,
}

decision, err := playbackModule.GetPlanner().DecidePlayback(mediaPath, deviceProfile)
if err != nil {
    return err
}

// Start transcoding if needed
if decision.ShouldTranscode {
    session, err := playbackModule.GetTranscodeManager().StartTranscode(decision.TranscodeParams)
    if err != nil {
        return err
    }
    
    // Stream the transcoded video
    transcodingService, err := playbackModule.GetTranscodeManager().GetTranscodeStream(session.ID)
    if err != nil {
        return err
    }
    
    stream, err := transcodingService.GetTranscodeStream(ctx, session.ID)
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
2. **Provide `TranscodingService()` method**
3. **Implement `plugins.TranscodingService` interface**

```go
type MyTranscoderPlugin struct {
    // Plugin fields
}

func (p *MyTranscoderPlugin) TranscodingService() plugins.TranscodingService {
    return p.transcodingService
}

func (p *MyTranscoderPlugin) Initialize(ctx *plugins.PluginContext) error {
    // Initialize transcoding service
    return nil
}
```

### Transcoding Service Implementation

```go
func (s *MyTranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
    return &plugins.TranscodingCapabilities{
        Name:                  "my-transcoder",
        SupportedCodecs:       []string{"h264", "hevc"},
        SupportedResolutions:  []string{"480p", "720p", "1080p"},
        SupportedContainers:   []string{"mp4", "webm"},
        MaxConcurrentSessions: 5,
        Priority:              75, // Higher = preferred
        Features: plugins.TranscodingFeatures{
            StreamingOutput:     true,
            SubtitleBurnIn:      true,
            MultiAudioTracks:    true,
        },
    }, nil
}

func (s *MyTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
    // Implement transcoding logic
}
```

## Monitoring and Statistics

### Real-time Statistics
```go
stats, err := playbackModule.GetTranscodeManager().GetStats()

// Access backend information
for backendID, backend := range stats.Backends {
    fmt.Printf("Backend: %s (Priority: %d, Active: %d)\n", 
        backend.Name, backend.Priority, backend.ActiveSessions)
    fmt.Printf("Codecs: %v\n", backend.Capabilities.SupportedCodecs)
}

// Access recent sessions
for _, session := range stats.RecentSessions {
    fmt.Printf("Session %s: %s (%s)\n", 
        session.ID, session.Status, session.Backend)
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

 