# Transcoding Module

The transcoding module provides video and audio transcoding capabilities for Viewra. It manages file-based transcoding operations with content-addressable storage, session persistence, and resource management.

## Overview

The transcoding module is responsible for:
- Converting media files to different formats, codecs, and resolutions
- Managing transcoding sessions with database persistence
- Implementing content-addressable storage for deduplication
- Providing progress tracking and session management
- Handling resource cleanup and process management
- Serving transcoded content via HTTP APIs

## Architecture

### Clean Architecture Design

```
transcodingmodule/
├── api/                    # HTTP handlers (presentation layer)
│   ├── handlers.go        # Main API handlers
│   ├── content_handler.go # Content serving endpoints
│   ├── interfaces.go      # API interfaces
│   └── routes.go          # Route registration
├── core/                   # Business logic (domain layer)
│   ├── transcoding/       # Main transcoding orchestration
│   │   ├── manager.go     # Central coordinator
│   │   ├── provider_registry.go # Provider management
│   │   ├── process/       # Process management
│   │   └── resource/      # Resource management
│   ├── session/           # Session management
│   ├── storage/           # Content-addressable storage
│   ├── pipeline/          # Transcoding pipeline
│   ├── cleanup/           # Cleanup services
│   └── repository/        # Data access layer
├── service/               # Service interface implementation
│   └── transcoding_service.go # Thin wrapper implementing TranscodingService
├── types/                 # All type definitions
│   ├── config.go         # Configuration types
│   ├── request.go        # Request types
│   ├── result.go         # Result types
│   ├── session.go        # Session types
│   ├── status.go         # Status types
│   ├── profile.go        # Encoding profiles
│   ├── errors.go         # Error types
│   └── interfaces.go     # Module interfaces
├── utils/                 # Utility functions
│   ├── ffmpeg/           # FFmpeg integration
│   ├── hardware/         # Hardware detection
│   ├── progress/         # Progress parsing
│   ├── system/           # System utilities
│   └── validation/       # Input validation
└── errors/               # Error handling
    └── errors.go         # Centralized error types
```

### Service Architecture

```
External Consumers (Playback Module, Frontend)
    ↓
Service Registry (services.TranscodingService interface)
    ↓
TranscodingService Implementation (thin wrapper)
    ↓
Transcoding Manager (orchestrates operations)
    ↓
Core Components:
├── Provider Registry (provider selection)
├── Session Store (persistence)
├── Content Store (file storage)
├── Resource Manager (concurrency)
└── Cleanup Service (maintenance)
    ↓
FFmpeg Process (actual transcoding)
```

## Core Concepts

### File-Based Transcoding
The module uses a complete file transcoding approach rather than real-time streaming:
- **Reliability**: Complete files before serving
- **Simplicity**: No complex streaming protocols
- **Quality**: Consistent output quality
- **Compatibility**: Works with any HTTP client

### Content-Addressable Storage
Transcoded files are stored using content hashes:
- **Deduplication**: Same content = same hash
- **Efficiency**: Avoid re-transcoding identical content
- **Organization**: Clean directory structure by hash

### Session Management
Every transcoding operation is tracked as a session:
- **Persistence**: Sessions stored in database
- **Progress**: Real-time progress updates
- **History**: Complete audit trail
- **Cleanup**: Automatic resource management

## API Endpoints

### Transcoding Operations

#### Start Transcoding
```http
POST /api/v1/transcode
```

Request Body:
```json
{
  "mediaId": "file-123",
  "container": "mp4",
  "videoCodec": "h264",
  "audioCodec": "aac",
  "resolution": {"width": 1920, "height": 1080},
  "quality": 23,
  "enableABR": false
}
```

#### Get Session Info
```http
GET /api/v1/transcode/sessions/{sessionId}
```

#### Get Progress
```http
GET /api/v1/transcode/sessions/{sessionId}/progress
```

#### Stop Transcoding
```http
DELETE /api/v1/transcode/sessions/{sessionId}
```

### Content Serving

#### Get Content by Hash
```http
GET /api/v1/content/{contentHash}
GET /api/v1/content/{contentHash}/manifest.mpd
GET /api/v1/content/{contentHash}/{filename}
```

### Management

#### List Providers
```http
GET /api/v1/transcode/providers
```

#### Get Statistics
```http
GET /api/v1/transcode/stats
```

## Implementation Details

### Clean Architecture Benefits

1. **Separation of Concerns**:
   - API handlers only handle HTTP concerns
   - Business logic isolated in core package
   - Service is a thin adapter layer

2. **Testability**:
   - Core logic can be tested without HTTP
   - Mock providers for unit tests
   - Clear interfaces for all components

3. **Extensibility**:
   - Easy to add new providers
   - Plugin-based architecture
   - Clear extension points

### Key Components

#### Transcoding Manager (core/transcoding/manager.go)
Orchestrates all transcoding operations:
- Provider selection and management
- Session lifecycle management
- Resource allocation
- Progress monitoring

#### Session Store (core/session/store.go)
Handles session persistence:
- Database operations
- Session state management
- Content hash tracking

#### Content Store (core/storage/content_store.go)
Manages transcoded files:
- Content-addressable storage
- File organization
- Metadata management

#### File Pipeline (core/pipeline/file_pipeline.go)
Implements actual transcoding:
- FFmpeg process management
- Progress parsing
- Error handling

## Usage Examples

### Starting a Transcode

```go
req := &plugins.TranscodeRequest{
    MediaID:    "movie-123",
    Container:  "mp4",
    VideoCodec: "h264",
    AudioCodec: "aac",
    Quality:    23,
}

handle, err := transcodingService.StartTranscode(ctx, req)
```

### Monitoring Progress

```go
progress, err := transcodingService.GetProgress(handle.SessionID)
fmt.Printf("Progress: %.2f%%\n", progress.PercentComplete)
```

### Serving Content

```go
// Get content URL from session
session, err := transcodingService.GetSession(sessionID)
contentURL := fmt.Sprintf("/api/v1/content/%s", session.ContentHash)
```

## Performance Considerations

### Resource Management
- Configurable concurrent session limits
- Process priority management
- Memory usage monitoring
- Disk space management

### Optimization Strategies
- Content deduplication
- Hardware acceleration support
- Efficient progress parsing
- Batch cleanup operations

## Configuration

The module uses configuration for tuning:

```go
type Config struct {
    TranscodingDir        string        // Base directory for files
    MaxConcurrentSessions int          // Concurrent limit
    SessionTimeout        time.Duration // Max session duration
    CleanupInterval       time.Duration // Cleanup frequency
    RetentionPeriod       time.Duration // File retention
}
```

Environment variables:
```bash
VIEWRA_TRANSCODING_DIR=/app/viewra-data/transcoding
TRANSCODING_MAX_CONCURRENT=5
TRANSCODING_SESSION_TIMEOUT=2h
TRANSCODING_CLEANUP_INTERVAL=30m
```

## Testing

### Unit Tests
```bash
go test ./core/...           # Test business logic
go test ./core/pipeline/     # Test pipeline
go test ./core/storage/      # Test storage
```

### Integration Tests
```bash
go test ./integration_tests/ # Full integration tests
```

## Troubleshooting

### Common Issues

1. **FFmpeg Not Found**
   - Ensure FFmpeg is installed in container
   - Check PATH environment variable

2. **Permission Errors**
   - Check directory permissions
   - Ensure write access to transcoding directory

3. **Resource Exhaustion**
   - Monitor concurrent sessions
   - Check disk space
   - Review memory usage

### Debug Logging

Enable debug logging:
```bash
LOG_LEVEL=debug
TRANSCODING_DEBUG=true
```

## Future Enhancements

- GPU acceleration support
- Distributed transcoding
- Advanced quality presets
- Streaming protocol support (DASH/HLS)
- Transcoding queue management
- Priority-based scheduling