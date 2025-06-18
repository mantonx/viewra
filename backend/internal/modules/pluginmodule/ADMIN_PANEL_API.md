# Plugin API System for Admin Panel

This document describes the comprehensive plugin API system designed to support robust admin panel functionality with plugin-defined admin pages.

## Overview

The plugin API system provides a unified interface for managing plugins, their configurations, and admin panel integration. It includes:

- **Standardized API responses** with consistent error handling
- **Comprehensive plugin configuration management** with schema validation
- **Admin panel integration** supporting plugin-defined admin pages
- **Hot reload functionality** for development workflows
- **Health monitoring** and system status endpoints

## API Structure

### Main Plugin Management (`/api/v1/plugins`)

#### Core Endpoints
- `GET /` - List all plugins with filtering and pagination
- `GET /search` - Search plugins by name, description, or type
- `GET /categories` - Get available plugin categories
- `GET /capabilities` - Get system capabilities

#### Individual Plugin Management
- `GET /:id` - Get detailed plugin information
- `PUT /:id` - Update plugin metadata
- `DELETE /:id` - Uninstall plugin
- `POST /:id/enable` - Enable plugin
- `POST /:id/disable` - Disable plugin
- `POST /:id/restart` - Restart plugin
- `POST /:id/reload` - Hot reload plugin

#### Configuration Management
- `GET /:id/config` - Get plugin configuration
- `PUT /:id/config` - Update plugin configuration
- `GET /:id/config/schema` - Get configuration schema for UI generation
- `POST /:id/config/validate` - Validate configuration values
- `POST /:id/config/reset` - Reset to default configuration

#### Health & Monitoring
- `GET /:id/health` - Get plugin health status
- `GET /:id/metrics` - Get plugin performance metrics
- `GET /:id/logs` - Get plugin logs
- `POST /:id/health/reset` - Reset health status

#### Admin Panel Integration
- `GET /:id/admin-pages` - Get admin pages provided by plugin
- `GET /:id/ui-components` - Get UI components provided by plugin
- `GET /:id/assets` - Get plugin assets (CSS, JS, images)

#### Dependencies & Validation
- `GET /:id/dependencies` - Get plugin dependencies
- `GET /:id/dependents` - Get plugins that depend on this plugin
- `POST /:id/validate-dependencies` - Validate dependency tree
- `POST /:id/test` - Run plugin tests
- `POST /:id/validate` - Validate plugin installation

### Core Plugin Management (`/api/v1/plugins/core`)

- `GET /` - List core plugins
- `GET /:name` - Get core plugin details
- `POST /:name/enable` - Enable core plugin
- `POST /:name/disable` - Disable core plugin
- `GET /:name/config` - Get core plugin configuration
- `PUT /:name/config` - Update core plugin configuration

### External Plugin Management (`/api/v1/plugins/external`)

- `GET /` - List external plugins
- `POST /` - Install new plugin
- `POST /refresh` - Refresh plugin discovery
- `GET /:id` - Get external plugin details
- `POST /:id/load` - Load external plugin
- `POST /:id/unload` - Unload external plugin
- `GET /:id/manifest` - Get plugin manifest

### System Management (`/api/v1/plugins/system`)

- `GET /status` - Get overall system status
- `GET /stats` - Get system statistics
- `POST /refresh` - Refresh all plugins
- `POST /cleanup` - Clean up system resources

#### Hot Reload Management
- `GET /hot-reload` - Get hot reload status
- `POST /hot-reload/enable` - Enable hot reload
- `POST /hot-reload/disable` - Disable hot reload
- `POST /hot-reload/trigger/:id` - Trigger reload for specific plugin

#### Bulk Operations
- `POST /bulk/enable` - Enable multiple plugins
- `POST /bulk/disable` - Disable multiple plugins
- `POST /bulk/update` - Update multiple plugins

### Admin Panel Integration (`/api/v1/plugins/admin`)

- `GET /pages` - Get all plugin-provided admin pages
- `GET /navigation` - Get admin navigation structure
- `GET /permissions` - Get plugin permissions
- `PUT /permissions` - Update plugin permissions
- `GET /settings` - Get global plugin settings
- `PUT /settings` - Update global plugin settings

## Response Format

### Standard Response
```json
{
  "success": true,
  "data": { /* response data */ },
  "message": "Operation completed successfully",
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_123456"
}
```

### Error Response
```json
{
  "success": false,
  "error": "Detailed error message",
  "message": "User-friendly error description",
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_123456"
}
```

### Paginated Response
```json
{
  "success": true,
  "data": [ /* array of items */ ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "total_pages": 8,
    "has_next": true,
    "has_previous": false
  },
  "message": "Results retrieved successfully",
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_123456"
}
```

## Plugin Configuration System

### Configuration Schema
Plugins define configuration schemas that enable automatic UI generation:

```json
{
  "version": "1.0",
  "title": "Plugin Configuration",
  "description": "Configuration options for the plugin",
  "properties": {
    "api_key": {
      "type": "string",
      "title": "API Key",
      "description": "Your API key for external service",
      "sensitive": true,
      "required": true
    },
    "max_requests": {
      "type": "number",
      "title": "Max Requests",
      "description": "Maximum requests per minute",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    },
    "enabled_features": {
      "type": "array",
      "title": "Enabled Features",
      "items": {
        "type": "string",
        "enum": ["feature1", "feature2", "feature3"]
      }
    }
  },
  "categories": [
    {
      "id": "general",
      "title": "General Settings",
      "properties": ["api_key", "max_requests"]
    },
    {
      "id": "advanced",
      "title": "Advanced Options",
      "properties": ["enabled_features"],
      "collapsible": true
    }
  ]
}
```

### Admin Page Definition
Plugins can define admin pages that integrate into the main admin panel:

```json
{
  "id": "plugin-admin",
  "title": "Plugin Settings",
  "path": "/admin/plugins/my-plugin",
  "icon": "settings",
  "category": "Plugins",
  "url": "/api/plugins/my-plugin/admin",
  "type": "iframe",
  "permissions": ["plugin.configure"]
}
```

## Implementation Features

### Configuration Management
- **Schema-based validation** with type checking
- **Automatic UI generation** from configuration schemas
- **Default value handling** and override detection
- **Configuration versioning** and change tracking
- **Conditional properties** and dependencies

### Admin Panel Support
- **Plugin-defined admin pages** with iframe/component integration
- **Navigation structure** generation from plugin metadata
- **Permission-based access control** for admin features
- **Asset serving** for plugin CSS/JS/images

### Development Support
- **Hot reload functionality** for rapid development iteration
- **Comprehensive logging** and error reporting
- **Plugin health monitoring** with circuit breaker patterns
- **Dependency validation** and conflict detection

### Legacy Compatibility
The system maintains backward compatibility with existing plugin routes under `/api/plugin-manager` while providing the new comprehensive API under `/api/v1/plugins`.

## Future Enhancements

1. **Plugin Marketplace Integration** - Install plugins from external repositories
2. **Advanced Dependency Management** - Semantic versioning and conflict resolution
3. **Plugin Sandboxing** - Enhanced security isolation
4. **Performance Monitoring** - Detailed metrics and profiling
5. **A/B Testing Support** - Plugin configuration experiments
6. **Auto-update System** - Automatic plugin updates with rollback support 