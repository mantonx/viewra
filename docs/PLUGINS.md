# Viewra Plugin System

## Overview

The Viewra plugin system allows for extending the core functionality of the application. It is built upon HashiCorp's go-plugin architecture, providing:

- **Subprocess Isolation**: Plugins run as separate processes, ensuring safety and preventing a crashing plugin from taking down the main application.
- **Hot Reload**: Plugins can often be updated, restarted, or added/removed without restarting the main Viewra application (depending on the plugin type and changes).
- **gRPC Communication**: Efficient and strongly-typed communication between the Viewra host application and plugins.
- **Plugin Discovery**: Automatic discovery of plugins placed in designated plugin directories.
- **Runtime Management**: Capabilities to load, unload, and manage plugins at runtime.

## Architecture

### Core Components

1.  **Plugin Manager** (`backend/internal/plugins/manager.go`)

    - Manages the lifecycle of all plugins (discovery, loading, unloading, restarting).
    - Monitors plugin directories for changes (e.g., new plugins, updates) to support hot reloading.
    - Registers discovered plugins and their capabilities.
    - Handles plugin process management and cleanup.

2.  **gRPC Interface & Implementation** (`backend/internal/plugins/grpc_impl.go`)

    - Implements the server and client sides of the HashiCorp go-plugin gRPC interface.
    - Defines how the host application communicates with each plugin process.
    - Includes service definitions for various plugin types (see below).

3.  **Protocol Definitions** (`backend/internal/plugins/proto/plugin.proto`)

    - Contains the Protocol Buffer (protobuf) definitions for gRPC services and messages.
    - These definitions provide the contract for communication, ensuring type safety.

4.  **Core Types & Interfaces** (`backend/internal/plugins/types.go`)
    - Defines the core Go interfaces that plugins must implement (e.g., `Implementation` interface).
    - Includes structs for plugin metadata, configuration (`Config`), and runtime state (`Plugin`).

### Plugin Configuration (CueLang)

Plugins are configured using CueLang (`.cue` files), replacing the previous YAML-based system. Each plugin must have a `plugin.cue` file in its root directory.

**Key benefits of CueLang:**

- **Type Safety & Validation**: Configurations are validated against a schema, reducing errors.
- **Modularity & Reusability**: CueLang allows for defining and importing configuration schemas.
- **Expressiveness**: More powerful than YAML for defining complex configurations and constraints.

**Example `plugin.cue` structure:**

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
    }

    permissions?: [
        {
            resource: string // e.g., "filesystem", "network", "database"
            actions: [...string] // e.g., ["read", "write"]
        },
    ]

    settings?: {
        // Plugin-specific settings defined here
        // Example:
        // my_setting: string | *"default_value"
        // api_key?: string
    }
}
```

### Supported Plugin Service Types

Plugins can implement one or more service interfaces to extend different parts of Viewra:

- **Core Plugin Interface** (`Implementation` in `types.go`):
  - All plugins must implement this base interface.
  - Methods: `Initialize`, `Start`, `Stop`, `Info`, `Health`.
- **Metadata Scraper Service** (`MetadataScraperService` in `types.go`):
  - For plugins that extract metadata from files (e.g., music tags, video resolution).
  - Methods: `CanHandle`, `ExtractMetadata`, `GetSupportedTypes`.
- **Scanner Hook Service** (`ScannerHookService` in `types.go`):
  - For plugins that need to react to events during media library scans.
  - Methods: `OnMediaFileScanned`, `OnScanStarted`, `OnScanCompleted`.
- **Database Service** (`DatabaseService` in `types.go`):
  - For plugins that require their own database tables or need to interact with the database in a custom way.
  - Methods: `GetModels` (to define GORM models), `Migrate`, `Rollback`.
- **Admin Page Service** (`AdminPageService` in `types.go`):
  - For plugins that want to expose configuration or management pages within the Viewra admin interface.
  - Methods: `GetAdminPages`, `RegisterRoutes`.

## Developing a Plugin

Plugins are standalone Go applications that are compiled into executables.

1.  **Project Structure:**
    A typical plugin will have a structure like this:

    ```
    my_viewra_plugin/
    ├── main.go          # Plugin implementation (main package)
    ├── plugin.cue       # Plugin configuration and manifest
    ├── go.mod           # Go module definition for the plugin
    ├── go.sum           # Go module checksums
    └── (any_other_plugin_specific_code_or_assets)
    ```

2.  **Implement Plugin Interfaces:**

    - In your `main.go`, define a struct that will be your plugin's implementation.
    - This struct must satisfy the `plugins.Implementation` interface from Viewra's `backend/internal/plugins/types.go`.
    - Optionally, implement any of the service-specific interfaces (`MetadataScraperService`, `ScannerHookService`, etc.) by having your main plugin struct return an implementation for those services (often, the main struct itself).

    ```go
    package main

    import (
        "github.com/hashicorp/go-hclog"
        goplugin "github.com/hashicorp/go-plugin"
        "github.com/mantonx/viewra/internal/plugins" // Viewra plugin types
        "github.com/mantonx/viewra/internal/plugins/proto" // Viewra plugin protobuf types
    )

    type MyPlugin struct {
        logger hclog.Logger
        // ... other fields like config, db connections etc.
    }

    // --- Implementation interface ---
    func (p *MyPlugin) Initialize(ctx *proto.PluginContext) error {
        p.logger = hclog.New(&hclog.LoggerOptions{
            Name:  ctx.PluginId,
            Level: hclog.LevelFromString(ctx.LogLevel),
        })
        p.logger.Info("MyPlugin Initializing", "version", "1.0.0")
        // Parse ctx.Config, setup database connections, etc.
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
            Id: "my_plugin_id", // Should match plugin.cue
            Name: "My Awesome Plugin",
            Version: "1.0.0",
            // ... other fields
        }, nil
    }

    func (p *MyPlugin) Health() error {
        // Check dependencies, etc.
        return nil
    }

    // --- Optional: MetadataScraperService ---
    func (p *MyPlugin) MetadataScraperService() plugins.MetadataScraperService {
        // Return 'p' if MyPlugin implements MetadataScraperService, or another struct
        return p
    }

    func (p *MyPlugin) CanHandle(filePath, mimeType string) bool { return false }
    func (p *MyPlugin) ExtractMetadata(filePath string) (map[string]string, error) { return nil, nil }
    func (p *MyPlugin) GetSupportedTypes() []string { return nil }

    // ... implement other service interfaces as needed

    func main() {
        goplugin.Serve(&goplugin.ServeConfig{
            HandshakeConfig: plugins.Handshake, // Use Viewra's handshake
            Plugins: map[string]goplugin.Plugin{
                // The key here must match the one expected by Viewra's manager
                // It is usually "plugin" or the plugin's ID.
                // For Viewra, the standard key is "plugin".
                "plugin": &plugins.GRPCPlugin{Impl: &MyPlugin{}},
            },
            GRPCServer: goplugin.DefaultGRPCServer, // Use default gRPC server
        })
    }
    ```

3.  **Create `plugin.cue`:**
    Define your plugin's metadata, entry point (executable name), and any specific settings as shown in the example structure earlier.

4.  **Build the Plugin:**
    Navigate to your plugin's directory and build the executable:

    ```bash
    go build -o <entry_point_name_from_cue_file> main.go
    ```

    For example, if `entry_points: { main: "my_plugin_executable" }` in `plugin.cue`, then:

    ```bash
    go build -o my_plugin_executable main.go
    ```

5.  **Deploy the Plugin:**
    - Create a directory for your plugin within Viewra's designated plugin directory (e.g., `backend/data/plugins/my_viewra_plugin/`).
    - Place the compiled plugin executable (e.g., `my_plugin_executable`) and the `plugin.cue` file into this directory.
    - Viewra should automatically discover and load it (or it may require a restart/manual load depending on the Viewra configuration and plugin type).

### Communication with Viewra Host

- **Plugin Context**: The `Initialize` method receives a `PluginContext` which includes:
  - `PluginId`: The ID of the plugin.
  - `Config`: A `map[string]string` of the plugin-specific settings from its `plugin.cue` file.
  - `LogLevel`: Suggested log level from the host.
  - `DatabaseUrl`: Connection string for the main Viewra database (if the plugin needs it and has permissions).
  - `BasePath`: The file system path to the plugin's directory.
- **Logging**: Use the `hclog.Logger` provided or created in `Initialize` for logging. These logs are typically captured by the Viewra host.

## Plugin Management in Viewra

Viewra's plugin manager handles:

- **Discovery**: Scanning the `backend/data/plugins` directory (or configured path) for valid plugin structures (executable + `plugin.cue`).
- **Loading/Unloading**: Starting and stopping plugin processes.
- **Service Registration**: Making plugin-implemented services (like metadata scrapers) available to the core application.
- **Health Checks**: Periodically checking plugin health.
- **Hot Reload (Experimental/Partial)**: Some changes to `plugin.cue` or plugin binaries might be detected and reloaded.

## Advanced Topics

- **Plugin-Specific Database Tables**: Use the `DatabaseService` to have GORM auto-migrate your plugin's models.
- **Custom Admin UI**: Use `AdminPageService` to serve HTML/JS for a custom admin section for your plugin.
- **Security**: Be mindful of the permissions your plugin requires and what operations it performs, as it runs as a separate process but can still interact with the system based on its code.

This document provides a starting point for understanding and developing plugins for Viewra. Refer to the example plugins (like MusicBrainz Enricher) and the core plugin system code for more detailed insights.
