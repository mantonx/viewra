# Media Module

The media module provides comprehensive media library management for Viewra. It handles media file discovery, metadata extraction, library organization, and provides APIs for accessing media content across different media types including movies, TV shows, and music.

## Overview

The media module is responsible for:
- Managing media libraries (movie, TV, music collections)
- Scanning and cataloging media files from filesystem paths
- Extracting and managing media metadata
- Providing search and filtering capabilities with playback compatibility
- Supporting multiple media types with a unified interface
- Determining playback methods (direct, remux, transcode) based on file compatibility

## Architecture

### Clean Architecture Design

```
mediamodule/
├── api/                    # HTTP handlers (presentation layer)
│   ├── handlers.go        # Base handler struct
│   ├── media_handlers.go  # Media file endpoints
│   ├── tv_handlers.go     # TV show endpoints
│   ├── movie_handlers.go  # Movie endpoints
│   ├── music_handlers.go  # Music endpoints
│   ├── library_handlers.go # Library management
│   └── routes.go          # Route registration
├── core/                   # Business logic (domain layer)
│   ├── media_manager.go   # Orchestrates all operations
│   ├── filters/           # Filtering and query logic
│   ├── library/           # Library management
│   ├── metadata/          # Metadata operations
│   └── repository/        # Data access layer
├── service/               # Service interface implementation
│   └── media_service.go   # Thin wrapper implementing MediaService
├── types/                 # All type definitions
│   ├── media.go          # Media file and stream types
│   ├── library.go        # Library management types
│   ├── filters.go        # Query and filter types
│   ├── metadata.go       # Metadata types
│   ├── events.go         # Event types
│   └── interfaces.go     # Internal interfaces
├── utils/                 # Utility functions
│   └── probe.go          # FFmpeg media analysis
└── tests/                 # Integration tests
```

### Service Architecture

```
External Consumers (Frontend, Other Modules)
    ↓
Service Registry (services.MediaService interface)
    ↓
MediaService Implementation (thin wrapper)
    ↓
MediaManager (orchestrates business logic)
    ↓
Core Components:
├── Repository (data access)
├── Filter (query logic)
├── LibraryManager (library ops)
└── MetadataManager (metadata ops)
    ↓
Database (shared models)
```

## Core Concepts

### Playback Methods

The module determines playback compatibility for media files:

- **Direct Play**: File can be played directly in browsers (H264/AAC in MP4)
- **Remux**: Only container needs changing, codecs are compatible
- **Transcode**: Full transcoding required for incompatible formats

### Media Libraries

Media libraries represent collections organized by type:
- **Movie Libraries**: Collections of movie files
- **TV Libraries**: Hierarchical TV show/season/episode structure
- **Music Libraries**: Artists, albums, and tracks

### Type System

All types are organized in the `types/` directory:
- **Media Types**: File info, streams, analysis results
- **Library Types**: Configuration, statistics, scan status
- **Filter Types**: Query parameters, playback methods
- **Metadata Types**: Enrichment data for movies, episodes, tracks

## API Endpoints

### Media Files

#### List Media Files with Filtering
```http
GET /api/media/files
```

Query Parameters:
- `library_id`: Filter by library
- `media_type`: Filter by type (movie, episode, track)
- `playback_method`: Filter by compatibility (direct, remux, transcode)
- `video_codec`, `audio_codec`, `container`: Format filters
- `resolution`: Resolution filter (720p, 1080p, 4k)
- `search`: Search in file paths
- `limit`, `offset`: Pagination
- `sort_by`, `sort_order`: Sorting

Response includes playback compatibility information.

#### Get Media File Details
```http
GET /api/media/files/{id}
GET /api/media/files/{id}/metadata
```

### TV Shows

```http
GET /api/tv/shows
GET /api/tv/shows/{id}
GET /api/tv/shows/{id}/seasons
GET /api/tv/shows/{id}/seasons/{seasonId}/episodes
GET /api/tv/episodes/{episodeId}
```

### Movies

```http
GET /api/movies
GET /api/movies/{id}
GET /api/movies/search
GET /api/movies/{id}/similar
```

### Music

```http
GET /api/music/artists
GET /api/music/artists/{id}
GET /api/music/artists/{id}/albums
GET /api/music/albums
GET /api/music/albums/{id}
GET /api/music/playlists
```

### Library Management

```http
GET    /api/libraries
POST   /api/libraries
GET    /api/libraries/{id}
PUT    /api/libraries/{id}
DELETE /api/libraries/{id}
POST   /api/libraries/{id}/scan
GET    /api/libraries/{id}/scan/status
POST   /api/libraries/{id}/metadata/refresh
GET    /api/libraries/{id}/stats
```

## Implementation Details

### Clean Architecture Benefits

1. **Separation of Concerns**:
   - API handlers only handle HTTP concerns
   - Business logic isolated in core package
   - Service is a thin adapter layer

2. **Testability**:
   - Core logic can be tested without HTTP
   - Repository can be mocked for unit tests
   - Clear interfaces for all components

3. **Maintainability**:
   - Types organized by domain
   - Single responsibility for each component
   - Easy to locate and modify code

### Key Components

#### MediaManager (core/media_manager.go)
Orchestrates all media operations:
- Delegates to specialized components
- Coordinates between repository, filter, and managers
- Provides high-level business operations

#### Repository (core/repository/media_repository.go)
Handles all database operations:
- CRUD operations for media files
- Returns query builders for complex queries
- Isolates GORM dependencies

#### Filter (core/filters/media_filter.go)
Implements filtering and compatibility logic:
- Applies query filters to database queries
- Determines playback methods
- Handles resolution and codec filtering

#### MediaService (service/media_service.go)
Thin wrapper implementing the service interface:
- No business logic
- Simply delegates to MediaManager
- Implements the registry interface

## Usage Examples

### Checking Playback Compatibility

```go
// Get files that can be played directly
filter := types.MediaFilter{
    PlaybackMethod: "direct",
    MediaType: "movie",
}

files, err := mediaService.ListFiles(ctx, filter)
```

### Advanced Filtering

```go
// Find 4K HEVC files that need transcoding
filter := types.MediaFilter{
    Resolution: "4k",
    VideoCodec: "hevc",
    PlaybackMethod: "transcode",
    Limit: 20,
}

files, err := mediaService.ListFiles(ctx, filter)
```

## Performance Considerations

### Query Optimization
- Efficient indexes on commonly filtered columns
- Pagination for large result sets
- Query builder pattern for complex filters

### Playback Method Filtering
- Database-level filtering for performance
- No post-query filtering needed
- Efficient codec compatibility checks

## Testing

### Unit Tests
```bash
go test ./core/...        # Test business logic
go test ./core/filters/   # Test filter logic
go test ./core/repository # Test data access
```

### Integration Tests
```bash
go test ./tests/          # Full integration tests
```

## Configuration

The module uses environment variables for configuration:

```bash
# Scanning
MEDIA_SCAN_INTERVAL=24h
MEDIA_SCAN_CONCURRENT=4

# File Processing  
MAX_FILE_SIZE=10GB
SUPPORTED_VIDEO_FORMATS=mp4,mkv,avi,mov,webm
SUPPORTED_AUDIO_FORMATS=mp3,flac,wav,aac,ogg
```

## Future Enhancements

- External metadata providers integration
- Advanced duplicate detection
- Real-time file system monitoring
- Batch metadata operations
- Full-text search capabilities