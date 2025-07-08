# ADR-0005: Playback and Transcoding Architecture

Date: 2024-01-08
Status: Accepted

## Context

Video playback in modern applications requires supporting diverse device capabilities, network conditions, and media formats. A robust transcoding system is essential for providing optimal playback experiences across all platforms.

## Decision

We have implemented a modular playback and transcoding architecture with the following key components:

### 1. Clean Service Architecture
- **Service Registry Pattern**: Type-safe inter-module communication
- **Clear API Boundaries**: Well-defined service interfaces
- **No Circular Dependencies**: Modules communicate through services

### 2. Plugin-Based Transcoding
- **Provider Interface**: Standard interface for all transcoding providers
- **Dynamic Discovery**: Plugins discovered at runtime
- **Hardware Acceleration**: Support for various GPU acceleration methods

### 3. Intelligent Provider Selection
```go
// Smart scoring algorithm considers:
1. Format compatibility (required)
2. Hardware acceleration preference
3. Current load balancing
4. Provider priority configuration
```

### 4. Adaptive Streaming Support
- **DASH**: Dynamic Adaptive Streaming over HTTP
- **HLS**: HTTP Live Streaming
- **Progressive**: Direct download for compatible formats

### 5. Session Management
- **Database-backed**: Persistent session storage
- **Lifecycle Tracking**: pending → active → completed/failed
- **Resource Limits**: Concurrent session management
- **Automatic Cleanup**: Background cleanup service

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Media Module   │────▶│ Playback Module │────▶│Transcoding Mod. │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                               │                          │
                               ▼                          ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │Decision Engine  │     │Provider Manager │
                        └─────────────────┘     └─────────────────┘
                                                         │
                                                         ▼
                                                ┌─────────────────┐
                                                │ Plugin Providers│
                                                │ (FFmpeg, etc.)  │
                                                └─────────────────┘
```

## Implementation Details

### Service Interfaces
```go
type PlaybackService interface {
    DecidePlayback(mediaPath string, deviceProfile *DeviceProfile) (*PlaybackDecision, error)
    StartPlaybackSession(mediaFileID, userID, deviceID, method string) (*PlaybackSession, error)
    PrepareStreamURL(decision *PlaybackDecision, baseURL string) (string, error)
}

type TranscodingService interface {
    StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)
    GetProgress(sessionID string) (*TranscodingProgress, error)
    GetProviders() []ProviderInfo
}
```

### Provider Plugin Structure
```
plugins/ffmpeg_software/
├── plugin.cue          # Configuration
├── main.go            # Implementation
├── Dockerfile         # Container build
└── capabilities.go    # Format support
```

### Session Lifecycle
1. **Request**: Client requests playback
2. **Decision**: Analyze media and device capabilities
3. **Provider Selection**: Choose optimal transcoding provider
4. **Session Creation**: Start transcoding session
5. **Streaming**: Deliver content progressively
6. **Cleanup**: Remove temporary files after timeout

## Consequences

### Positive
- **Extensibility**: Easy to add new transcoding providers
- **Scalability**: Can distribute load across multiple providers
- **Flexibility**: Supports various streaming protocols
- **Maintainability**: Clear separation of concerns
- **Performance**: Hardware acceleration when available

### Negative
- **Complexity**: Multiple layers of abstraction
- **Resource Usage**: Transcoding requires significant CPU/GPU
- **Storage**: Temporary files during transcoding

### Trade-offs
- **Flexibility vs Simplicity**: Plugin system adds complexity but enables extensibility
- **Performance vs Compatibility**: Hardware acceleration faster but less portable
- **Real-time vs Pre-transcoded**: Dynamic transcoding uses more resources but saves storage

## Migration Path

For existing systems:
1. Implement service interfaces
2. Wrap existing transcoding logic in provider plugins
3. Add session management layer
4. Implement provider selection algorithm
5. Add cleanup service

## Monitoring

Key metrics to track:
- Active sessions per provider
- Provider selection distribution
- Session success/failure rates
- Resource utilization
- Transcoding performance

## Security Considerations

- Input validation for media paths
- Session authentication/authorization
- Resource limits to prevent DoS
- Secure temporary file handling
- Plugin sandboxing

## Future Enhancements

1. **Distributed Transcoding**: Multi-node support
2. **Quality Adaptation**: Dynamic bitrate adjustment
3. **Pre-transcoding**: Background optimization
4. **CDN Integration**: Direct upload to CDN
5. **ML-based Optimization**: Smart quality predictions