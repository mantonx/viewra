# TMDb Enricher Plugin v2

A modern, service-oriented TMDb metadata enrichment plugin for Viewra media management system.

## Overview

This is a complete refactor of the original TMDb enricher plugin, designed with modern Go patterns and clean architecture principles.

## Architecture

### Service-Oriented Design

The plugin is built around dedicated services with single responsibilities:

- **EnrichmentService** (`internal/services/enrichment.go`) - Core business logic for processing media files and enriching metadata
- **MatchingService** (`internal/services/matching.go`) - Sophisticated content matching algorithms with Levenshtein distance calculations
- **CacheManager** (`internal/services/cache.go`) - Comprehensive caching for all TMDb API responses with automatic cleanup
- **ArtworkService** (`internal/services/artwork.go`) - Handles artwork downloading and management with quality scoring
- **APIClient** (`internal/services/api_client.go`) - Shared API client for TMDb interactions with rate limiting

### Configuration System

**Configuration** (`internal/config/config.go`) - Type-safe configuration with sections:
- API settings (rate limiting, timeouts, language/region)
- Feature toggles (movies, TV shows, episodes, artwork)
- Artwork preferences (download types, sizes, limits)
- Matching algorithms (thresholds, year tolerance)
- Cache settings (duration, cleanup intervals)
- Reliability settings (retries, backoff strategies)
- Debug options

### Data Models

**Models** (`internal/models/models.go`) - Clean database schema:
- `TMDbCache` - API response caching with expiration
- `TMDbEnrichment` - Enrichment metadata storage
- `TMDbArtwork` - Artwork metadata tracking

## Key Features

### Intelligent Content Matching
- Levenshtein distance calculations for title similarity
- Content type detection (movie vs TV show vs episode)
- Match scoring with configurable thresholds
- Year-based matching with tolerance ranges
- Title extraction and normalization

### Comprehensive Artwork Management
- Quality-based artwork selection using TMDb vote data
- Support for posters, backdrops, logos, stills, season posters
- Configurable image sizes and download limits
- Automatic artwork downloading during enrichment
- Integration with Viewra's unified asset management system

### Advanced Caching
- Intelligent caching of all API responses
- Configurable cache duration and cleanup intervals
- Cache statistics and management
- Automatic cache warming and optimization

### Rate Limiting & Reliability
- Sophisticated rate limiting to respect TMDb API limits
- Exponential backoff retry mechanisms
- Comprehensive error handling and recovery
- Request timeout and size limit management

## File Structure

```
tmdb_enricher_v2/
├── main.go                           # Plugin entry point (359 lines)
├── internal/
│   ├── config/
│   │   └── config.go                 # Configuration system (222 lines)
│   ├── models/
│   │   └── models.go                 # Database models (177 lines)
│   └── services/
│       ├── api_client.go             # Shared API client (105 lines)
│       ├── artwork.go                # Artwork management (694 lines)
│       ├── cache.go                  # Cache management (268 lines)
│       ├── enrichment.go             # Core enrichment logic (814 lines)
│       └── matching.go               # Content matching (348 lines)
└── README.md                         # This file
```

## Benefits Over V1

### Maintainability
- **3,858 lines → ~2,600 lines**: More focused, better organized code
- **Single file → 8 files**: Clear separation of concerns
- **Modular design**: Easy to test, extend, and modify individual components

### Performance
- **Intelligent caching**: Reduces redundant API calls
- **Background artwork downloads**: Non-blocking enrichment
- **Optimized rate limiting**: Maximizes API efficiency

### Reliability
- **Better error handling**: Graceful degradation and recovery
- **Retry mechanisms**: Handles temporary failures automatically
- **Configuration validation**: Prevents runtime configuration errors

### Extensibility
- **Service interfaces**: Easy to add new functionality
- **Clean dependencies**: Services can be swapped or extended
- **Plugin patterns**: Follows modern plugin architecture

## Configuration

The plugin uses CUE for configuration validation. Key settings include:

```yaml
api:
  key: "your-tmdb-api-key"
  rate_limit: 0.6  # requests per second
  language: "en-US"

features:
  enable_movies: true
  enable_tv_shows: true
  enable_artwork: true
  auto_enrich: true

artwork:
  download_posters: true
  download_backdrops: false
  poster_size: "w500"
  max_asset_size_mb: 10

matching:
  match_threshold: 0.85
  year_tolerance: 2

cache:
  duration_hours: 168  # 1 week
  cleanup_interval: 24 # daily
```

## Integration

The plugin integrates seamlessly with Viewra's core systems:

- **Scanner hooks**: Automatic enrichment during media scanning
- **Asset management**: Artwork saved through unified asset service
- **Database**: Clean schema with proper migrations
- **API**: RESTful endpoints for cache management and health checks

## Future Enhancements

- Unit test coverage for all services
- Advanced search capabilities
- Real-time enrichment dashboard
- Machine learning-based match scoring
- Multi-language artwork support
- Custom metadata field mapping 