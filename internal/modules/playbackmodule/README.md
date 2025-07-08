# Playback Module

The Playback Module is responsible for intelligent media playback decisions, progressive download support, session management, and user behavior tracking. It provides a clean, modern approach to media streaming without complex legacy protocols.

## Overview

This module handles the core playback functionality for Viewra's media management platform, supporting both video and audio content with device-aware streaming capabilities.

### Key Features

- **Intelligent Playback Decisions**: Automatically chooses between direct play, remux, or transcode based on device capabilities
- **Progressive Download**: HTTP range-based streaming with resume support
- **Session Management**: Comprehensive playback session tracking and analytics
- **Device Compatibility**: Support for browsers, native apps, TVs, and streaming devices
- **Music-Aware Logic**: Specialized handling for audio content with different completion criteria
- **Deduplication**: Smart transcode caching to avoid redundant processing
- **Future-Ready**: Designed for easy integration with recommendation engines

## Architecture

### Core Components

```
playbackmodule_new/
├── api/                    # HTTP API handlers
├── core/                   # Core business logic
│   ├── decision_engine.go  # Playback method decisions
│   ├── progressive_handler.go  # Range request handling
│   ├── session_manager.go  # Session lifecycle
│   ├── cleanup_manager.go  # Resource cleanup
│   ├── history_manager.go  # User history coordination
│   ├── media_history.go    # Media-aware history tracking
│   ├── session_tracker.go  # Session event tracking
│   ├── recommendation_tracker.go  # User interaction tracking
│   └── transcode_deduplicator.go  # Transcode optimization
├── models/                 # Database models
├── service/                # Service layer implementation
├── types/                  # Type definitions
└── utils/                  # Module-specific utilities
```

### Service Dependencies

- **MediaService**: Media file information and metadata
- **TranscodingService**: On-demand transcoding capabilities
- **DatabaseModule**: Persistent storage for sessions and history

## How It Works

### 1. Playback Decision Engine

The decision engine analyzes media files and device capabilities to determine the optimal playback method:

```go
decision, err := playbackService.DecidePlayback(mediaPath, deviceProfile)
```

**Decision Logic:**
- **Direct Play**: Native format support, no processing needed
- **Remux**: Container change needed, no re-encoding  
- **Transcode**: Full re-encoding required for compatibility

### 2. Progressive Download

Supports HTTP range requests for efficient streaming:

```go
// Handles partial content requests
Range: bytes=0-1023
Content-Range: bytes 0-1023/2048
```

**Features:**
- Resume support for interrupted downloads
- Efficient seeking in large media files
- Bandwidth-aware streaming

### 3. Session Management

Tracks comprehensive playback sessions:

```go
session, err := playbackService.StartPlaybackSession(mediaFileID, userID, deviceID, method)
```

**Session Data:**
- Playback position and duration
- Device information and capabilities
- Quality settings and bandwidth
- User interactions and events

### 4. Music-Aware History

Different completion criteria for different media types:

**Audio Content (Music/Podcasts):**
- 70% completion OR
- 30 seconds + 50% completion OR  
- 90 seconds minimum

**Video Content:**
- Traditional 90% completion rule

## API Endpoints

### Playback Control

```http
POST /api/playback/decide
POST /api/playback/session/start
PUT  /api/playback/session/{id}
POST /api/playback/session/{id}/end
GET  /api/playback/session/{id}
```

### Streaming

```http
GET /api/playback/stream/direct/{id}
GET /api/playback/stream/remux/{id}
GET /api/playback/stream/transcode/{id}
```

### History & Analytics

```http
GET /api/playback/history/{userID}
GET /api/playback/recently-played/{userID}
GET /api/playback/incomplete/{userID}
GET /api/playback/stats/{userID}
```

## Database Models

### Core Models

- **PlaybackSession**: Active playback sessions with real-time state
- **PlaybackHistory**: Denormalized history for quick queries
- **UserMediaProgress**: Resume positions and completion tracking
- **SessionEvent**: Granular playback events (play, pause, seek, etc.)

### Analytics Models

- **PlaybackAnalytics**: Aggregated daily statistics
- **UserPlaybackStats**: User-specific aggregated metrics
- **MediaInteraction**: Detailed user interaction tracking

### Recommendation-Ready Models

- **UserPreferences**: Learned user preferences
- **MediaFeatures**: Extracted content features
- **UserVector**: User preference vectors for ML
- **RecommendationCache**: Pre-calculated recommendations

### Optimization Models

- **TranscodeCache**: Deduplication and caching
- **TranscodeCleanupTask**: Resource management

## Usage Examples

### Basic Playback Flow

```go
// 1. Decide playback method
decision, err := playbackService.DecidePlayback("/path/to/video.mkv", deviceProfile)

// 2. Start session
session, err := playbackService.StartPlaybackSession(mediaFileID, userID, deviceID, decision.Method)

// 3. Get streaming URL
streamURL, err := playbackService.PrepareStreamURL(decision, baseURL)

// 4. Update session during playback
updates := map[string]interface{}{
    "position": 1800, // 30 minutes
    "state": "playing",
}
err = playbackService.UpdatePlaybackSession(session.ID, updates)

// 5. End session
err = playbackService.EndPlaybackSession(session.ID)
```

### Device Profile Examples

```go
// Browser
browserProfile := &types.DeviceProfile{
    SupportedCodecs: []string{"h264", "vp9"},
    MaxResolution: "1080p",
    SupportsHEVC: false,
}

// Apple TV
appleTVProfile := &types.DeviceProfile{
    SupportedCodecs: []string{"h264", "hevc"},
    MaxResolution: "4K",
    SupportsHEVC: true,
    SupportsHDR: true,
}
```

### History Queries

```go
// Get recent playback history
history, err := playbackService.GetUserPlaybackHistory(userID, "video", 20)

// Get incomplete videos for resume
incomplete, err := historyManager.GetIncompleteVideos(userID, 10)

// Get most played music
music, err := historyManager.GetMostPlayedMusic(userID, 50)
```

## Integration with Other Modules

### Media Module Integration

```go
// Get media file information
mediaInfo, err := playbackService.GetMediaInfo(mediaPath)

// Validate playback compatibility  
err = playbackService.ValidatePlayback(mediaPath, deviceProfile)
```

### Transcoding Module Integration

```go
// Get recommended transcode parameters
params, err := playbackService.GetRecommendedTranscodeParams(mediaPath, deviceProfile)

// Start transcode session
session, err := transcodingService.StartTranscode(ctx, params)
```

### Future Recommendation Engine

The module is designed for easy integration with recommendation engines:

```go
// Register for playback events
playbackService.RegisterEventHandler(recommendationEngine)

// Access user behavior data
preferences, err := playbackService.GetUserPreferences(userID)
interactions, err := playbackService.GetUserInteractionHistory(userID, 500)
```

## Configuration

### Environment Variables

```env
PLAYBACK_CLEANUP_INTERVAL=1h
PLAYBACK_SESSION_TIMEOUT=24h
PLAYBACK_TRANSCODE_CACHE_SIZE=1000
PLAYBACK_CACHE_MAX_AGE=24h
```

### Cleanup Configuration

```go
type CleanupConfig struct {
    SessionTimeout    time.Duration // 24 hours
    TranscodeTimeout  time.Duration // 1 hour  
    CleanupInterval   time.Duration // 1 hour
    MaxCacheSize      int           // 1000 entries
    CacheMaxAge       time.Duration // 24 hours
}
```

## Monitoring & Observability

### Metrics

- **playback_sessions_active**: Current active sessions
- **playback_decisions_total**: Playback decisions by method
- **playback_completion_rate**: Content completion rates
- **transcode_deduplication_ratio**: Cache hit ratio
- **cleanup_operations_total**: Resource cleanup operations

### Logging

```go
logger.Info("Playback decision made",
    "method", decision.Method,
    "mediaType", mediaType,
    "deviceType", deviceProfile.Type,
    "reason", decision.Reason)
```

### Health Checks

- Database connectivity
- Cleanup manager status  
- Active session count
- Cache hit ratios

## Development

### Running Tests

```bash
# Unit tests
go test ./internal/modules/playbackmodule_new/...

# Integration tests  
go test -tags=integration ./internal/modules/playbackmodule_new/...

# With coverage
go test -cover ./internal/modules/playbackmodule_new/...
```

### Adding New Device Profiles

1. Define device capabilities in `types/device_profile.go`
2. Update decision logic in `core/decision_engine.go`
3. Add device-specific tests
4. Update API documentation

### Adding New Media Types

1. Update completion logic in `core/media_history.go`
2. Add quality assessment in `utils/media.go`
3. Update database models if needed
4. Add media-type-specific tests

## Utilities

### Module-Specific Utilities (`utils/`)

- **HTTP Range Handling**: Parse and format HTTP range headers
- **Media Analysis**: Audio/video detection, codec identification
- **Quality Assessment**: Bitrate recommendations, format optimization
- **Resolution Handling**: Parse and format resolution strings

### Shared Utilities (`/internal/utils`)

- **UUID Generation**: Session and request IDs
- **Content Type Detection**: MIME type identification
- **Media File Validation**: Supported format checking

## Security Considerations

- **Path Validation**: Prevent directory traversal attacks
- **Range Request Limits**: Prevent abuse of partial content requests
- **Session Isolation**: User sessions are properly isolated
- **Input Sanitization**: All user inputs are validated and sanitized

## Performance Optimization

- **Transcode Deduplication**: Avoid redundant processing
- **Database Indexing**: Optimized queries for user history
- **Connection Pooling**: Efficient database connections
- **Memory Management**: Cleanup of expired sessions and cache entries

## Future Enhancements

- **Adaptive Bitrate Streaming**: Quality switching based on bandwidth
- **Recommendation Integration**: ML-based content suggestions
- **Advanced Analytics**: User behavior insights and reporting
- **Multi-CDN Support**: Geographic content distribution
- **Live Streaming**: Real-time content streaming capabilities

---

This module provides a solid foundation for media playback while maintaining flexibility for future enhancements and integrations.