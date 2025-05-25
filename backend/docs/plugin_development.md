# Viewra Plugin Development Guide

This guide provides information on how to develop plugins for Viewra, a media management system.

## Plugin System Overview

Viewra's plugin system enables extending the application with custom functionality:

- **Metadata Scrapers**: Extract metadata from media files
- **Admin Pages**: Add custom admin UI pages
- **UI Components**: Add UI components to the frontend
- **Scanners**: Custom file scanning logic
- **Analyzers**: Analyze media files for insights
- **Notification Plugins**: Send notifications
- **Transcoders**: Convert media files between formats

## Plugin Structure

Each plugin is a self-contained directory with the following structure:

```
plugin_name/
├── plugin.yml          # Plugin manifest (can also be plugin.yaml)
├── main.go             # Main entry point (optional)
├── assets/             # Static assets (optional)
│   ├── css/
│   ├── js/
│   └── images/
├── templates/          # HTML templates (optional)
└── README.md           # Plugin documentation
```

## Plugin Manifest (plugin.yml)

The `plugin.yml` file defines the plugin's metadata and capabilities:

```yaml
schema_version: "1.0"
id: "unique_plugin_id"
name: "My Plugin"
version: "1.0.0"
description: "Description of the plugin"
author: "Your Name"
website: "https://example.com"
repository: "https://github.com/username/viewra-plugin"
license: "MIT"
type: "metadata_scraper"
tags:
  - "metadata"
  - "example"

capabilities:
    "metadata_extraction": true,
    "admin_pages": false,
    "ui_components": false,
    "api_endpoints": false,
    "background_tasks": false,
    "file_transcoding": false,
    "notifications": false,
    "database_access": false,
    "external_services": false
  },

  "dependencies": {
    "viewra_version": ">=0.1.0",
    "plugins": {
      "another_plugin": ">=1.0.0"
    }
  },

  "config_schema": {
    "type": "object",
    "properties": {
      "setting_1": {
        "type": "string",
        "title": "Setting 1",
        "description": "Description of Setting 1",
        "default": "default value"
      }
    }
  },

  "entry_points": {
    "main": "main.go",
    "setup": "setup.sh",
    "teardown": "teardown.sh",
    "web_server": "server.js"
  },

  "ui": {
    "admin_pages": [
      {
        "id": "my_admin_page",
        "title": "My Admin Page",
        "path": "my-page",
        "icon": "cog",
        "category": "Settings",
        "url": "/plugins/my_plugin/page.html",
        "type": "iframe"
      }
    ],
    "components": [
      {
        "id": "my_component",
        "name": "My Component",
        "type": "widget",
        "url": "/plugins/my_plugin/component.js"
      }
    ]
  },

  "permissions": ["database_read", "api_access"]
}
```

## Plugin Types

### Metadata Scraper Plugin

```go
// MetadataScraperPlugin interface for extracting metadata
type MetadataScraperPlugin interface {
	Plugin

	// Check if this plugin can handle the given file
	CanHandle(filePath string, mimeType string) bool

	// Extract metadata from a file
	ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error)

	// Return supported file types
	SupportedTypes() []string
}
```

### Admin Page Plugin

```go
// AdminPagePlugin interface for adding admin pages
type AdminPagePlugin interface {
	Plugin

	// Register admin routes
	RegisterRoutes(router *gin.RouterGroup) error

	// Get admin pages provided by this plugin
	GetAdminPages() []AdminPageConfig
}
```

### UI Component Plugin

```go
// UIComponentPlugin interface for adding UI components
type UIComponentPlugin interface {
	Plugin

	// Get UI components provided by this plugin
	GetUIComponents() []UIComponentConfig

	// Register component routes if needed
	RegisterRoutes(router *gin.RouterGroup) error
}
```

## Plugin Context

Each plugin receives a `PluginContext` with these helpers:

```go
type PluginContext struct {
	PluginID    string
	Logger      PluginLogger
	Database    Database
	Config      PluginConfig
	HTTPClient  HTTPClient
	FileSystem  FileSystemAccess
	Events      EventBus
	Hooks       HookRegistry
}
```

### Logger

```go
type PluginLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}
```

### Config

```go
type PluginConfig interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
}
```

### FileSystem Access

```go
type FileSystemAccess interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	Exists(path string) bool
	ListFiles(dir string) ([]string, error)
	CreateDir(path string) error
}
```

### Events

```go
type EventBus interface {
	Publish(event string, data interface{}) error
	Subscribe(event string, handler func(data interface{})) error
	Unsubscribe(event string, handler func(data interface{})) error
}
```

### Hooks

```go
type HookRegistry interface {
	Register(hook string, handler func(data interface{}) interface{}) error
	Execute(hook string, data interface{}) interface{}
	Remove(hook string, handler func(data interface{}) interface{}) error
}
```

## Plugin Lifecycle

1. **Discovery**: The system scans plugin directories for `plugin.yml` and `plugin.yaml` files
2. **Loading**: When enabled, the plugin is loaded and initialized
3. **Initialization**: The plugin's `Initialize(ctx)` method is called
4. **Starting**: The plugin's `Start(ctx)` method is called
5. **Running**: The plugin is active and processing requests/events
6. **Stopping**: When disabled, the plugin's `Stop(ctx)` method is called
7. **Unloading**: The plugin is unloaded from memory

## Security Considerations

- Plugins run within a restricted environment with limited access
- File system access is sandboxed to the plugin's directory
- Database access is controlled through permissions
- Plugin permissions must be explicitly granted by users

## Example Plugin

Here's a simple metadata scraper plugin:

```go
type ExampleMetadataScraperPlugin struct {
	*BasicPlugin
}

func (p *ExampleMetadataScraperPlugin) CanHandle(filePath string, mimeType string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".mp3")
}

func (p *ExampleMetadataScraperPlugin) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	metadata := map[string]interface{}{
		"extracted_by": p.Info().ID,
	}

	// ... extract actual metadata

	return metadata, nil
}

func (p *ExampleMetadataScraperPlugin) SupportedTypes() []string {
	return []string{"audio/mpeg", ".mp3"}
}
```

## Testing and Debugging Plugins

1. Place your plugin in the `data/plugins/your_plugin_name` directory
2. Enable debug logging with `PLUGIN_DEBUG=true`
3. Restart Viewra to discover the plugin
4. Check logs for plugin-related messages
5. Enable your plugin via the admin interface
