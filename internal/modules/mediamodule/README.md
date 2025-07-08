# Media Module

The media module provides comprehensive media library management for Viewra. It handles media file discovery, metadata extraction, library organization, and provides APIs for accessing media content across different media types including movies, TV shows, and music.

## Overview

The media module is responsible for:
- Managing media libraries (movie, TV, music collections)
- Scanning and cataloging media files from filesystem paths
- Extracting and managing media metadata
- Providing search and filtering capabilities
- Supporting multiple media types with unified interface
- Integration with other modules for transcoding and playback

## Architecture

### Core Components

```
MediaModule
├── MediaService           # Public service interface
├── API Handlers          # HTTP endpoints for media operations  
├── Core Components:
│   ├── Library/          # Media library management
│   └── Metadata/         # Metadata extraction and management
├── Service/              # Service implementation
├── Utils/                # Helper utilities (probe, etc.)
└── Tests/                # Integration tests
```

### Service Architecture

```
Frontend/Other Modules
    ↓ (requests media data)
MediaService (interface)
    ↓ (implemented by)
MediaModule
    ↓ (manages)
[Library Manager, Metadata Manager]
    ↓ (operates on)
Database Models (MediaFile, MediaLibrary, etc.)
```

## Core Concepts

### Media Libraries

Media libraries represent collections of media organized by type:

- **Movie Libraries**: Collections of movie files
- **TV Libraries**: Collections of TV shows and episodes  
- **Music Libraries**: Collections of music tracks and albums

Each library has:
- **Path**: Filesystem directory to scan
- **Type**: Media type (movie, tv, music)
- **Metadata**: Library-specific configuration

### Media Files

All media content is represented as MediaFile entities with:
- **Basic Info**: Path, filename, size, type
- **Media Metadata**: Duration, resolution, codecs, etc.
- **Content Metadata**: Title, description, release date, etc.
- **Relationships**: Library associations, episode relationships

### Metadata Management

The module extracts and manages metadata from multiple sources:
- **File Analysis**: Technical metadata via FFmpeg probe
- **Filename Parsing**: Basic content info from filenames
- **External APIs**: Enhanced metadata from external services (future)

## API Endpoints

### Media Libraries

#### List Libraries
```http
GET /api/media/libraries
```

Response:
```json
{
  "libraries": [
    {
      "id": 1,
      "path": "/media/movies",
      "type": "movie",
      "created_at": "2023-01-01T12:00:00Z"
    }
  ]
}
```

#### Create Library
```http
POST /api/media/libraries
Content-Type: application/json

{
  "path": "/media/movies",
  "type": "movie"
}
```

#### Delete Library
```http
DELETE /api/media/libraries/{id}
```

### Media Files

#### List Media Files
```http
GET /api/media/files?library_id=1&media_type=movie&search=query&limit=50&offset=0
```

Query Parameters:
- `library_id`: Filter by library ID
- `media_type`: Filter by media type (movie, episode, track)
- `search`: Search in titles and descriptions
- `limit`: Maximum results to return (default: 50)
- `offset`: Number of results to skip (default: 0)
- `sort_by`: Sort field (title, created_at, file_size)
- `sort_order`: Sort direction (asc, desc)

Response:
```json
{
  "files": [
    {
      "id": "uuid",
      "library_id": 1,
      "media_type": "movie",
      "file_path": "/media/movies/example.mp4",
      "title": "Example Movie",
      "description": "Movie description",
      "release_date": "2023-01-01",
      "duration": 7200.0,
      "file_size": 1073741824,
      "video_codec": "h264",
      "audio_codec": "aac",
      "resolution": "1920x1080",
      "created_at": "2023-01-01T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

#### Get Media File
```http
GET /api/media/files/{id}
```

#### Update Media File  
```http
PUT /api/media/files/{id}
Content-Type: application/json

{
  "title": "Updated Title",
  "description": "Updated description"
}
```

#### Delete Media File
```http
DELETE /api/media/files/{id}
```

### Library Management

#### Scan Library
```http
POST /api/media/libraries/{id}/scan
```

Initiates a scan of the library directory to discover new media files.

#### Get Library Statistics
```http
GET /api/media/libraries/{id}/stats
```

Response:
```json
{
  "total_files": 1250,
  "total_size": 1099511627776,
  "by_type": {
    "movie": 800,
    "episode": 400,
    "track": 50
  },
  "by_codec": {
    "h264": 1000,
    "h265": 200,
    "av1": 50
  }
}
```

## Configuration

### Environment Variables

```bash
# Media storage paths
MEDIA_ROOT_DIR=/media
MEDIA_CACHE_DIR=/cache/media

# Scanning settings
MEDIA_SCAN_ENABLED=true
MEDIA_SCAN_INTERVAL=24h
MEDIA_SCAN_DEPTH=10

# Metadata settings
METADATA_EXTRACTION_ENABLED=true
METADATA_CACHE_TTL=168h

# File processing
MAX_FILE_SIZE=10GB
SUPPORTED_VIDEO_FORMATS=mp4,mkv,avi,mov,webm
SUPPORTED_AUDIO_FORMATS=mp3,flac,wav,aac,ogg
SUPPORTED_IMAGE_FORMATS=jpg,jpeg,png,webp
```

### Database Configuration

The module automatically creates and manages these database tables:
- `media_libraries`: Library definitions and configuration
- `media_files`: Individual media file records
- `movies`: Movie-specific metadata
- `tv_shows`: TV show information
- `episodes`: TV episode metadata  
- `albums`: Music album information
- `tracks`: Music track metadata

## Usage Examples

### Basic Library Management

```go
// Get media service
mediaService := services.GetService[services.MediaService]("media")

// Create a new movie library
library := &database.MediaLibraryRequest{
    Path: "/media/movies",
    Type: "movie",
}

libraryID, err := mediaService.CreateLibrary(ctx, library)
if err != nil {
    return fmt.Errorf("failed to create library: %w", err)
}

log.Printf("Created library: %d", libraryID)
```

### Media File Operations

```go
// Get media files with filtering
filter := &types.MediaFilter{
    LibraryID: &libraryID,
    MediaType: "movie",
    Search:    "action",
    Limit:     20,
    Offset:    0,
}

files, total, err := mediaService.GetFiles(ctx, filter)
if err != nil {
    return fmt.Errorf("failed to get files: %w", err)
}

log.Printf("Found %d files out of %d total", len(files), total)
```

### Metadata Extraction

```go
// Get detailed media information
fileInfo, err := mediaService.GetFile(ctx, fileID)
if err != nil {
    return fmt.Errorf("failed to get file: %w", err)
}

log.Printf("File: %s, Duration: %.2fs, Resolution: %s", 
    fileInfo.Title, 
    fileInfo.Duration, 
    fileInfo.Resolution)
```

## Implementation Details

### Library Manager

Core library management (`core/library/manager.go`):

- **Library Creation**: Validates paths and creates database records
- **Path Validation**: Ensures library paths exist and are accessible
- **Library Operations**: CRUD operations for media libraries
- **Configuration Management**: Library-specific settings and preferences

### Metadata Manager  

Metadata extraction and management (`core/metadata/manager.go`):

- **File Analysis**: Uses FFmpeg probe for technical metadata
- **Content Parsing**: Extracts content info from filenames and directories
- **Metadata Storage**: Saves extracted metadata to database
- **Enrichment**: Future integration with external metadata sources

### Media Service

Service implementation (`service/media_service.go`):

- **Public Interface**: Implements the MediaService interface
- **Business Logic**: Coordinates between managers and database
- **Data Access**: Provides filtered and paginated data access
- **Error Handling**: Consistent error handling and logging

### Media Probe Utility

File analysis utility (`utils/probe.go`):

- **FFmpeg Integration**: Executes FFmpeg probe commands
- **Metadata Parsing**: Parses FFmpeg output into structured data
- **Error Recovery**: Handles probe failures gracefully
- **Performance**: Efficient probing for large media collections

## Database Models

### MediaLibrary

```go
type MediaLibrary struct {
    ID        uint32    `gorm:"primaryKey"`
    Path      string    `gorm:"not null"`
    Type      string    `gorm:"not null"` // "movie", "tv", "music"
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### MediaFile

```go
type MediaFile struct {
    ID           string    `gorm:"primaryKey;type:varchar(128)"`
    LibraryID    uint32    `gorm:"not null;index"`
    MediaType    MediaType `gorm:"type:varchar(32);not null;index"`
    FilePath     string    `gorm:"not null;uniqueIndex"`
    FileName     string    `gorm:"not null"`
    FileSize     int64     `gorm:"not null"`
    
    // Content metadata
    Title        string
    Description  string  
    ReleaseDate  *time.Time
    
    // Technical metadata
    Duration     float64
    VideoCodec   string
    AudioCodec   string
    Resolution   string
    Bitrate      int64
    
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### Content-Specific Models

Media-specific models for enhanced metadata:
- **Movie**: Genre, director, cast, ratings
- **TVShow**: Series info, seasons count
- **Episode**: Season/episode numbers, series relationship
- **Album**: Artist, genre, year, track count
- **Track**: Album relationship, track number, artist

## Error Handling

The module provides comprehensive error handling:

### Common Error Types

- **Path Errors**: Invalid library paths, permission issues
- **File Errors**: Corrupted files, unsupported formats
- **Database Errors**: Connection issues, constraint violations  
- **Metadata Errors**: FFmpeg failures, parsing errors

### Error Response Format

```json
{
  "error": "library_not_found",
  "message": "Library with ID 123 not found",
  "details": {
    "library_id": 123,
    "operation": "scan"
  }
}
```

## Performance Considerations

### Scanning Optimization

- **Incremental Scanning**: Only process changed files
- **Concurrent Processing**: Parallel file analysis
- **Caching**: Metadata caching to avoid re-analysis
- **Batch Operations**: Efficient database insertions

### Database Optimization

- **Indexing**: Optimized indexes for common queries
- **Pagination**: Efficient large dataset handling
- **Query Optimization**: Minimal N+1 query issues
- **Connection Pooling**: Efficient database connections

### Memory Management

- **Streaming**: Large file processing without loading into memory
- **Cleanup**: Automatic cleanup of temporary resources
- **Limits**: Configurable file size and processing limits

## Testing

### Unit Tests

```bash
# Run all media module tests
cd internal/modules/mediamodule
go test ./...

# Run with coverage
go test -cover ./...

# Run specific components
go test ./core/library/
go test ./core/metadata/
go test ./service/
```

### Integration Tests

```bash
# Run integration tests
go test ./tests/

# Test with real media files
go test -run TestMediaScanning ./tests/
go test -run TestMetadataExtraction ./tests/
```

### Manual Testing

```bash
# Start development environment
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Create a test library
curl -X POST http://localhost:8080/api/media/libraries \
  -H "Content-Type: application/json" \
  -d '{"path": "/app/test-media", "type": "movie"}'

# List media files  
curl http://localhost:8080/api/media/files?library_id=1
```

## Troubleshooting

### Common Issues

1. **Permission Errors**
   - Verify read permissions on media directories
   - Check Docker volume mounts in development

2. **FFmpeg Probe Failures**
   - Ensure FFmpeg is installed and accessible
   - Check for corrupted or unsupported media files

3. **Database Constraint Errors**
   - Verify unique constraints on file paths
   - Check foreign key relationships

4. **Large Library Performance**
   - Enable incremental scanning
   - Adjust database connection limits
   - Consider pagination for large queries

### Debug Mode

Enable debug logging:

```bash
export LOG_LEVEL=debug
export MEDIA_DEBUG=true
```

### Monitoring

Key metrics to monitor:
- Library scan duration and frequency
- Media file count and growth rate
- Database query performance
- FFmpeg probe success/failure rates
- API response times

## Future Enhancements

Planned improvements:
- **External Metadata**: Integration with TMDB, TVDB, MusicBrainz
- **Advanced Search**: Full-text search with Elasticsearch
- **Duplicate Detection**: Intelligent duplicate file detection
- **Batch Operations**: Bulk metadata editing and organization
- **Watch Folders**: Real-time filesystem monitoring
- **Metadata Scraping**: Automatic poster/artwork downloading
- **Library Analytics**: Usage statistics and recommendations
- **Export/Import**: Library backup and migration tools