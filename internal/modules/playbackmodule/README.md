# Playback Module

The playback module provides intelligent media playback decisions, progressive download support, session management, and user behavior tracking for Viewra. It orchestrates the streaming experience with device-aware logic and comprehensive analytics.

## Overview

The playback module is responsible for:
- Making intelligent playback decisions (direct play, remux, or transcode)
- Managing progressive download with HTTP range support
- Tracking playback sessions and user interactions
- Providing device-specific compatibility checking
- Managing watch history and progress tracking
- Coordinating with transcoding for incompatible media
- Cleaning up resources and managing transcode cache

## Architecture

### Clean Architecture Design

```
playbackmodule/
├── api/                    # HTTP handlers (presentation layer)
│   ├── handlers.go        # Base handler struct
│   ├── decision_handlers.go    # Playback decision endpoints
│   ├── session_handlers.go     # Session management
│   ├── streaming_handlers.go   # Progressive download
│   ├── analytics_handlers.go   # Analytics endpoints
│   └── routes.go              # Route registration
├── core/                   # Business logic (domain layer)
│   ├── playback/          # Playback orchestration
│   │   ├── manager.go     # Main coordinator
│   │   ├── decision_engine.go      # Playback decisions
│   │   ├── device_detector.go      # Device capabilities
│   │   ├── recommendation_tracker.go # User interactions
│   │   └── transcode_deduplicator.go # Cache management
│   ├── session/           # Session management
│   │   ├── manager.go     # Session lifecycle
│   │   └── tracker.go     # Event tracking
│   ├── streaming/         # Progressive download
│   │   ├── manager.go     # Streaming coordinator
│   │   └── progressive_handler.go  # Range requests
│   ├── history/           # Watch history
│   │   ├── manager.go     # History coordinator
│   │   └── media_history.go # Media-aware tracking
│   ├── cleanup/           # Resource management
│   │   └── manager.go     # Cleanup operations
│   └── repository/        # Data access layer
│       ├── session_repository.go
│       └── history_repository.go
├── service/               # Service interface implementation
│   └── playback_service.go # Thin wrapper implementing PlaybackService
├── models/                # Database models
│   └── models.go         # All playback-related models
├── types/                 # Type definitions
│   ├── playback.go       # Playback types
│   └── analytics.go      # Analytics types
└── utils/                 # Utility functions
    └── compatibility.go   # Codec compatibility checks
```

### Service Architecture

```
External Consumers (Frontend, Apps)
    ↓
Service Registry (services.PlaybackService interface)
    ↓
PlaybackService Implementation (thin wrapper)
    ↓
Core Managers (orchestrate operations)
    ↓
Domain Components:
├── Decision Engine (playback logic)
├── Session Manager (lifecycle)
├── Progressive Handler (streaming)
├── History Manager (tracking)
└── Cleanup Manager (maintenance)
    ↓
Dependencies:
├── Media Service (file info)
└── Transcoding Service (conversions)
```

## Core Concepts

### Playback Decision Engine
Determines the optimal playback method based on:
- **Device Capabilities**: Browser, app, TV support
- **Media Format**: Container, codecs, resolution
- **Network Conditions**: Bandwidth considerations
- **User Preferences**: Quality settings

Decision outcomes:
- **Direct Play**: File compatible with device
- **Remux**: Only container needs changing
- **Transcode**: Full conversion required

### Progressive Download
HTTP-based streaming without complex protocols:
- **Range Requests**: Byte-range support for seeking
- **Resume Support**: Continue interrupted downloads
- **Bandwidth Efficiency**: Stream while downloading
- **Universal Compatibility**: Works with any HTTP client

### Session Management
Comprehensive tracking of playback sessions:
- **Lifecycle Events**: Start, pause, resume, stop
- **Progress Tracking**: Automatic position saving
- **Analytics**: Duration, completion rates
- **Multi-device**: Resume across devices

### History & Analytics
Smart tracking with media-aware logic:
- **Watch Progress**: Per-media position tracking
- **Completion Logic**: Different for movies vs episodes
- **User Patterns**: Viewing habits and preferences
- **Recommendations**: Data for future ML integration

## API Endpoints

### Playback Decisions

#### Get Playback Decision
```http
POST /api/playback/decision
```

Request Body:
```json
{
  "mediaFileId": "file-123",
  "deviceProfile": {
    "type": "browser",
    "supportedVideoCodecs": ["h264"],
    "supportedAudioCodecs": ["aac", "mp3"],
    "supportedContainers": ["mp4", "webm"],
    "maxResolution": {"width": 1920, "height": 1080}
  }
}
```

Response:
```json
{
  "method": "transcode",
  "reason": "incompatible video codec: hevc",
  "transcodeRequest": {
    "container": "mp4",
    "videoCodec": "h264",
    "audioCodec": "aac"
  }
}
```

### Session Management

#### Start Playback Session
```http
POST /api/playback/sessions/start
```

#### Update Progress
```http
POST /api/playback/sessions/{sessionId}/progress
```

#### End Session
```http
POST /api/playback/sessions/{sessionId}/end
```

#### Get Active Sessions
```http
GET /api/playback/sessions/active
```

### Progressive Download

#### Stream Content
```http
GET /api/playback/progressive/{contentId}
```

Headers:
- `Range: bytes=0-1048575` - Request specific byte range
- Returns `206 Partial Content` with requested range

### Analytics

#### Submit Analytics Event
```http
POST /api/analytics/session
```

Request Body:
```json
{
  "sessionId": "session-123",
  "events": [{
    "type": "buffer",
    "timestamp": "2024-01-01T12:00:00Z",
    "data": {"duration": 2.5}
  }]
}
```

### History

#### Get Watch History
```http
GET /api/playback/history
```

Query Parameters:
- `user_id`: User identifier
- `media_type`: Filter by type
- `limit`, `offset`: Pagination

## Implementation Details

### Clean Architecture Benefits

1. **Separation of Concerns**:
   - Each manager handles specific domain
   - Clear boundaries between layers
   - Service layer is just an adapter

2. **Testability**:
   - Business logic testable without HTTP
   - Mock dependencies easily
   - Domain logic isolated

3. **Flexibility**:
   - Easy to add new device types
   - Extensible decision logic
   - Plugin points for ML/recommendations

### Key Components

#### Decision Engine (core/playback/decision_engine.go)
Makes intelligent playback decisions:
- Analyzes media compatibility
- Considers device capabilities
- Optimizes for user experience
- Provides detailed reasoning

#### Progressive Handler (core/streaming/progressive_handler.go)
Implements HTTP range streaming:
- Handles byte-range requests
- Manages file access
- Provides resume support
- Optimizes read buffers

#### Session Manager (core/session/manager.go)
Manages playback session lifecycle:
- Creates and tracks sessions
- Handles state transitions
- Persists progress
- Manages cleanup

#### History Manager (core/history/manager.go)
Tracks viewing history:
- Records watch progress
- Implements completion logic
- Aggregates statistics
- Provides history queries

## Usage Examples

### Making a Playback Decision

```go
decision, err := playbackService.MakeDecision(ctx, &types.DecisionRequest{
    MediaFileID: "movie-123",
    DeviceProfile: &types.DeviceProfile{
        Type: "browser",
        SupportedVideoCodecs: []string{"h264"},
        SupportedAudioCodecs: []string{"aac"},
    },
})

switch decision.Method {
case "direct":
    // Serve file directly
case "transcode":
    // Start transcoding with decision.TranscodeRequest
}
```

### Progressive Streaming

```go
// Client requests with Range header
rangeHeader := "bytes=1048576-2097151"
response := progressiveHandler.ServeContent(
    mediaFile,
    rangeHeader,
    responseWriter,
)
```

### Session Tracking

```go
// Start session
session, err := sessionManager.StartSession(ctx, &models.PlaybackSession{
    UserID:      "user-123",
    MediaFileID: "file-456",
    DeviceID:    "device-789",
})

// Update progress
err = sessionManager.UpdateProgress(ctx, session.ID, 120.5, 0.25)
```

## Performance Considerations

### Decision Caching
- Cache compatibility checks
- Reuse device profiles
- Minimize repeated analysis

### Streaming Optimization
- Configurable buffer sizes
- Efficient file reading
- Connection pooling

### Session Management
- Batch progress updates
- Efficient queries
- Periodic cleanup

## Configuration

Environment variables:
```bash
# Session Management
PLAYBACK_SESSION_TIMEOUT=6h
PLAYBACK_PROGRESS_INTERVAL=30s

# Cleanup
PLAYBACK_CLEANUP_INTERVAL=1h
PLAYBACK_CLEANUP_MAX_AGE=7d

# Streaming
PLAYBACK_BUFFER_SIZE=4MB
PLAYBACK_MAX_CONNECTIONS=100

# Analytics
PLAYBACK_ANALYTICS_BATCH_SIZE=100
PLAYBACK_ANALYTICS_FLUSH_INTERVAL=5m
```

## Testing

### Unit Tests
```bash
go test ./core/...              # Test business logic
go test ./core/playback/        # Test decision engine
go test ./core/streaming/       # Test progressive handler
```

### Integration Tests
```bash
go test ./tests/                # Full integration tests
```

## Media Type Awareness

The module handles different media types intelligently:

### Movies
- Single progress tracking
- 90% watched = complete
- Simple history entries

### TV Episodes  
- Per-episode tracking
- Series progress aggregation
- Next episode suggestions

### Music
- Different completion criteria
- Playlist support
- Repeat tracking

## Future Enhancements

- Machine learning recommendations
- Adaptive bitrate support
- Multi-user session coordination
- Advanced analytics dashboard
- Bandwidth prediction
- Offline download management
- Social viewing features