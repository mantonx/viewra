# Transcoding Module

The transcoding module manages all video transcoding operations in Viewra. It provides a unified interface for multiple transcoding providers and implements a two-stage pipeline for adaptive streaming formats.

## Overview

The transcoding module is responsible for:
- Managing transcoding providers (plugins)
- Executing transcoding jobs
- Tracking session lifecycle
- Progress reporting
- Resource management and cleanup
- Implementing the two-stage pipeline (encode → package)

## Architecture

### Module Structure

```
transcodingmodule/
├── api_handler.go          # HTTP API handlers
├── interfaces.go           # Core interfaces and types
├── manager.go              # Main transcoding manager
├── module.go               # Module registration and lifecycle
├── routes.go               # HTTP route definitions
├── service.go              # TranscodingService implementation
├── session_store.go        # Database session management
├── core/
│   ├── cleanup/            # Cleanup service
│   ├── pipeline/           # Two-stage pipeline provider
│   ├── process/            # Process management
│   └── progress/           # Progress tracking
├── providers/
│   ├── ffmpeg/             # FFmpeg provider adapter
│   └── registry.go         # Provider registry
└── utils/
    ├── filemanager/        # File operations
    ├── hash/               # Content hashing
    └── paths/              # Path management
```

### Service Architecture

```
MediaModule
    ↓ (requests transcoding)
TranscodingService (interface)
    ↓ (implemented by)
TranscodingModule
    ↓ (manages)
Provider Registry
    ↓ (contains)
[FFmpeg Provider, Pipeline Provider, Hardware Providers]
```

### Two-Stage Pipeline

The module implements a two-stage transcoding pipeline for DASH/HLS output:

1. **Stage 1: Encoding (70% of progress)**
   - Uses FFmpeg or hardware-accelerated providers
   - Transcodes source to multiple quality MP4 files
   - Output: `/transcode/{contentHash}/transcoding/{sessionId}/encoded/`

2. **Stage 2: Packaging (30% of progress)**
   - Uses Shaka Packager
   - Converts MP4 files to DASH/HLS segments
   - Output: `/transcode/{contentHash}/transcoding/{sessionId}/packaged/`

## Core Components

### Manager

The `Manager` is the central component that:
- Maintains provider registry
- Routes transcoding requests to appropriate providers
- Manages session lifecycle
- Coordinates cleanup operations

### Providers

Providers implement the actual transcoding logic:

- **FFmpeg Provider**: Software transcoding using FFmpeg
- **Pipeline Provider**: Coordinates two-stage DASH/HLS pipeline
- **Hardware Providers**: GPU-accelerated transcoding (NVIDIA, Intel, etc.)

### Session Store

Manages transcoding sessions in the database:
- Tracks session status and progress
- Stores provider information
- Handles content deduplication via SHA256 hashes

### Cleanup Service

Automatic resource management:
- Removes temporary files after configurable retention period
- Cleans up failed session artifacts
- Manages disk space usage
- Kills orphaned processes

## API Endpoints

### Transcoding Operations
- `POST /api/transcode/start` - Start a new transcoding session
- `GET /api/transcode/session/:id` - Get session status
- `POST /api/transcode/stop/:id` - Stop a transcoding session
- `GET /api/transcode/progress/:id` - Get real-time progress

### Management
- `GET /api/transcode/stats` - Get transcoding statistics
- `GET /api/transcode/providers` - List available providers
- `POST /api/transcode/providers/:name/reload` - Reload provider configuration

## Usage Example

```go
// Get the transcoding service
transcodingService := services.GetService[services.TranscodingService]("transcoding")

// Start transcoding
request := &plugins.TranscodeRequest{
    InputPath:   "/media/movies/source.mkv",
    OutputPath:  "/transcode/output/",
    VideoCodec:  "h264",
    AudioCodec:  "aac",
    Qualities: []plugins.Quality{
        {Height: 1080, VideoBitrate: 5000000},
        {Height: 720,  VideoBitrate: 3000000},
    },
}

session, err := transcodingService.StartTranscode(ctx, request)
if err != nil {
    return err
}

// Monitor progress
for {
    progress, err := transcodingService.GetProgress(session.SessionID)
    if err != nil {
        break
    }
    
    log.Printf("Progress: %.2f%%", progress.Progress)
    
    if progress.Progress >= 100 {
        break
    }
    
    time.Sleep(1 * time.Second)
}
```

## Configuration

The module can be configured via environment variables:

```bash
# Base directory for transcoding operations
VIEWRA_TRANSCODE_DIR=/transcode

# Cleanup settings
VIEWRA_TRANSCODE_CLEANUP_ENABLED=true
VIEWRA_TRANSCODE_CLEANUP_INTERVAL=1h
VIEWRA_TRANSCODE_RETENTION_COMPLETED=24h
VIEWRA_TRANSCODE_RETENTION_FAILED=6h

# Provider settings
VIEWRA_FFMPEG_PATH=/usr/bin/ffmpeg
VIEWRA_SHAKA_PACKAGER_PATH=/usr/bin/packager
```

## Provider Development

To add a new transcoding provider:

1. Implement the `TranscodingProvider` interface
2. Register with the provider registry during initialization
3. Handle the provider-specific configuration

Example provider skeleton:

```go
type MyProvider struct {
    name string
    logger hclog.Logger
}

func (p *MyProvider) Transcode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeResponse, error) {
    // Implementation
}

func (p *MyProvider) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
    // Implementation
}

// ... implement other required methods
```

## Content-Addressable Storage

The module uses content-addressable storage based on SHA256 hashes:

- Source files are hashed to create unique content IDs
- Transcoded content is stored under `/transcode/{contentHash}/`
- Enables automatic deduplication
- Multiple sessions can share the same transcoded content

## Process Management

The module includes robust process management:

- Tracks all spawned processes
- Implements graceful shutdown with signal escalation
- Cleans up orphaned processes on startup
- Monitors process health

## Error Handling

The module provides detailed error information:

- Provider-specific error codes
- Process exit codes and stderr capture
- Validation errors for invalid requests
- Resource limitation errors

## Testing

The module includes comprehensive tests:

```bash
cd internal/modules/transcodingmodule
go test ./...
```

Key test areas:
- Provider registration and selection
- Session lifecycle management
- Cleanup service operation
- Process management
- Progress tracking

## Future Enhancements

Planned improvements:
- Distributed transcoding across multiple nodes
- Priority queue for transcoding jobs
- Advanced quality selection algorithms
- Resume capability for interrupted jobs
- Real-time streaming support
- Hardware capability auto-detection