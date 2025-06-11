# Viewra - Your Modern Media Management Platform

Viewra is a flexible and extensible media management platform designed to help you organize, browse, and interact with your media library efficiently. It features a robust backend built in Go and a modern frontend using Vue.js.

## Key Features

- **Extensible Plugin System**: Customize and extend Viewra's functionality with powerful plugins. (See [Plugin Documentation](docs/PLUGINS.md))
- **Efficient Media Scanning**: Fast and reliable scanning of your media libraries.
- **Metadata Enrichment**: Leverage plugins (like MusicBrainz) to enrich your media files with detailed metadata.
- **Modern Web Interface**: A clean and user-friendly interface built with Vue.js.
- **Dockerized Deployment**: Easy to deploy and manage using Docker.
- **CueLang Configuration**: Type-safe and powerful configuration for plugins using CueLang.

## Tech Stack

- **Backend**: Go (Golang)
- **Frontend**: Vue.js, TypeScript
- **Plugin System**: HashiCorp go-plugin, gRPC, CueLang
- **Database**: (Specify primary database, e.g., SQLite, PostgreSQL - GORM is used for ORM)
- **Containerization**: Docker, Docker Compose

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go (version X.Y.Z) - for backend development or running outside Docker
- Node.js and npm/yarn - for frontend development
- `jq` and `curl` (for some utility scripts)

### Installation & Setup

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/mantonx/viewra.git
    cd viewra
    ```

2.  **Development Environment (Docker Compose):**
    The easiest way to get started for development is using Docker Compose. This will set up the backend, frontend, and any necessary services.

    ```bash
    # Make sure development startup scripts are executable
    chmod +x dev-compose.sh
    chmod +x backend/scripts/scanner/test-scanner.sh # If you plan to use it

    # Start the development environment
    ./dev-compose.sh up
    ```

    This typically starts:

    - Viewra Backend (with hot-reloading via Air)
    - Viewra Frontend (Vite dev server with hot-reloading)
    - (Any other services defined in `docker-compose.yml`)

3.  **Manual Setup (Backend):**
    If you prefer to run the backend manually:

    ```bash
    cd backend
    # Install dependencies (if not already handled by your Go environment)
    go mod download
    # Run the backend (example)
    go run cmd/viewra/main.go
    ```

4.  **Manual Setup (Frontend):**
    If you prefer to run the frontend manually:
    ```bash
    cd frontend
    npm install # or yarn install
    npm run dev # or yarn dev
    ```

### Accessing Viewra

- **Frontend**: Typically `http://localhost:5173` (Vite default) or `http://localhost:3000` (check `docker-compose.yml` or your frontend setup).
- **Backend API**: Typically `http://localhost:8080/api` (check `docker-compose.yml` or your backend setup).

## Usage

Once Viewra is running, navigate to the frontend URL in your web browser. You can start by:

1.  Configuring your media libraries through the admin interface.
2.  Initiating a scan of your libraries.
3.  Browsing your media.

## Plugin System

Viewra features a powerful plugin system that allows for significant customization and extension of its core capabilities. For detailed information on how the plugin system works and how to develop your own plugins, please refer to the [Plugin Documentation](docs/PLUGINS.md).

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for more details on how to get involved, coding standards, and the development process.

## Testing

Viewra aims for comprehensive test coverage.

- **Backend Testing**: Go tests are used for unit and integration testing. You can run backend tests using the Makefile targets:
  ```bash
  cd backend
  make test          # Run all backend tests
  make test-coverage # Generate coverage reports
  ```
- **Scanner Testing**: A specific script is available for testing the media scanner functionality. See [Scanner Testing Documentation](backend/scripts/scanner/README.md).
- **Frontend Testing**: (Details about frontend testing tools and commands would go here - e.g., Vitest, Cypress).

For more general information on testing, please refer to `docs/TESTING.md` (if this file exists and is up-to-date).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

_This README is a starting point. Please update it with more specific details about your project setup, database choices, and frontend testing procedures._

# MusicBrainz Metadata Enricher Plugin

A comprehensive music metadata enrichment plugin for Viewra that uses the MusicBrainz database to enrich audio files with detailed metadata including artist information, album details, track data, and cover art.

## Overview

This plugin represents the modern, modular architecture template for Viewra plugins, featuring:

- **Clean separation of concerns** with dedicated packages for configuration, models, and services
- **Type-safe CUE configuration** with validation and sensible defaults
- **Comprehensive MusicBrainz integration** supporting artists, albums, tracks, and cover art
- **Built-in reliability patterns** including circuit breakers, exponential backoff, and retry logic
- **Efficient caching** to minimize API requests and improve performance
- **Full SDK integration** for seamless Viewra ecosystem compatibility

## Architecture

```
musicbrainz_enricher/
├── plugin.cue              # Type-safe configuration schema
├── go.mod                  # Go module with local SDK references
├── main.go                 # Plugin entry point and SDK integration
├── internal/               # Internal implementation packages
│   ├── config/            # Configuration management
│   │   └── config.go      # Type-safe config structures with validation
│   ├── models/            # Database models
│   │   └── models.go      # MusicBrainz-specific data models
│   └── services/          # Business logic services
│       └── services.go    # Core enrichment and API services
└── README.md              # This documentation
```

## Features

### Music Metadata Enrichment
- **Artist Information**: Full artist profiles, aliases, and relationships
- **Album Details**: Release information, label data, and catalog numbers
- **Track Metadata**: Detailed track information including ISRC codes
- **Work Information**: Composition details for classical music
- **Technical Metadata**: Duration, track numbers, disc information

### Cover Art Management
- **Multiple Sources**: Cover Art Archive, Amazon, Discogs, Last.fm
- **Size Options**: Support for various image sizes (250px, 500px, 1200px, original)
- **Smart Downloads**: Automatic cover art downloading with size preferences
- **Thumbnail Support**: Optional thumbnail generation for performance

### Advanced Matching
- **Multi-factor Matching**: Title, artist, album, duration, and unique identifiers
- **ISRC Matching**: International Standard Recording Code support
- **Barcode Matching**: Album barcode (EAN/UPC) matching
- **Fuzzy Matching**: Intelligent matching with configurable thresholds
- **Duration Tolerance**: Track length matching with configurable tolerance

### Reliability & Performance
- **Circuit Breaker Pattern**: Automatic failure handling and recovery
- **Exponential Backoff**: Smart retry logic with progressive delays
- **Request Rate Limiting**: Respectful API usage following MusicBrainz guidelines
- **Comprehensive Caching**: Reduce API calls and improve response times
- **Connection Pooling**: Efficient resource utilization

## Configuration

The plugin uses a comprehensive CUE-based configuration system with the following main sections:

### Core Settings
```cue
enabled: bool | *true  // Enable/disable the plugin
```

### API Configuration
```cue
api: {
    user_agent: string | *"Viewra/2.0 (https://github.com/mantonx/viewra)"
    request_timeout: int | *30        // seconds
    max_connections: int | *5         // concurrent connections
    request_delay: int | *1000        // milliseconds (MusicBrainz rate limiting)
    enable_cache: bool | *true
    cache_duration_hours: int | *168  // 1 week
}
```

### Feature Toggles
```cue
features: {
    enable_artists: bool | *true
    enable_albums: bool | *true  
    enable_tracks: bool | *true
    enable_cover_art: bool | *true
    enable_relationships: bool | *true
    enable_genres: bool | *true
    enable_tags: bool | *true
}
```

### Cover Art Settings
```cue
cover_art: {
    download_covers: bool | *true
    download_thumbnails: bool | *false
    preferred_size: string | *"500"    // 250, 500, 1200, original
    max_size_mb: int | *5              // Maximum file size
    skip_existing: bool | *true
    cover_sources: [                   // Preferred sources in order
        "Cover Art Archive",
        "Amazon",
        "Discogs", 
        "Last.fm"
    ]
}
```

### Matching Configuration
```cue
matching: {
    match_threshold: number | *0.80       // Minimum match confidence
    auto_enrich: bool | *true
    overwrite_existing: bool | *false
    fuzzy_matching: bool | *true
    match_by_isrc: bool | *true           // International Standard Recording Code
    match_by_barcode: bool | *true        // Album barcode matching
    match_duration: bool | *true          // Track duration matching
    duration_tolerance: int | *10         // seconds tolerance
}
```

### Reliability Settings
```cue
reliability: {
    max_retries: int | *3
    initial_retry_delay: int | *2         // seconds
    max_retry_delay: int | *30            // seconds  
    backoff_multiplier: number | *2.0    // exponential backoff
    timeout_multiplier: number | *1.5    // increase timeout on retries
    
    circuit_breaker: {
        failure_threshold: int | *5       // failures before opening circuit
        success_threshold: int | *3       // successes to close circuit  
        timeout: int | *60                // seconds before retry
    }
}
```

## Database Models

### MusicBrainzEnrichment
Comprehensive music metadata storage including:
- MusicBrainz IDs (Recording, Release, Artist, Work)
- Basic metadata (title, artist, album, year)
- Audio metadata (duration, ISRC, barcode)
- Release information (date, country, status, type)
- Label information (name, catalog number)
- Artist credits (performers, composers, producers)
- Classification (genres, tags, styles)
- Work information (classical compositions)
- Relationships and aliases
- Cover art references
- Match confidence scoring

### MusicBrainzArtwork  
Cover art management including:
- Cover Art Archive integration
- Multiple size support (thumbnails, small, large)
- Local storage paths
- Image metadata (type, dimensions, file size)
- Source tracking and approval status
- Download status and error handling

### MusicBrainzCache
API response caching for:
- Artist lookups
- Release searches  
- Recording queries
- Work information
- Configurable expiration times

## Plugin Interfaces

This plugin implements the following Viewra plugin interfaces:

- **MetadataScraperService**: Core metadata extraction and enrichment
- **ScannerHookService**: Integration with media scanning workflow
- **DatabaseService**: Database model management and migrations
- **APIRegistrationService**: HTTP API endpoint registration

## Installation

1. Ensure the plugin is in the correct directory: `backend/data/plugins/musicbrainz_enricher/`
2. The plugin will be automatically discovered and loaded by Viewra
3. Configuration can be customized by editing `plugin.cue`
4. Database migrations will be automatically applied

## API Endpoints

The plugin registers the following HTTP endpoints:

- `GET /api/plugins/musicbrainz_enricher/search` - Search MusicBrainz for content
- `GET /api/plugins/musicbrainz_enricher/config` - Get current plugin configuration

## Development

### Building
```bash
go build -o musicbrainz_enricher
```

### Testing
```bash
go test ./...
```

### Dependencies
- Go 1.21+
- MusicBrainz API access (no key required)
- Viewra plugin SDK
- GORM for database operations
- SQLite for local storage

## Best Practices

This plugin demonstrates several best practices for Viewra plugin development:

1. **Modular Architecture**: Clear separation between configuration, models, and business logic
2. **Type Safety**: Comprehensive CUE schemas with validation
3. **Error Handling**: Robust error handling with circuit breakers and retries
4. **Resource Management**: Proper cleanup and connection pooling
5. **Caching Strategy**: Intelligent caching to minimize external API calls
6. **Rate Limiting**: Respectful API usage following service guidelines
7. **Documentation**: Comprehensive inline documentation and examples

## Contributing

When extending this plugin or creating similar plugins, please:

1. Follow the established architectural patterns
2. Maintain comprehensive test coverage
3. Update documentation for any configuration changes
4. Follow MusicBrainz API guidelines and terms of service
5. Consider performance implications of new features

## License

This plugin is part of the Viewra project and follows the same licensing terms.

## References

- [MusicBrainz API Documentation](https://musicbrainz.org/doc/MusicBrainz_API)
- [Cover Art Archive API](https://coverartarchive.org/doc/API)
- [Viewra Plugin SDK](../../../pkg/plugins/)
- [CUE Language Specification](https://cuelang.org/docs/)
