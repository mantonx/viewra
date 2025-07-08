# Plugin Development Guide

This guide covers developing plugins for Viewra's extensible architecture.

## Table of Contents
- [Overview](#overview)
- [Plugin Types](#plugin-types)
- [Quick Start](#quick-start)
- [Plugin Structure](#plugin-structure)
- [Development Workflow](#development-workflow)
- [Plugin Configuration](#plugin-configuration)
- [Building Plugins](#building-plugins)
- [Testing Plugins](#testing-plugins)
- [Best Practices](#best-practices)

## Overview

Viewra uses HashiCorp's go-plugin framework for extensible functionality. Plugins run as separate processes and communicate via gRPC, providing isolation and language flexibility.

## Plugin Types

### 1. Transcoding Providers
Convert media files to different formats and codecs.

**Interface:**
```go
type TranscodingProvider interface {
    GetInfo() ProviderInfo
    GetSupportedFormats() []ContainerFormat
    StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeHandle, error)
    GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)
    StopTranscode(handle *TranscodeHandle) error
}
```

**Examples:**
- `ffmpeg_software` - CPU-based transcoding
- `ffmpeg_nvidia` - NVIDIA GPU acceleration
- `ffmpeg_vaapi` - Intel/AMD GPU acceleration
- `ffmpeg_qsv` - Intel Quick Sync Video

### 2. Metadata Scrapers
Extract metadata from media files.

**Interface:**
```go
type MetadataScraperService interface {
    GetName() string
    GetVersion() string
    ExtractMetadata(ctx context.Context, filePath string) (map[string]string, error)
    GetSupportedFormats() []string
}
```

### 3. Enrichment Services
Enhance metadata from external sources.

**Interface:**
```go
type EnrichmentService interface {
    GetServiceInfo() ServiceInfo
    EnrichMovie(ctx context.Context, req *MovieRequest) (*MovieMetadata, error)
    EnrichTVShow(ctx context.Context, req *TVShowRequest) (*TVShowMetadata, error)
    EnrichMusic(ctx context.Context, req *MusicRequest) (*MusicMetadata, error)
}
```

**Examples:**
- `tmdb_enricher` - Movie/TV metadata from TMDB
- `musicbrainz_enricher` - Music metadata
- `audiodb_enricher` - Album artwork and info

## Quick Start

### 1. Setup Development Environment
```bash
# Complete setup
make plugin-dev

# Or manual steps:
make plugin-setup
make plugin-build-all
make plugin-refresh
```

### 2. Create New Plugin

#### Directory Structure
```bash
mkdir -p plugins/my_plugin
cd plugins/my_plugin
```

#### Required Files

**`plugin.cue`** - Plugin configuration:
```cue
plugin_name: "my_plugin"
plugin_type: "transcoder"  // or "enrichment", "scanner"
version: "1.0.0"
enabled: true

// Type-specific configuration
config: {
    // Plugin settings
}
```

**`main.go`** - Plugin implementation:
```go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/mantonx/viewra/sdk"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: sdk.HandshakeConfig,
        Plugins: map[string]plugin.Plugin{
            "transcoder": &sdk.TranscodingProviderPlugin{
                Impl: &MyTranscoder{},
            },
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}

type MyTranscoder struct {
    // Implementation
}
```

**`go.mod`** - Dependencies:
```go
module github.com/mantonx/viewra/plugins/my_plugin

go 1.21

require (
    github.com/hashicorp/go-plugin v1.6.0
    github.com/mantonx/viewra/sdk v0.1.0
)

replace github.com/mantonx/viewra/sdk => ../../sdk
```

### 3. Build and Deploy
```bash
# Build specific plugin
make plugin-build p=my_plugin

# Hot reload (disable → rebuild → enable)
make plugin-reload p=my_plugin

# List all plugins
make plugin-list
```

## Plugin Structure

### Standard Layout
```
plugins/my_plugin/
├── main.go           # Plugin entry point
├── plugin.cue        # Configuration schema
├── go.mod           # Go module definition
├── go.sum           # Dependency checksums
├── README.md        # Documentation
├── Dockerfile       # Container build (optional)
└── internal/        # Internal packages
    ├── config/      # Configuration handling
    ├── processor/   # Core logic
    └── utils/       # Utilities
```

### Configuration Schema

All plugins use CueLang for type-safe configuration:

```cue
// Required fields
plugin_name: string
plugin_type: "transcoder" | "enrichment" | "scanner"
version: string & =~"^\\d+\\.\\d+\\.\\d+$"
enabled: bool | *true

// Optional metadata
description: string
author: string
license: string
repository: string

// Type-specific capabilities
capabilities: {
    // For transcoders
    formats?: [...string]
    codecs?: [...string]
    hardware_acceleration?: string
    
    // For enrichers
    media_types?: [...string]
    rate_limit?: int
}

// Plugin configuration
config: {
    // Custom settings with defaults
    setting1: string | *"default"
    setting2: int | *100
    setting3: bool | *false
}
```

## Development Workflow

### 1. Container-Based Development

All plugins are built inside Docker for consistency:

```bash
# Setup environment
docker-compose up -d

# Build in container
docker-compose exec backend \
    /app/scripts/build-plugin.sh my_plugin

# Check logs
docker-compose logs backend | grep plugin
```

### 2. Hot Reload Workflow

```bash
# 1. Make code changes
vim plugins/my_plugin/main.go

# 2. Hot reload
make plugin-reload p=my_plugin

# 3. Test changes
make plugin-test p=my_plugin
```

### 3. VS Code Integration

Available tasks (Ctrl+Shift+P → "Run Task"):
- `Plugin: Setup Development Environment`
- `Plugin: Build [Plugin Name]`
- `Plugin: Hot Reload [Plugin Name]`
- `Plugin: List All Plugins`
- `Plugin: Test [Plugin Name]`

## Plugin Configuration

### Runtime Configuration

Plugins can be configured via:

1. **CUE Files** (Recommended)
```cue
// plugins/my_plugin/plugin.cue
config: {
    api_key: string @tag(secret)
    timeout: int | *30
    cache_dir: string | *"/tmp/cache"
}
```

2. **Environment Variables**
```bash
MY_PLUGIN_API_KEY=secret123
MY_PLUGIN_TIMEOUT=60
```

3. **API Updates**
```bash
curl -X PUT http://localhost:8080/api/v1/plugins/my_plugin/config \
  -H "Content-Type: application/json" \
  -d '{"api_key": "new-key", "timeout": 45}'
```

### Configuration Validation

CueLang provides automatic validation:
```cue
config: {
    // Type constraints
    port: int & >=1 & <=65535
    
    // Regular expressions
    email: string & =~"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
    
    // Enums
    log_level: "debug" | "info" | "warn" | "error"
    
    // Conditional fields
    if enabled {
        required_field: string
    }
}
```

## Building Plugins

### Local Development Build
```bash
cd plugins/my_plugin
go build -o my_plugin
```

### Production Build (Container)
```bash
# Single plugin
make plugin-build p=my_plugin

# All plugins
make plugin-build-all
```

### Build Script Details

The build script (`scripts/build-plugin.sh`):
1. Validates plugin directory exists
2. Runs `go mod tidy` for dependencies
3. Builds with production flags
4. Copies binary and config to data directory
5. Sets correct permissions

### Docker Build (Optional)

For plugins with special dependencies:

```dockerfile
# plugins/my_plugin/Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN go build -o my_plugin

FROM alpine:latest
RUN apk add --no-cache ffmpeg
COPY --from=builder /build/my_plugin /usr/local/bin/
ENTRYPOINT ["my_plugin"]
```

## Testing Plugins

### 1. Unit Tests
```go
// plugins/my_plugin/main_test.go
func TestTranscode(t *testing.T) {
    plugin := &MyTranscoder{}
    req := &TranscodeRequest{
        InputPath: "test.mp4",
        Container: "dash",
    }
    
    handle, err := plugin.StartTranscode(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, handle)
}
```

### 2. Integration Tests
```bash
# Test plugin functionality
make plugin-test p=my_plugin

# Manual test via API
curl http://localhost:8080/api/v1/plugins/my_plugin/test
```

### 3. Load Testing
```go
// Test concurrent operations
func TestConcurrentTranscode(t *testing.T) {
    plugin := &MyTranscoder{}
    var wg sync.WaitGroup
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            // Test concurrent transcoding
        }(i)
    }
    
    wg.Wait()
}
```

## Best Practices

### 1. Error Handling
```go
// Use structured errors
type PluginError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

func (e *PluginError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
```

### 2. Logging
```go
import "github.com/hashicorp/go-hclog"

type MyPlugin struct {
    logger hclog.Logger
}

func (p *MyPlugin) Init(logger hclog.Logger) {
    p.logger = logger.Named("my_plugin")
    p.logger.Info("Plugin initialized")
}
```

### 3. Resource Management
```go
// Clean up resources
func (p *MyPlugin) Shutdown() error {
    p.logger.Info("Shutting down plugin")
    
    // Close connections
    if p.conn != nil {
        p.conn.Close()
    }
    
    // Clean temporary files
    os.RemoveAll(p.tempDir)
    
    return nil
}
```

### 4. Configuration Handling
```go
// Validate configuration on load
func (p *MyPlugin) Configure(config map[string]interface{}) error {
    // Type-safe configuration parsing
    if err := mapstructure.Decode(config, &p.config); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    
    // Validate required fields
    if p.config.APIKey == "" {
        return errors.New("api_key is required")
    }
    
    return nil
}
```

### 5. Graceful Degradation
```go
// Handle failures gracefully
func (p *MyPlugin) Process(input string) (string, error) {
    // Try primary method
    result, err := p.primaryProcess(input)
    if err == nil {
        return result, nil
    }
    
    p.logger.Warn("Primary process failed, trying fallback", "error", err)
    
    // Fallback method
    return p.fallbackProcess(input)
}
```

## Plugin Lifecycle

### 1. Discovery
- Viewra scans plugin directory on startup
- Validates plugin.cue configuration
- Attempts to start plugin process

### 2. Initialization
- Plugin handshake via gRPC
- Configuration loading
- Capability registration

### 3. Runtime
- Plugin receives requests via gRPC
- Processes operations
- Returns results or errors

### 4. Shutdown
- Graceful shutdown signal
- Resource cleanup
- Process termination

## Troubleshooting

### Plugin Won't Start
```bash
# Check plugin binary exists
ls -la data/plugins/my_plugin/

# Check plugin logs
docker-compose logs backend | grep my_plugin

# Test plugin directly
./data/plugins/my_plugin/my_plugin
```

### Configuration Issues
```bash
# Validate CUE configuration
cue eval plugins/my_plugin/plugin.cue

# Check loaded configuration
curl http://localhost:8080/api/v1/plugins/my_plugin
```

### Performance Issues
```bash
# Monitor plugin resources
docker stats

# Check plugin metrics
curl http://localhost:8080/api/v1/plugins/my_plugin/metrics
```

## Advanced Topics

### 1. Plugin Communication
Plugins can communicate with Viewra services:
```go
// Access media service
mediaClient := sdk.NewMediaClient(config.ViewraURL)
mediaInfo, err := mediaClient.GetMediaFile(ctx, mediaID)
```

### 2. Distributed Plugins
Run plugins on separate machines:
```yaml
# plugin.cue
remote: {
    enabled: true
    address: "plugin-host:50051"
    tls: {
        enabled: true
        cert_file: "/certs/plugin.crt"
        key_file: "/certs/plugin.key"
    }
}
```

### 3. Plugin Marketplace
Share plugins with the community:
1. Publish to GitHub
2. Add to Viewra plugin registry
3. Enable one-click installation

## Next Steps

1. Explore example plugins in `/plugins/`
2. Create your first plugin
3. Test with real media files
4. Share with the community

For more examples and advanced patterns, see the [Plugin Examples](https://github.com/mantonx/viewra/tree/main/plugins) directory.