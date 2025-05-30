# Events Module Architecture

## Overview

The events functionality in Viewra follows the **hybrid approach** - core infrastructure remains standalone while advanced features are modularized.

## Core Events Package (`backend/internal/events/`)

**Purpose**: Provides fundamental event bus infrastructure that the entire application needs.

**Contents**:

- `interface.go` - Event bus interfaces and contracts (314 lines)
- `types.go` - Core event types and structures (274 lines)
- `bus.go` - Main event bus implementation (547 lines)
- `global.go` - Global event bus instance access (24 lines)
- `storage.go` - Event persistence layer (426 lines)
- `metrics.go` - Event metrics collection (301 lines)

**Usage**:

- Imported by **26+ files** across the codebase
- Provides essential event publishing and subscription
- Lightweight interfaces for basic event operations

## Events Module (`backend/internal/modules/eventsmodule/`)

**Purpose**: Provides advanced event management features as part of the module system.

**Contents**:

- `module.go` - Module wrapper and lifecycle management
- Centralizes event-related API routes and handlers
- Provides advanced event administration features

**Features**:

- Event querying and filtering API
- Event management endpoints (delete, clear)
- Event health monitoring
- Future: Event streaming, advanced analytics, archival

## API Endpoints Provided by Events Module

### Event Querying

- `GET /api/v1/events/` - List events with filtering
- `GET /api/v1/events/range` - Get events by time range
- `GET /api/v1/events/types` - List available event types

### Event Management

- `POST /api/v1/events/` - Publish new event (admin)
- `DELETE /api/v1/events/:id` - Delete specific event
- `POST /api/v1/events/clear` - Clear all events

### Monitoring

- `GET /api/v1/events/health` - Event bus health check

## Why This Hybrid Approach Works

### ✅ **Core Package Benefits**

- **Lightweight**: Essential event functionality available everywhere
- **Performance**: Minimal overhead for basic event operations
- **Dependencies**: Clean dependency graph, no circular imports
- **Stability**: Core event interfaces remain stable

### ✅ **Module Benefits**

- **Centralized Management**: All event admin features in one place
- **API Organization**: Clean REST API for event operations
- **Future Extensibility**: Easy to add advanced features
- **Module Lifecycle**: Proper initialization and cleanup

### ✅ **Integration**

- **Seamless**: Module uses the core event bus via global instance
- **No Duplication**: Module enhances rather than replaces core functionality
- **Consistent**: Same event types and interfaces throughout

## Usage Examples

### Basic Event Publishing (Core Package)

```go
import "github.com/mantonx/viewra/internal/events"

// Get global event bus
eventBus := events.GetGlobalEventBus()

// Create and publish event
event := events.NewSystemEvent("scan.completed", "Scan Completed", "Media scan finished successfully")
eventBus.PublishAsync(event)
```

### Event Administration (Module API)

```bash
# Get recent events
curl "http://localhost:8080/api/v1/events/?limit=10"

# Check event bus health
curl "http://localhost:8080/api/v1/events/health"

# Clear all events (admin)
curl -X POST "http://localhost:8080/api/v1/events/clear"
```

## Comparison with Database Architecture

This follows the **same successful pattern** as the database architecture:

| Component          | Database             | Events                 |
| ------------------ | -------------------- | ---------------------- |
| **Core Package**   | `database/`          | `events/`              |
| **Module**         | `databasemodule/`    | `eventsmodule/`        |
| **Core Purpose**   | Models & connections | Event bus & interfaces |
| **Module Purpose** | Advanced DB features | Event management APIs  |
| **Imports**        | 29+ files            | 26+ files              |
| **Pattern**        | ✅ Working well      | ✅ Same approach       |

## Future Enhancements

The events module provides a foundation for adding:

1. **Real-time Event Streaming** - WebSocket subscriptions
2. **Event Analytics** - Advanced metrics and dashboards
3. **Event Archival** - Automatic cleanup and long-term storage
4. **Event Replay** - Debugging and audit capabilities
5. **Custom Event Types** - Plugin-defined event schemas
6. **Event Aggregation** - Complex event processing

## Conclusion

The events system now follows the **proven database pattern**:

- ✅ **Core infrastructure** stays lightweight and widely accessible
- ✅ **Advanced features** are properly modularized
- ✅ **Clean separation** of concerns
- ✅ **Easy to extend** with new features
- ✅ **Consistent architecture** across the application

This is **good architecture**, not tech debt!
