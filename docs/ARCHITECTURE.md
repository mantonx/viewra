# Viewra Architecture

## Overview

Viewra is a modular media management platform built with clean architecture principles. It provides media library management, intelligent playback decisions, and extensible transcoding capabilities.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend (React)                       │
│                   Responsive Web Interface                    │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway (Gin)                        │
│                    RESTful API Endpoints                      │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────┬─────────────────┬─────────────────────────┐
│  Media Module   │ Playback Module │ Transcoding Module      │
├─────────────────┼─────────────────┼─────────────────────────┤
│ • Libraries     │ • Decisions     │ • Session Management    │
│ • Scanning      │ • Sessions      │ • Provider Selection    │
│ • Metadata      │ • Streaming     │ • Content Storage       │
└─────────────────┴─────────────────┴─────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                    Service Registry                           │
│              Inter-module Communication                       │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────┬─────────────────┬─────────────────────────┐
│    Database     │  File System    │    Plugin System        │
│   (SQLite/PG)   │  Media Storage  │   External Plugins      │
└─────────────────┴─────────────────┴─────────────────────────┘
```

## Core Modules

### 1. Media Module
Manages media libraries and metadata.

**Responsibilities:**
- Library management (create, scan, update)
- Media file tracking and metadata
- Search and filtering capabilities
- Integration with scanner services

**Key Components:**
- `LibraryManager`: Handles library operations
- `MetadataManager`: Manages media metadata
- `MediaService`: Service layer interface

### 2. Playback Module
Makes intelligent playback decisions based on media format and device capabilities.

**Responsibilities:**
- Analyze media file compatibility
- Determine optimal playback method
- Manage playback sessions
- Track viewing history

**Key Components:**
- `DecisionEngine`: Analyzes and decides playback strategy
- `SessionManager`: Tracks active playback sessions
- `StreamingManager`: Handles streaming operations

### 3. Transcoding Module
Manages media transcoding through various providers.

**Responsibilities:**
- Provider management and selection
- Transcoding session lifecycle
- Content-addressable storage
- Progress tracking

**Key Components:**
- `TranscodingManager`: Orchestrates transcoding operations
- `ProviderRegistry`: Manages available providers
- `ContentStore`: Handles transcoded content storage

## Clean Architecture Pattern

Each module follows a consistent structure:

```
module/
├── api/           # HTTP handlers
├── core/          # Business logic
├── service/       # Service interface
├── repository/    # Data access
├── models/        # Database models
├── types/         # Shared types
├── errors/        # Error handling
└── module.go      # Module registration
```

**Principles:**
- Dependencies flow inward (API → Service → Core)
- Business logic isolated in core layer
- Clear interfaces between layers
- Module independence

## Plugin Architecture

Viewra uses HashiCorp's go-plugin for extensibility:

```
┌─────────────────┐     gRPC      ┌─────────────────┐
│   Viewra Core   │◄─────────────►│  Plugin Process │
│                 │               │                 │
│ Plugin Manager  │               │ Implementation  │
└─────────────────┘               └─────────────────┘
```

**Plugin Types:**
- **Transcoding Providers**: FFmpeg variants, hardware acceleration
- **Metadata Scrapers**: Extract metadata from files
- **Enrichment Services**: External metadata sources (TMDB, MusicBrainz)

## Data Flow

### Media Playback Flow

1. **Client Request**: User selects media to play
2. **Playback Decision**: 
   - Media module provides file information
   - Playback module analyzes compatibility
   - Decision: Direct play or transcode
3. **Direct Play**: Stream original file
4. **Transcode Path**:
   - Select optimal provider
   - Start transcoding session
   - Stream transcoded content

### Transcoding Flow

1. **Request**: Playback module requests transcoding
2. **Provider Selection**: Based on format support and availability
3. **Processing**: Provider transcodes media
4. **Storage**: Content stored with hash-based addressing
5. **Delivery**: Stream URL provided to client

## Storage Architecture

### Media Storage
- Original media files remain in configured library paths
- Database tracks file locations and metadata
- No modification of source files

### Transcoded Content
- Content-addressable storage using SHA256 hashes
- Automatic deduplication
- Directory sharding for scalability
- Structure: `/content/{hash[0:2]}/{hash[2:4]}/{full_hash}/`

## Communication Patterns

### Service Registry
Modules communicate through registered services:

```go
// Registration
services.Register("media", mediaService)

// Usage
mediaService, _ := services.Get("media")
```

### Event System
Asynchronous communication via event bus:
- Library scan completion
- Transcoding progress
- Session state changes

## Configuration

### Module Configuration
Each module can be configured independently:
- Database connections
- Storage paths
- Feature flags

### Plugin Configuration
Plugins use CueLang for type-safe configuration:
```cue
enabled: true
priority: 50
capabilities: {
    formats: ["mp4", "mkv"]
    codecs: ["h264", "hevc"]
}
```

## Security Considerations

- Input validation at API layer
- Path traversal prevention
- Session authentication (planned)
- Resource limits on transcoding

## Performance Optimizations

- Database query optimization with indexes
- Concurrent media scanning
- Provider load balancing
- Content caching strategies

## Monitoring & Observability

- Structured logging with context
- Health check endpoints
- Metrics collection (planned)
- Error tracking and reporting

## Future Enhancements

1. **Authentication & Authorization**: User management and access control
2. **Distributed Transcoding**: Multi-node transcoding support
3. **Cloud Storage**: S3 and cloud provider integration
4. **Advanced Analytics**: Viewing patterns and recommendations
5. **Mobile Apps**: Native mobile applications