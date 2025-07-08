# Transcoding Module

The transcoding module provides file-based media transcoding functionality for Viewra. It implements a simplified, reliable approach focused on complete file transcoding rather than complex real-time streaming.

## Overview

The transcoding module is responsible for:
- Converting media files to different formats, codecs, and resolutions
- Managing transcoding sessions with database persistence
- Implementing content-addressable storage for deduplication
- Providing progress tracking and session management
- Handling resource cleanup and process management
- Serving transcoded content via HTTP APIs

## Architecture

### Core Components

```
TranscodingModule
├── Manager                 # Central coordinator
├── TranscodingService     # Public service interface
├── API Handlers           # HTTP endpoints
├── Core Components:
│   ├── Pipeline/          # File-based transcoding implementation
│   ├── Session/           # Session management and persistence
│   ├── Storage/           # Content-addressable storage
│   ├── Cleanup/           # Resource management
│   └── FFmpeg/            # FFmpeg integration
└── Utilities/             # Helper components
```

### Service Architecture

```
Media Module
    ↓ (requests transcoding)
TranscodingService (interface)
    ↓ (implemented by)
TranscodingModule
    ↓ (manages)
File Pipeline Provider
    ↓ (uses)
[FFmpeg, Session Store, Content Store]
```

## Core Concepts

### File-Based Transcoding

The module uses a straightforward file-based approach:

1. **Input**: Source media file (MP4, MKV, AVI, etc.)
2. **Processing**: FFmpeg transcoding with specified parameters
3. **Output**: Complete transcoded file(s) in target format
4. **Storage**: Content-addressable storage using SHA256 hashes

### Content-Addressable Storage

All transcoded content is stored using SHA256 hashes for automatic deduplication:

```
/app/viewra-data/transcoding/
├── content/
│   └── {sha256_hash}/     # Content-addressable storage
│       ├── output.mp4     # Transcoded file
│       └── metadata.json  # Content metadata
└── sessions/
    └── {session_id}/      # Temporary session files
```

### Session Management

Each transcoding operation creates a persistent session:

- **Database tracking**: Status, progress, metadata
- **Temporary workspace**: Isolated session directory
- **Content deduplication**: Reuse existing transcodes
- **Cleanup**: Automatic removal of old sessions

## API Endpoints

### Transcoding Operations

#### Start Transcoding
```http
POST /api/v1/transcoding/transcode
Content-Type: application/json

{
    "input_path": "/media/movies/source.mkv",
    "media_id": "movie_12345",
    "container": "mp4",
    "video_codec": "h264",
    "audio_codec": "aac",
    "resolution": {"width": 1920, "height": 1080},
    "video_bitrate": 5000000,
    "audio_bitrate": 128000
}
```

Response:
```json
{
    "session_id": "uuid-session-id",
    "status": "running",
    "content_hash": "sha256-hash"
}
```

#### Get Session Status
```http
GET /api/v1/transcoding/session/{session_id}
```

Response:
```json
{
    "session_id": "uuid-session-id",
    "status": "running",
    "progress": {
        "percent_complete": 45.5,
        "time_elapsed": "00:02:30",
        "time_remaining": "00:03:15"
    },
    "created_at": "2023-01-01T12:00:00Z"
}
```

#### Stop Transcoding
```http
POST /api/v1/transcoding/session/{session_id}/stop
```

### Content Access

#### Serve Transcoded Content
```http
GET /api/v1/content/{content_hash}/{filename}
```

#### Get Content Metadata
```http
GET /api/v1/content/{content_hash}/info
```

### Management

#### Get Statistics
```http
GET /api/v1/transcoding/stats
```

#### List Active Sessions
```http
GET /api/v1/transcoding/sessions
```

#### Get Resource Usage
```http
GET /api/v1/transcoding/resources
```

Response:
```json
{
    "active_sessions": 2,
    "max_sessions": 4,
    "queued_requests": 1,
    "max_queue_size": 20,
    "total_memory_mb": 850,
    "session_details": [
        {
            "session_id": "uuid-session-id",
            "media_id": "movie_12345",
            "provider": "streaming_pipeline",
            "status": "running",
            "duration": "00:02:30",
            "estimated_memory_mb": 450
        }
    ]
}
```

## Configuration

### Environment Variables

```bash
# Base directories
VIEWRA_DATA_DIR=/app/viewra-data
VIEWRA_TRANSCODE_DIR=/app/viewra-data/transcoding

# FFmpeg settings
FFMPEG_PATH=/usr/bin/ffmpeg
FFMPEG_PRESET=fast

# Cleanup settings
TRANSCODING_CLEANUP_ENABLED=true
TRANSCODING_CLEANUP_INTERVAL=1h
TRANSCODING_RETENTION_COMPLETED=24h
TRANSCODING_RETENTION_FAILED=6h
TRANSCODING_RETENTION_STALE=2h

# Session limits and resource management
MAX_CONCURRENT_SESSIONS=4
SESSION_TIMEOUT=30m
MAX_QUEUE_SIZE=20
QUEUE_TIMEOUT=10m
```

### Database Configuration

The module automatically creates required database tables:
- `transcode_sessions`: Session tracking and metadata
- Indexes for efficient querying and cleanup

## Usage Examples

### Basic Transcoding

```go
// Get transcoding service
transcodingService := services.GetService[services.TranscodingService]("transcoding")

// Create transcode request
request := &plugins.TranscodeRequest{
    InputPath:    "/media/source.mkv",
    MediaID:      "movie_123",
    Container:    "mp4",
    VideoCodec:   "h264",
    AudioCodec:   "aac",
    Resolution:   &plugins.Resolution{Width: 1920, Height: 1080},
    VideoBitrate: 5000000,
    AudioBitrate: 128000,
}

// Start transcoding
session, err := transcodingService.StartTranscode(ctx, request)
if err != nil {
    return fmt.Errorf("failed to start transcode: %w", err)
}

log.Printf("Started transcoding session: %s", session.SessionID)
```

### Progress Monitoring

```go
// Monitor progress
for {
    progress, err := transcodingService.GetProgress(session.SessionID)
    if err != nil {
        break
    }
    
    log.Printf("Progress: %.1f%%", progress.PercentComplete)
    
    if progress.PercentComplete >= 100.0 {
        log.Printf("Transcoding completed")
        break
    }
    
    time.Sleep(5 * time.Second)
}
```

### Content Access

```go
// Get content information
contentInfo, err := transcodingService.GetContentInfo(session.ContentHash)
if err != nil {
    return err
}

// Content is now available at:
// /api/v1/content/{content_hash}/output.mp4
contentURL := fmt.Sprintf("/api/v1/content/%s/output.mp4", session.ContentHash)
```

## Implementation Details

### File Pipeline Provider

The core transcoding engine (`core/pipeline/file_pipeline.go`):

- **Session Creation**: Generates unique session IDs and workspaces
- **Content Deduplication**: Checks existing content before transcoding
- **FFmpeg Execution**: Runs FFmpeg with optimized parameters
- **Progress Tracking**: Estimates progress based on session status
- **Content Storage**: Moves completed files to content-addressable storage
- **Cleanup**: Removes temporary files after completion

### Session Store

Database-backed session management (`core/session/store.go`):

- **Persistence**: All sessions stored in database
- **Content Hashing**: Deterministic hashes for deduplication
- **Status Tracking**: Real-time status updates
- **Cleanup Integration**: Automatic cleanup of expired sessions

### Content Store

Content-addressable storage system (`core/storage/content_store.go`):

- **SHA256 Hashing**: Unique content identification
- **Metadata Storage**: File information and transcoding parameters
- **Efficient Serving**: Direct file serving with proper HTTP headers
- **Deduplication**: Automatic reuse of existing content

### Cleanup Service

Automatic resource management (`core/cleanup/cleanup_service.go`):

- **Session Cleanup**: Removes old completed/failed sessions
- **File Cleanup**: Deletes temporary and expired files
- **Process Cleanup**: Terminates orphaned FFmpeg processes
- **Configurable Retention**: Different retention policies by status

## Error Handling

The module provides comprehensive error handling:

### Common Error Types

- **Validation Errors**: Invalid input parameters
- **File Errors**: Missing input files, permission issues
- **Process Errors**: FFmpeg execution failures
- **Resource Errors**: Disk space, memory limitations
- **Timeout Errors**: Long-running session timeouts

### Error Response Format

```json
{
    "error": "transcoding_failed",
    "message": "FFmpeg process failed with exit code 1",
    "details": {
        "session_id": "uuid",
        "exit_code": 1,
        "stderr": "FFmpeg error output"
    }
}
```

## Performance Considerations

### Resource Management

- **Concurrent Sessions**: Configurable limit to prevent resource exhaustion
- **Request Queuing**: Automatic queuing when session limits are exceeded
- **Memory Estimation**: Tracks estimated memory usage per session
- **Session Monitoring**: Automatic cleanup of completed/failed sessions
- **Queue Management**: Configurable queue size and timeout handling
- **Disk Space**: Automatic cleanup of old files
- **CPU Usage**: Process priority and nice values

### Optimization

- **Content Deduplication**: Avoid redundant transcoding
- **Fast Start**: MP4 faststart optimization for web playback
- **Efficient Codecs**: H.264/H.265 with optimized presets
- **Parallel Processing**: Multiple concurrent sessions

## Testing

### Unit Tests

```bash
# Run all transcoding module tests
cd internal/modules/transcodingmodule
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test suites
go test ./core/pipeline/
go test ./core/session/
go test ./core/storage/
```

### Integration Tests

```bash
# Run integration tests with real media files
go test ./integration_tests/

# Test specific scenarios
go test -run TestFilePipeline ./integration_tests/
go test -run TestContentStore ./integration_tests/
```

### Manual Testing

```bash
# Start development environment
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Test transcoding endpoint
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/app/test-media/sample.mp4",
    "media_id": "test_video",
    "container": "mp4",
    "video_codec": "h264"
  }'
```

## Troubleshooting

### Common Issues

1. **FFmpeg Not Found**
   - Ensure FFmpeg is installed in the container
   - Check `FFMPEG_PATH` environment variable

2. **Permission Errors**
   - Verify file permissions on input media
   - Check write permissions on transcoding directory

3. **Session Timeouts**
   - Monitor long-running sessions
   - Adjust `SESSION_TIMEOUT` for large files

4. **Disk Space Issues**
   - Enable cleanup service
   - Monitor available disk space
   - Adjust retention policies

### Debug Mode

Enable debug logging:

```bash
export LOG_LEVEL=debug
export TRANSCODING_DEBUG=true
```

### Monitoring

Key metrics to monitor:
- Active session count
- Average transcoding duration
- Error rates by type
- Disk space usage
- FFmpeg process memory usage

## Future Enhancements

Planned improvements:
- **Hardware Acceleration**: NVIDIA NVENC, Intel QSV support
- **Priority Queuing**: High/low priority transcoding queues
- **Resume Capability**: Resume interrupted transcoding sessions
- **Distributed Processing**: Multi-node transcoding support
- **Advanced Quality**: CRF-based quality control
- **Format Support**: Additional codecs and containers
- **Real-time Progress**: FFmpeg stderr parsing for accurate progress