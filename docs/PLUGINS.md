# Viewra Plugin System Architecture

Viewra uses a modular plugin architecture built on HashiCorp's go-plugin framework to provide extensible functionality for media processing, metadata enrichment, and custom features.

## Overview

The plugin system is designed to be:

- **Isolated**: Plugins run in separate processes and communicate via gRPC
- **Modular**: Clean interfaces with minimal dependencies on core application
- **Extensible**: Support for multiple service types within a single plugin
- **Developer-Friendly**: Simple SDK for external plugin development

## Architecture

### Core Components

1. **Plugin SDK** (`backend/pkg/plugins/`)

   - Standalone Go module for plugin development
   - Clean interfaces without dependencies on main application
   - gRPC communication helpers
   - Standard plugin lifecycle management

2. **Plugin Manager** (`backend/internal/plugins/`)

   - Discovers and loads plugins at runtime
   - Manages plugin lifecycle (start/stop/health checks)
   - Routes service calls to appropriate plugins
   - Handles plugin communication and error recovery

3. **Build System**
   - Automated plugin building with `make build-plugin`
   - Smart CGO detection and container builds
   - Cross-platform compatibility

## Plugin Types & Services

Plugins can implement multiple service interfaces:

### MetadataScraperService

Extracts metadata from media files.

```go
type MetadataScraperService interface {
    CanHandle(filePath, mimeType string) bool
    ExtractMetadata(filePath string) (map[string]string, error)
    GetSupportedTypes() []string
}
```

### ScannerHookService

Hooks into the media scanning process.

```go
type ScannerHookService interface {
    OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error
    OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error
    OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error
}
```

### SearchService

Provides search capabilities across external data sources.

```go
type SearchService interface {
    Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*SearchResult, uint32, bool, error)
    GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error)
}
```

### DatabaseService

Manages plugin-specific database models and migrations.

```go
type DatabaseService interface {
    GetModels() []string
    Migrate(connectionString string) error
    Rollback(connectionString string) error
}
```

### APIRegistrationService

Registers custom API endpoints.

```go
type APIRegistrationService interface {
    GetRegisteredRoutes(ctx context.Context) ([]*APIRoute, error)
}
```

### AssetService

Manages media assets (artwork, fanart, etc.).

```go
type AssetService interface {
    SaveAsset(mediaFileID uint32, assetType, category, subtype string, data []byte, mimeType, sourceURL string, metadata map[string]string) (uint32, string, string, error)
    AssetExists(mediaFileID uint32, assetType, category, subtype, hash string) (bool, uint32, string, error)
    RemoveAsset(assetID uint32) error
}
```

## Plugin Development

### Template Plugin

For a comprehensive reference implementation, see the **Template Plugin** at `backend/pkg/plugins/template/`:

- **Complete SDK showcase**: Demonstrates all service interfaces and modern patterns
- **Comprehensive documentation**: Includes README with code examples and best practices
- **Production-ready structure**: Database models, configuration, logging, error handling
- **Copy-and-modify approach**: Easy starting point for new plugin development
- **Part of SDK**: Co-located with interface definitions for easy reference

**Quick start with template**:

```bash
# Copy template to create new plugin
cp -r backend/pkg/plugins/template backend/data/plugins/my_plugin
cd backend/data/plugins/my_plugin

# Update module path and identifiers
sed -i 's|pkg/plugins/template|plugins/my_plugin|' go.mod
sed -i 's|=> ../|=> ../../../pkg/plugins|' go.mod
# Update plugin.cue identifiers and implement your logic
make build-plugin p=my_plugin
```

### Quick Start

1. **Create Plugin Directory**

```bash
mkdir backend/data/plugins/my_plugin
cd backend/data/plugins/my_plugin
```

2. **Initialize Go Module**

```bash
go mod init github.com/mantonx/viewra/plugins/my_plugin
```

3. **Add Dependencies**

```go
// go.mod
require (
    github.com/mantonx/viewra/pkg/plugins v0.0.0
)

replace github.com/mantonx/viewra/pkg/plugins => ../../../pkg/plugins
```

4. **Implement Plugin**

```go
// main.go
package main

import (
    "github.com/mantonx/viewra/pkg/plugins"
)

type MyPlugin struct {
    logger plugins.Logger
    config *Config
}

func (p *MyPlugin) Initialize(ctx *plugins.PluginContext) error {
    p.logger = ctx.Logger
    // Initialize plugin
    return nil
}

func (p *MyPlugin) Start() error {
    p.logger.Info("My plugin started")
    return nil
}

func (p *MyPlugin) Stop() error {
    return nil
}

func (p *MyPlugin) Info() (*plugins.PluginInfo, error) {
    return &plugins.PluginInfo{
        ID:          "my_plugin",
        Name:        "My Plugin",
        Version:     "1.0.0",
        Type:        "metadata_scraper",
        Description: "Example plugin",
        Author:      "Your Name",
    }, nil
}

func (p *MyPlugin) Health() error {
    return nil
}

// Implement service interfaces as needed
func (p *MyPlugin) MetadataScraperService() plugins.MetadataScraperService {
    return p
}

// ... other service methods return nil if not implemented

func main() {
    plugin := &MyPlugin{}
    plugins.StartPlugin(plugin)
}
```

5. **Build Plugin**

```bash
make build-plugin p=my_plugin
```

### Configuration

Plugins receive configuration through the `PluginContext.Config` field during initialization. Configuration should be JSON-serializable:

```go
type Config struct {
    Enabled     bool   `json:"enabled"`
    APIKey      string `json:"api_key"`
    UserAgent   string `json:"user_agent"`
    // ... other config fields
}
```

### Logging

Use the provided logger interface for consistent logging:

```go
func (p *MyPlugin) someMethod() {
    p.logger.Info("Processing started", "file", filepath)
    p.logger.Debug("Debug info", "details", data)
    p.logger.Warn("Warning message", "reason", reason)
    p.logger.Error("Error occurred", "error", err)
}
```

### Database Integration

Plugins can define their own database models and migrations:

```go
type MyModel struct {
    ID        uint      `gorm:"primaryKey"`
    MediaFileID uint    `gorm:"not null;index"`
    Data      string    `gorm:"type:text"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (p *MyPlugin) GetModels() []string {
    return []string{"MyModel"}
}

func (p *MyPlugin) Migrate(connectionString string) error {
    db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
    if err != nil {
        return err
    }
    return db.AutoMigrate(&MyModel{})
}
```

## Built-in Plugins

### Template Plugin

**Location**: `backend/pkg/plugins/template/`

Comprehensive reference implementation demonstrating all plugin capabilities:

- **Services**: All service interfaces (MetadataScraperService, ScannerHookService, SearchService, DatabaseService, APIRegistrationService)
- **Features**: Complete SDK usage, database models, configuration validation, structured logging
- **Purpose**: Developer reference and starting point for new plugins
- **Documentation**: Extensive README with code patterns and best practices
- **SDK Integration**: Part of the plugin SDK package for easy discovery

### MusicBrainz Enricher

**Location**: `backend/data/plugins/musicbrainz_enricher/`

Enriches music metadata using the MusicBrainz database:

- **Services**: MetadataScraperService, ScannerHookService, SearchService, DatabaseService, APIRegistrationService
- **Features**: Caching, rate limiting, fuzzy matching, score-based selection
- **API**: `/api/plugins/musicbrainz/search`

### AudioDB Enricher

**Location**: `backend/data/plugins/audiodb_enricher/`

Enriches music metadata using The AudioDB API:

- **Services**: MetadataScraperService, ScannerHookService, SearchService, DatabaseService, APIRegistrationService
- **Features**: Artist/album artwork, biography, genre classification
- **API**: `/api/plugins/audiodb/search`, `/api/plugins/audiodb/enrich`

## Build System

### Building Plugins

**All plugin builds now use Docker containers for maximum consistency and reliability.**

```bash
# Build single plugin (Docker only)
make build-plugin p=plugin_name

# Build all plugins (Docker only)
make build-plugins

# Check Docker environment
make enforce-docker-builds
```

### Build Requirements

- **Docker**: Required for all plugin builds
- **Backend Container**: Must be running (`docker-compose up -d backend`)
- **Architecture**: Automatically detects and builds for container architecture

### Build Process

1. **Environment Check**: Verifies Docker is available and backend container is running
2. **Container Build**: Compiles plugin inside Docker container with proper dependencies
3. **Binary Verification**: Validates the compiled binary is executable in the container
4. **Deployment**: Ensures binary is available to the running backend

### Deprecated Features

- **Host Builds**: No longer supported for consistency and reliability
- **Build Mode Selection**: All builds now use containers automatically

### CGO Detection

The build system automatically detects CGO requirements by analyzing:

- Import statements in Go files
- Dependencies in `go.mod`
- Transitive dependencies in `go.sum`

All plugins (CGO and non-CGO) now use container builds for maximum compatibility.

## Plugin Configuration (CUE)

### Improved CUE Parsing

The plugin system now includes enhanced CUE parsing with:

- **Multiline String Support**: Properly handles JWT tokens and long configuration values
- **Whitespace Normalization**: Automatically cleans up extra whitespace from parsed values
- **Better Error Reporting**: Detailed error messages with line numbers and context
- **Nested Structure Support**: Handles complex configuration hierarchies

### Configuration Loading Priority

1. **Runtime Overrides**: Configuration passed from host application
2. **CUE File Settings**: Values from `plugin.cue` files
3. **Default Values**: Struct tag defaults

### Example CUE Configuration

```cue
#Plugin: {
    // Plugin metadata
    id: "example_plugin"
    name: "Example Plugin"
    enabled_by_default: true
    
    // Configuration settings
    settings: {
        api: {
            // JWT tokens are now properly parsed
            key: "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI1YTU2ODc0YjRmMzU4YjIzZDhkM2YzZmI5ZDc4NDNiOSIsIm5iZiI6MTc0ODYzOTc1Ny40MDEsInN1YiI6IjY4M2EyMDBkNzA5OGI4MzMzNThmZThmOSIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.OXT68T0EtU-WXhcP7nwyWjMePuEuCpfWtDlvdntWKw8"
            rate_limit: 40
            timeout: 30
        }
        
        features: {
            auto_enrich: bool | *true
            enable_artwork: bool | *true
        }
    }
}
```

### Configuration Validation

- **API Key Validation**: Supports both legacy 32-character hex keys and JWT tokens
- **Type Validation**: Ensures configuration values match expected types
- **Range Validation**: Validates numeric values are within acceptable ranges
- **Required Fields**: Validates all required configuration is present

## Plugin Discovery

Plugins are discovered at runtime from:

1. `backend/data/plugins/*/plugin_name` (compiled binaries)
2. Plugin metadata in `plugin.cue` files
3. Dynamic registration via plugin manager

## Communication Protocol

Plugins communicate with the host application via gRPC:

- **Handshake**: Secure plugin authentication
- **Service Discovery**: Dynamic capability detection
- **Health Checks**: Automatic plugin monitoring
- **Graceful Shutdown**: Clean resource cleanup

## Development Best Practices

### Error Handling

- Return descriptive errors with context
- Use structured logging for debugging
- Implement proper health checks

### Performance

- Implement caching for external API calls
- Use rate limiting for API requests
- Minimize database queries

### Configuration

- Provide sensible defaults
- Validate configuration on startup
- Support runtime configuration updates

### Testing

- Write unit tests for core logic
- Mock external dependencies
- Test plugin lifecycle management

### Security

- Validate all input data
- Use secure communication protocols
- Implement proper authentication for external APIs

## Migration from Legacy Plugins

Legacy plugins using internal interfaces can be migrated to the new architecture:

1. **Update Dependencies**: Replace internal imports with `github.com/mantonx/viewra/pkg/plugins`
2. **Update Interfaces**: Implement new service interfaces
3. **Update Configuration**: Use new configuration structure
4. **Update Build**: Use new build system commands
5. **Test Integration**: Verify plugin loads and functions correctly

## Troubleshooting

### Common Issues

1. **Build Failures**

   - Check CGO dependencies
   - Verify Go module structure
   - Ensure proper replace directives

2. **Plugin Not Loading**

   - Check plugin binary permissions
   - Verify plugin responds to `--help`
   - Check plugin logs for initialization errors

3. **Service Registration**
   - Implement required interface methods
   - Return proper service instances
   - Check method signatures match interfaces

### Debugging

Enable debug logging to troubleshoot plugin issues:

```bash
LOG_LEVEL=debug ./viewra-backend
```

Check plugin-specific logs:

```bash
tail -f backend/data/plugins/plugin_name/plugin.log
```
