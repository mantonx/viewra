# Viewra Plugin SDK

This package provides the SDK for developing Viewra plugins. It contains the interfaces, types, and utilities needed to create plugins that integrate with the Viewra media server.

## Overview

The Viewra Plugin SDK enables developers to create isolated plugins that extend Viewra's functionality through well-defined service interfaces. Plugins run as separate processes and communicate with the main application via gRPC.

## Architecture

- **Process Isolation**: Plugins run as separate binaries communicating via gRPC
- **Service Interfaces**: Multiple service types can be implemented in a single plugin
- **Clean Dependencies**: SDK has no dependencies on internal Viewra code
- **Plugin Lifecycle**: Structured initialization, startup, shutdown, and health checking

## Service Interfaces

### Core Plugin Interface

All plugins must implement the base `Plugin` interface:

```go
type Plugin interface {
    Initialize(ctx *PluginContext) error
    Start() error
    Stop() error
    Info() (*PluginInfo, error)
    Health() error

    // Service accessors - return nil if not implemented
    MetadataScraperService() MetadataScraperService
    ScannerHookService() ScannerHookService
    SearchService() SearchService
    DatabaseService() DatabaseService
    APIRegistrationService() APIRegistrationService
    AssetService() AssetService
    AdminPageService() AdminPageService
}
```

### Available Services

1. **MetadataScraperService** - Extract metadata from media files
2. **ScannerHookService** - Hook into media scanning lifecycle
3. **SearchService** - Provide search capabilities
4. **DatabaseService** - Manage plugin database models
5. **APIRegistrationService** - Register custom API endpoints
6. **AssetService** - Manage media assets (artwork, etc.)
7. **AdminPageService** - Provide admin interface pages

See `interfaces.go` for complete interface definitions.

## Template Plugin

A comprehensive reference implementation is available in the `template/` directory:

```
backend/pkg/plugins/template/
├── main.go         # Complete plugin implementation
├── go.mod          # SDK dependencies
├── plugin.cue      # Configuration schema
└── README.md       # Detailed documentation
```

The template demonstrates:

- All service interface implementations
- Database integration with GORM
- Configuration management
- Error handling patterns
- Logging best practices
- API endpoint registration

## Quick Start

1. **Copy the template**:

   ```bash
   cp -r backend/pkg/plugins/template backend/data/plugins/my_plugin
   cd backend/data/plugins/my_plugin
   ```

2. **Update module and identifiers**:

   ```bash
   # Update go.mod module name
   sed -i 's|pkg/plugins/template|plugins/my_plugin|' go.mod

   # Fix replace directive path
   sed -i 's|=> ../|=> ../../../pkg/plugins|' go.mod

   # Update plugin.cue identifiers
   # Edit main.go implementation
   ```

3. **Build and test**:
   ```bash
   go mod tidy
   make build-plugin p=my_plugin
   make test-plugin p=my_plugin
   ```

## Plugin Context

Plugins receive a `PluginContext` during initialization:

```go
type PluginContext struct {
    Logger      Logger
    DatabaseURL string
    BasePath    string
}
```

- **Logger**: Structured logger instance
- **DatabaseURL**: SQLite database connection string
- **BasePath**: Plugin's data directory

## Configuration

Plugins define their configuration schema in `plugin.cue` files using CueLang:

```cue
#Plugin: {
    schema_version: "1.0"

    id:          "my_plugin"
    name:        "My Plugin"
    version:     "1.0.0"
    description: "Plugin description"
    author:      "Author Name"
    type:        "metadata_scraper"

    capabilities: {
        metadata_extraction: true
        database_access:     true
        // ...
    }

    permissions: [
        "database:read",
        "database:write",
        "network:external"
    ]

    settings: {
        enabled: bool | *true
        // Custom configuration fields
    }
}
```

## Database Integration

Plugins can define custom database models using GORM:

```go
type MyModel struct {
    ID          uint      `gorm:"primaryKey"`
    MediaFileID uint      `gorm:"not null;index"`
    Data        string    `gorm:"type:text"`
    CreatedAt   time.Time `gorm:"autoCreateTime"`
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

## Error Handling

Use proper error wrapping and context:

```go
func (p *MyPlugin) doSomething() error {
    if !p.config.Enabled {
        return fmt.Errorf("plugin is disabled")
    }

    if err := someOperation(); err != nil {
        p.logger.Error("Operation failed", "error", err)
        return fmt.Errorf("failed to perform operation: %w", err)
    }

    return nil
}
```

## Logging

Use structured logging with key-value pairs:

```go
p.logger.Info("Processing file", "path", filePath, "size", fileSize)
p.logger.Debug("Debug details", "metadata", debugData)
p.logger.Warn("Warning occurred", "reason", reason)
p.logger.Error("Error occurred", "error", err, "context", contextInfo)
```

## API Routes

Register custom API endpoints:

```go
func (p *MyPlugin) GetRegisteredRoutes(ctx context.Context) ([]*APIRoute, error) {
    return []*APIRoute{
        {
            Method:      "GET",
            Path:        "/api/plugins/myplugin/info",
            Description: "Get plugin information",
        },
        {
            Method:      "POST",
            Path:        "/api/plugins/myplugin/process",
            Description: "Process data",
        },
    }, nil
}
```

## Building Plugins

Plugins are built using the Viewra build system:

```bash
# Build single plugin
make build-plugin p=plugin_name

# Build all plugins
make build-plugins

# Test plugin
make test-plugin p=plugin_name
```

The build system automatically:

- Detects CGO dependencies
- Uses container builds for compatibility
- Optimizes binaries with proper flags
- Validates plugin responses

## Files

- `interfaces.go` - All service interface definitions
- `goplugin.go` - Plugin startup and gRPC helpers
- `template/` - Reference implementation
- `go.mod` - SDK module definition

## Best Practices

1. **Implement health checks** - Return meaningful health status
2. **Handle configuration** - Validate settings on startup
3. **Clean up resources** - Close connections in Stop()
4. **Use structured logging** - Include context in log messages
5. **Return proper errors** - Wrap errors with context
6. **Test thoroughly** - Unit and integration tests
7. **Document configuration** - Clear schema in plugin.cue

## Examples

- **Template Plugin**: `template/` - Comprehensive reference
- **MusicBrainz Enricher**: `../../data/plugins/musicbrainz_enricher/` - Production example
- **AudioDB Enricher**: `../../data/plugins/audiodb_enricher/` - API integration example

## See Also

- [Plugin System Documentation](../../docs/PLUGINS.md)
- [Template Plugin README](template/README.md)
- [Build System Documentation](../../docs/PLUGINS.md#build-system)
