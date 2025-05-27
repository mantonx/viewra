# MusicBrainz Metadata Enricher Plugin

A self-contained, idiomatic Go plugin for Viewra that enriches music metadata using the MusicBrainz database and Cover Art Archive.

## Architecture

This plugin follows Go best practices and is organized into focused packages:

```
plugins/musicbrainz_enricher/
├── plugin.go              # Entry point: init, registration, plugin metadata
├── enricher.go            # Core enrichment logic and HTTP handlers
├── musicbrainz/
│   ├── client.go          # MusicBrainz + CoverArtArchive API client
│   ├── models.go          # External response structs (MBID, Artist, etc.)
│   └── mapper.go          # Converts external data into internal Viewra models
├── tagreader/
│   └── reader.go          # Reads basic metadata from MP3/FLAC files
├── config/
│   └── config.go          # Per-plugin config loading (API keys, caching, etc.)
├── internal/
│   └── utils.go           # Helpers (string cleanup, duration math, etc.)
├── testdata/
│   └── sample.mp3         # Sample audio files for unit tests
├── plugin_test.go         # Tests for plugin integration
├── go.mod                 # Go module definition
├── plugin.yml             # Plugin manifest and configuration
└── README.md              # This file
```

## Features

- **Metadata Enrichment**: Searches MusicBrainz database for accurate music metadata
- **Artwork Download**: Integrates with Cover Art Archive for album artwork
- **Rate Limiting**: Respects MusicBrainz API guidelines (configurable 0.1-1.0 requests/second)
- **Intelligent Matching**: Configurable similarity threshold for automatic matching
- **Caching**: Configurable cache duration to reduce API calls
- **Scanner Integration**: Automatically enriches new files during media scans
- **Self-Contained**: Creates its own database tables and manages its own data
- **Idiomatic Go**: Follows Go best practices with proper package organization

## Plugin Architecture

This plugin follows Viewra's plugin principles and implements the generic enrichment interface:

- **Self-Contained**: All database models, API endpoints, and logic are contained within the plugin
- **Registry-Based**: Registers itself with the main Viewra enrichment registry at init time
- **Generic Interface**: Implements the `MetadataEnricher` interface for compatibility with other plugins
- **Priority-Based**: Works with other enrichers in a priority-based system
- **Database Independence**: Creates and manages its own database tables
- **YAML Configuration**: All configuration is managed through the plugin.yml file
- **Go Module**: Proper Go module with versioned dependencies

### Registration Pattern

The plugin registers itself at initialization:

```go
func Init() error {
    // Load configuration
    cfg := config.Default()

    // Create enricher instance
    enricher := NewMusicBrainzEnricher(nil, cfg)

    // Register with global registry
    registry := GetGlobalRegistry()
    return registry.RegisterEnricher(MediaTypeMusic, enricher)
}
```

### Generic Interface Implementation

The plugin implements the `MetadataEnricher` interface:

```go
type MetadataEnricher interface {
    GetID() string
    GetName() string
    GetSupportedMediaTypes() []string
    CanEnrich(track *Track) bool
    Enrich(ctx context.Context, request *EnrichmentRequest) (*EnrichmentResult, error)
    GetPriority() int
    IsEnabled() bool
    GetConfiguration() map[string]interface{}
}
```

## Database Schema

The plugin creates three tables with the `musicbrainz_enricher_` prefix:

### `musicbrainz_enricher_caches`

Caches MusicBrainz API responses to reduce API calls and respect rate limits.

### `musicbrainz_enricher_enrichments`

Stores enrichment data for media files with MusicBrainz IDs and enhanced metadata.

### `musicbrainz_enricher_stats`

Tracks plugin usage statistics and performance metrics.

## Configuration

All configuration is done through the `plugin.yml` file:

```yaml
config:
  enabled: true # Enable/disable the plugin
  api_rate_limit: 0.8 # API requests per second (max 1.0)
  user_agent: 'Viewra/1.0.0' # User agent for API requests
  enable_artwork: true # Download album artwork
  artwork_max_size: 1200 # Maximum artwork size in pixels
  artwork_quality: 'front' # Preferred artwork type
  match_threshold: 0.85 # Minimum similarity score (0.5-1.0)
  auto_enrich: false # Auto-enrich during scans
  overwrite_existing: false # Overwrite existing enrichment data
  cache_duration_hours: 168 # Cache duration (1 week)
```

### Configuration Validation

The plugin validates all configuration values:

- `api_rate_limit`: Must be between 0.1 and 1.0
- `match_threshold`: Must be between 0.5 and 1.0
- `artwork_max_size`: Must be between 250 and 2000 pixels
- `cache_duration_hours`: Must be between 1 and 8760 hours
- `artwork_quality`: Must be a valid Cover Art Archive type

## API Endpoints

The plugin provides REST API endpoints under `/api/plugins/musicbrainz`:

### Status and Statistics

- `GET /status` - Plugin status and configuration
- `GET /stats` - Usage statistics and performance metrics

### Enrichment Operations

- `POST /enrich/:mediaFileId` - Enrich a specific media file
- `POST /enrich-batch` - Batch enrichment of multiple files

### Search and Lookup

- `GET /search?q=query` - Search MusicBrainz database
- `GET /search?title=...&artist=...&album=...` - Structured search

### Cache Management

- `DELETE /cache` - Clear API response cache
- `GET /cache/stats` - Cache statistics and health

### Enrichment Data Management

- `GET /enrichments` - List all enrichments (paginated)
- `GET /enrichments/:mediaFileId` - Get enrichment for specific file
- `DELETE /enrichments/:mediaFileId` - Delete enrichment data

## Scanner Integration

The plugin implements the `ScannerHookPlugin` interface:

- **OnMediaFileScanned**: Called when a media file is processed during scanning
- **OnScanStarted**: Called when a scan job begins
- **OnScanCompleted**: Called when a scan job finishes

When `auto_enrich` is enabled, the plugin automatically attempts to enrich metadata for newly scanned audio files in the background.

## Development

### Building and Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./config
go test ./musicbrainz
go test ./internal
```

### Code Organization

- **plugin.go**: Main entry point and plugin interface implementation
- **enricher.go**: Core business logic and HTTP handlers
- **config/**: Configuration loading, validation, and defaults
- **musicbrainz/**: External API client and data mapping
- **internal/**: Internal utilities and helpers
- **tagreader/**: Audio file metadata reading (placeholder)

### Adding New Features

1. Add configuration options to `config/config.go`
2. Implement business logic in `enricher.go`
3. Add API endpoints as needed
4. Update database models if required
5. Add comprehensive tests

## Installation

1. Place the plugin files in `backend/data/plugins/musicbrainz_enricher/`
2. The plugin will be automatically discovered by the plugin manager
3. Enable the plugin through the admin interface or database
4. Configure settings in `plugin.yml` as needed

## Usage Examples

### Manual Enrichment via API

```bash
# Enrich a specific media file
curl -X POST /api/plugins/musicbrainz/enrich/12345 \
  -H "Content-Type: application/json" \
  -d '{"metadata": {"title": "Bohemian Rhapsody", "artist": "Queen", "album": "A Night at the Opera"}}'

# Search MusicBrainz database
curl "/api/plugins/musicbrainz/search?title=Bohemian%20Rhapsody&artist=Queen"

# Get plugin statistics
curl "/api/plugins/musicbrainz/stats"
```

### Programmatic Usage

```go
// Initialize the plugin (called at startup)
if err := Init(); err != nil {
    log.Fatal(err)
}

// Get the global registry
registry := GetGlobalRegistry()

// Create a track to enrich
track := &Track{
    ID:       12345,
    FilePath: "/music/queen/bohemian_rhapsody.mp3",
    Title:    "Bohemian Rhapsody",
    Artist:   "Queen",
    Album:    "A Night at the Opera",
}

// Enrich the track
ctx := context.Background()
result, err := registry.EnrichTrack(ctx, track, nil)
if err != nil {
    log.Printf("Enrichment failed: %v", err)
} else if result.Success {
    log.Printf("Successfully enriched with score %.2f", result.MatchScore)
}
```

### Multiple Enrichers

The system supports multiple enrichers working together:

```go
// Register multiple enrichers (in order of priority)
registry.RegisterEnricher(MediaTypeMusic, musicBrainzEnricher)  // priority 80
registry.RegisterEnricher(MediaTypeMusic, lastFMEnricher)       // priority 60
registry.RegisterEnricher(MediaTypeMusic, discogsEnricher)      // priority 70

// The registry will try enrichers in priority order
result, err := registry.EnrichTrack(ctx, track, nil)
```

## Performance Considerations

- **Rate Limiting**: Respects MusicBrainz API rate limits
- **Caching**: Aggressive caching reduces API calls
- **Background Processing**: Auto-enrichment runs in background
- **Database Indexing**: Proper indexes on foreign keys and search fields
- **Memory Efficiency**: Streaming JSON parsing for large responses

## Error Handling

The plugin implements comprehensive error handling:

- API rate limit exceeded
- Network connectivity issues
- Invalid metadata format
- Database connection problems
- Configuration validation errors

## Future Enhancements

- React/TypeScript admin interface
- Advanced fuzzy matching algorithms
- Support for additional metadata sources (Last.fm, Discogs)
- Bulk enrichment operations with progress tracking
- Integration with other music services
- Machine learning-based matching improvements

## License

MIT License - see the main Viewra project for details.
