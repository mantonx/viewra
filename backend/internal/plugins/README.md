# Viewra Plugin System

This directory contains the core plugin system for Viewra. The plugin system allows for extending Viewra's functionality through plugins that can be enabled or disabled at runtime.

## Plugin Types

Viewra supports various types of plugins:

- **Metadata Scrapers**: Plugins that extract metadata from files and media.
- **Admin Pages**: Plugins that add new pages to the admin interface.
- **UI Components**: Plugins that add new UI components to the frontend.
- **Scanners**: Plugins that scan the filesystem for media files.
- **Analyzers**: Plugins that analyze media files for additional information.
- **Notification**: Plugins that send notifications to users.

## Plugin Configuration

Plugins are configured using YAML manifest files.

### Plugin Manifest Files

A plugin manifest can be defined in one of the following files:
- `plugin.yaml` - YAML format
- `plugin.yml` - YAML format (alternative extension)


```

### Example Manifest (YAML format)

```yaml
---
schema_version: "1.0"
id: "example_yaml_plugin"
name: "Example YAML Plugin"
version: "0.1.0"
description: "An example plugin with YAML configuration"
author: "Viewra Team"
website: "https://viewra.example.com"
repository: "https://github.com/example/viewra-plugin-yaml"
license: "MIT"
type: "admin_page"
tags:
  - "yaml"
  - "admin"
  - "example"

capabilities:
  admin_pages: true
  api_endpoints: true
  ui_components: false
```

## Plugin Development

To create a new plugin, follow these steps:

1. Create a new directory in the `data/plugins` directory.
2. Create a manifest file (`plugin.yaml` or `plugin.yml`) in the directory.
3. Implement your plugin code according to the plugin type.

## Plugin Loading Process

Plugins are discovered during the initialization of the plugin manager. The plugin manager will scan the plugin directory for manifest files and create plugin information objects.

By default, plugins are loaded in a disabled state. They need to be explicitly enabled through the admin interface or programmatically.

## Plugin Status

Plugins can have one of the following status values:
- `disabled`: The plugin is discovered but not loaded.
- `enabled`: The plugin is successfully loaded and running.
- `error`: The plugin failed to load or encountered an error.

## Plugin Configuration

Plugins can have their own configuration settings. These are stored in the database and can be modified through the admin interface.
