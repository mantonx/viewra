# Viewra Plugin System Architecture

> 📖 **For build instructions and development workflow, see [DEVELOPMENT.md](../DEVELOPMENT.md)**

## Overview

The Viewra plugin system extends core functionality through HashiCorp's go-plugin architecture, providing:

- **Subprocess Isolation**: Plugins run as separate processes, ensuring safety
- **Hot Reload**: Plugins can be updated without restarting the main application
- **gRPC Communication**: Efficient and strongly-typed communication
- **Plugin Discovery**: Automatic discovery in designated plugin directories
- **Runtime Management**: Load, unload, and manage plugins at runtime

## Core Architecture Components

### 1. Plugin Manager (`backend/internal/plugins/manager.go`)

- Manages the lifecycle of all plugins (discovery, loading, unloading, restarting)
- Monitors plugin directories for changes to support hot reloading
- Registers discovered plugins and their capabilities
- Handles plugin process management and cleanup

### 2. gRPC Interface & Implementation (`backend/internal/plugins/grpc_impl.go`)

- Implements the server and client sides of the HashiCorp go-plugin gRPC interface
- Defines how the host application communicates with each plugin process
- Includes service definitions for various plugin types

### 3. Protocol Definitions (`backend/internal/plugins/proto/plugin.proto`)

- Contains Protocol Buffer (protobuf) definitions for gRPC services and messages
- Provides the contract for communication, ensuring type safety

### 4. Core Types & Interfaces (`backend/internal/plugins/types.go`)

- Defines the core Go interfaces that plugins must implement
- Includes structs for plugin metadata, configuration, and runtime state

## Plugin Service Interfaces

### Core Plugin Interface (`Implementation`)

All plugins must implement this base interface:

```go
type Implementation interface {
    Initialize(ctx *proto.PluginContext) error
    Start() error
    Stop() error
    Info() (*proto.PluginInfo, error)
    Health() error

    // Service getters (return nil if not implemented)
    MetadataScraperService() MetadataScraperService
    ScannerHookService() ScannerHookService
    DatabaseService() DatabaseService
    AdminPageService() AdminPageService
    APIRegistrationService() APIRegistrationService
    SearchService() SearchService
}
```

### Metadata Scraper Service

For plugins that extract metadata from files:

```go
type MetadataScraperService interface {
    CanHandle(filePath, mimeType string) bool
    ExtractMetadata(filePath string) (map[string]string, error)
    GetSupportedTypes() []string
}
```

### Scanner Hook Service

For plugins that react to scan events:

```go
type ScannerHookService interface {
    OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error
    OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error
    OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error
}
```

### Database Service

For plugins that require their own database tables:

```go
type DatabaseService interface {
    GetModels() []string
    Migrate(connectionString string) error
    Rollback(connectionString string) error
}
```

### Admin Page Service

For plugins that expose configuration/management pages:

```go
type AdminPageService interface {
    GetAdminPages() ([]*proto.AdminPage, error)
    RegisterRoutes(router interface{}) error
}
```

## Plugin Configuration Schema

Plugins use CueLang (`.cue` files) for configuration:

```cue
#Plugin: {
    schema_version: "1.0"
    id:             string // Unique identifier for the plugin
    name:           string // Human-readable name
    version:        string // Semantic version (e.g., "1.0.2")
    description:    string
    author?:        string
    website?:       string
    repository?:    string
    license?:       string
    type:           "metadata_scraper" | "scanner_hook" | "database" | "admin_page" | "generic"
    tags?:          [...string]

    entry_points: {
        main: string // Name of the plugin executable relative to plugin directory
    }

    capabilities?: {
        metadata_scraper?: bool
        scanner_hook?:     bool
        database_service?: bool
        admin_page?:       bool
        api_registration?: bool
        search_service?:   bool
    }

    permissions?: [
        {
            resource: string // e.g., "filesystem", "network", "database"
            actions: [...string] // e.g., ["read", "write"]
        },
    ]

    settings?: {
        // Plugin-specific settings defined here
        api_key?: string
        user_agent?: string
        timeout_seconds?: int | *30
        // ... other settings
    }
}
```

## Implementation Example

### Basic Plugin Structure

```go
package main

import (
    "github.com/hashicorp/go-hclog"
    goplugin "github.com/hashicorp/go-plugin"
    "github.com/mantonx/viewra/internal/plugins"
    "github.com/mantonx/viewra/internal/plugins/proto"
)

type MyPlugin struct {
    logger hclog.Logger
    config *MyPluginConfig
    // ... other fields
}

type MyPluginConfig struct {
    APIKey    string `json:"api_key"`
    UserAgent string `json:"user_agent"`
    Timeout   int    `json:"timeout_seconds"`
}

// Core Plugin Interface Implementation
func (p *MyPlugin) Initialize(ctx *proto.PluginContext) error {
    p.logger = hclog.New(&hclog.LoggerOptions{
        Name:  ctx.PluginId,
        Level: hclog.LevelFromString(ctx.LogLevel),
    })

    // Parse configuration from ctx.Config
    // Initialize connections, etc.

    return nil
}

func (p *MyPlugin) Start() error {
    p.logger.Info("MyPlugin Starting")
    return nil
}

func (p *MyPlugin) Stop() error {
    p.logger.Info("MyPlugin Stopping")
    return nil
}

func (p *MyPlugin) Info() (*proto.PluginInfo, error) {
    return &proto.PluginInfo{
        Id:          "my_plugin_id",
        Name:        "My Awesome Plugin",
        Version:     "1.0.0",
        Description: "Does awesome things",
        Author:      "Developer Name",
        Type:        "metadata_scraper",
        Capabilities: []string{"metadata_scraper"},
    }, nil
}

func (p *MyPlugin) Health() error {
    // Check dependencies, API connectivity, etc.
    return nil
}

// Service Interface Implementations
func (p *MyPlugin) MetadataScraperService() plugins.MetadataScraperService {
    return p // Return self if this plugin implements metadata scraping
}

func (p *MyPlugin) ScannerHookService() plugins.ScannerHookService {
    return nil // Return nil if not implemented
}

// ... implement other service getters

// Metadata Scraper Implementation
func (p *MyPlugin) CanHandle(filePath, mimeType string) bool {
    // Check if this plugin can handle the file type
    return strings.HasSuffix(filePath, ".mp3")
}

func (p *MyPlugin) ExtractMetadata(filePath string) (map[string]string, error) {
    // Extract metadata from the file
    return map[string]string{
        "title":  "Extracted Title",
        "artist": "Extracted Artist",
    }, nil
}

func (p *MyPlugin) GetSupportedTypes() []string {
    return []string{"audio/mpeg", "audio/mp3"}
}

func main() {
    goplugin.Serve(&goplugin.ServeConfig{
        HandshakeConfig: plugins.Handshake,
        Plugins: map[string]goplugin.Plugin{
            "plugin": &plugins.GRPCPlugin{Impl: &MyPlugin{}},
        },
        GRPCServer: goplugin.DefaultGRPCServer,
    })
}
```

## Plugin Lifecycle

1. **Discovery**: Plugin manager scans plugin directories for `.cue` configuration files
2. **Validation**: Configuration is validated against the schema
3. **Loading**: Plugin binary is launched as a subprocess
4. **Handshake**: gRPC connection is established using HashiCorp's plugin protocol
5. **Initialization**: `Initialize()` method is called with context and configuration
6. **Service Registration**: Plugin services are registered with the host application
7. **Runtime**: Plugin responds to service calls from the host
8. **Shutdown**: `Stop()` method is called, process is terminated gracefully

## Plugin Communication Flow

```
Host Application
       ↓
Plugin Manager
       ↓ (gRPC)
Plugin Process
       ↓
Service Implementation
       ↓
External APIs/Resources
```

## Security Considerations

- **Process Isolation**: Plugins run in separate processes, limiting impact of crashes
- **Permission System**: Plugins declare required permissions in configuration
- **Resource Limits**: Host can enforce CPU, memory, and network limits
- **API Validation**: All gRPC messages are validated against protobuf schemas

## Performance Considerations

- **Lazy Loading**: Plugins are only loaded when needed
- **Connection Pooling**: gRPC connections are reused for efficiency
- **Timeout Handling**: Operations have configurable timeouts
- **Resource Monitoring**: Host monitors plugin resource usage

## Debugging Plugins

### Logging

Plugins should use structured logging with appropriate levels:

```go
p.logger.Debug("Processing file", "path", filePath)
p.logger.Info("Plugin started successfully")
p.logger.Warn("API rate limit approaching")
p.logger.Error("Failed to connect to external service", "error", err)
```

### Health Checks

Implement comprehensive health checks:

```go
func (p *MyPlugin) Health() error {
    // Check external API connectivity
    if err := p.checkAPIConnection(); err != nil {
        return fmt.Errorf("API connection failed: %w", err)
    }

    // Check database connectivity
    if err := p.checkDatabaseConnection(); err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }

    return nil
}
```

### Error Handling

Use proper error wrapping and context:

```go
func (p *MyPlugin) ExtractMetadata(filePath string) (map[string]string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
    }
    defer file.Close()

    // ... processing
}
```
